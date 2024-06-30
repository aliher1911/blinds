package controller

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aliher1911/blinds/actuator"
	i2cdev "github.com/aliher1911/blinds/i2c"
	"github.com/aliher1911/blinds/sensor"

	"github.com/stianeikeland/go-rpio/v4"
	"golang.org/x/exp/constraints"
)

const NoAngle = -999999

type Config struct {
	// Delay between advances to current goal.
	Delay time.Duration
	// Delay between advances to current goal if at target or detected a stall.
	IdleDelay time.Duration
	// How frequently we read position sensor.
	PosUpdateInterval time.Duration
	// Stop movement if we can't get a valid reading for the period.
	StopMotionAfter time.Duration
	// Stepper + gearbox reduction param.
	StepsPerDegree int64
	// Threshold within target that we consider to be spot on.
	PositionAccuracy int32

	MinAngle int32
	MaxAngle int32

	MinRate float32
	MaxRate float32

	IntPin int
}

func Defaults() Config {
	return Config{
		Delay:             75 * time.Microsecond,
		IdleDelay:         time.Second,
		PosUpdateInterval: time.Second,
		StopMotionAfter:   5 * time.Second,
		StepsPerDegree:    180,
		PositionAccuracy:  5,
		MinAngle:          -140,
		MaxAngle:          140,
		MinRate:           0.0001,
		MaxRate:           0.01,
		IntPin:            -1,
	}
}

// Controller is responsible for rotating shaft and setting it to the requested angle.
// It runs tight loop so it is also responsible for checking external edge detector
// to handle UI interrupts.
type Controller struct {
	Config
	s *actuator.Stepper
	p *sensor.Position

	intPin i2cdev.IntPin
	intC   chan time.Time

	lastAngle   int32
	targetAngle int32
	targetC     chan int32

	startedC chan interface{}
	stoppedC chan interface{}
}

func NewController(s *actuator.Stepper, p *sensor.Position, cfg Config) *Controller {
	c := &Controller{
		Config:      cfg,
		s:           s,
		p:           p,
		lastAngle:   NoAngle,
		targetAngle: NoAngle,
		targetC:     make(chan int32, 1),
	}
	if cfg.IntPin >= 0 {
		// We do this because this is the only tight loop in the app.
		c.intC = make(chan time.Time)
		c.intPin = i2cdev.NewIntPin(cfg.IntPin, rpio.FallEdge)
	}
	return c
}

// Pos reads current position
func (c *Controller) Pos() int32 {
	return atomic.LoadInt32(&c.lastAngle)
}

// SetTarget sets desired shaft angle
func (c *Controller) SetTarget(angle int32) {
	if angle < c.MinAngle {
		angle = c.MinAngle
	}
	if angle > c.MaxAngle {
		angle = c.MaxAngle
	}
	if angle == c.targetAngle {
		return
	}
	c.targetAngle = angle
	c.targetC <- angle
}

// AtTarget is true if shaft is positioned at target
func (c *Controller) AtTarget() bool {
	return abs(c.targetAngle-c.Pos())*2 < c.PositionAccuracy
}

// Target restuns set shaft angle.
func (c *Controller) Target() int32 {
	return c.targetAngle
}

func (c *Controller) InterruptC() <-chan time.Time {
	return c.intC
}

type posUpdate struct {
	timeStamp time.Time
	pos       int64
	angle     int32 // TODO: maybe change to float
}

func (c *Controller) Run(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	// Position in the middle to avoid all the maths nesessary.
	// We would never run out of steps :-)
	// Note: This is atomic to synchronize with magnetometer poll.
	var pos int64 = math.MaxInt32

	// Computed position updated when incoming setter changes or
	// we receive shaft position update.
	var targetPos = pos

	// Target angle is saved position as requested from UI.
	var targetAngle int32 = NoAngle

	// Interaction with position poller.
	readPosC := make(chan posUpdate)
	shaftPos := posUpdate{
		pos:   0,
		angle: NoAngle,
	}
	// Must always be run inside wait group.
	var updatePending = false
	defer func() {
		if updatePending {
			// Wait for pos reader to terminate so that its chan is unblocked.
			<-readPosC
		}
		c.s.PowerOff()
	}()

	updateFn := func() {
		now := time.Now()
		savedPos := atomic.LoadInt64(&pos)
		a, err := c.p.Read()
		if err != nil {
			readPosC <- posUpdate{
				angle: NoAngle,
			}
		} else {
			readPosC <- posUpdate{
				timeStamp: now,
				pos:       savedPos,
				angle:     int32(a),
			}
		}
		wg.Done()
	}

	var reachedTarget bool
	var safetyStop bool

	for {
		// Check if we received any commands/updates or temination request.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case targetAngle = <-c.targetC:
			// Handle target update.
			reachedTarget = false
			targetPos = c.targetSteps(shaftPos, targetAngle, c.StepsPerDegree)
		case newShaftPos := <-readPosC:
			// Handle shaft angle update.
			updatePending = false
			if newShaftPos.angle != NoAngle {
				// No error reading shaft.
				da := abs(newShaftPos.angle - targetAngle)
				// Adjust if we didn't zero on target, then only if we are too far away.
				if targetAngle != NoAngle && (!reachedTarget && da > 0 || reachedTarget && da*2 > c.PositionAccuracy) {
					targetPos = c.targetSteps(newShaftPos, targetAngle, c.StepsPerDegree)
					reachedTarget = false
				}
				//fmt.Printf("ctrl: update: pos=%d, targetPos=%d, readPos=%d(dp=%d), "+
				//	"angle=%d, targetAngle=%d\n",
				//	pos, targetPos, newShaftPos.pos, pos-newShaftPos.pos, newShaftPos.angle,
				//	targetAngle)
				// Save current values.
				shaftPos = newShaftPos
				safetyStop = false
				atomic.StoreInt32(&c.lastAngle, shaftPos.angle)
			}
		default:
		}

		// Request refresh position as needed.
		{
			now := time.Now()
			sinceUpdate := now.Sub(shaftPos.timeStamp)
			if !updatePending && sinceUpdate > c.PosUpdateInterval {
				updatePending = true
				wg.Add(1)
				go updateFn()
			}
			if sinceUpdate > c.StopMotionAfter {
				// Prevent any movements to avoid crashing into stops.
				fmt.Printf("ctrl: No updates for %s, stopping motion\n", sinceUpdate)
				safetyStop = true
			}
		}

		// Update stepper and calculate next step delay.
		var next <-chan time.Time
		pd := targetPos - pos
		switch {
		case safetyStop:
			next = time.After(c.IdleDelay)
		case pd > 0:
			c.s.Step(1)
			atomic.AddInt64(&pos, 1)
			next = time.After(c.Delay)
		case pd < 0:
			c.s.Step(-1)
			atomic.AddInt64(&pos, -1)
			next = time.After(c.Delay)
		default:
			// When steps are reached trigger immediate update and ignore extra delay if first
			// attempt.
			if !reachedTarget {
				if !updatePending {
					updatePending = true
					wg.Add(1)
					go updateFn()
				}
				next = time.After(c.Delay)
			} else {
				next = time.After(c.IdleDelay)
			}
			reachedTarget = true
			c.s.PowerOff()
		}

		// Wait for next loop time.
	stepper_delay:
		for {
			// Handle interrupt polling as it is the only busy loop in app.
			// Maybe we should make it a callback to decouple?
			if c.intPin.EdgeDetected() {
				select {
				case c.intC <- time.Now():
				default:
				}
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(20 * time.Millisecond):
			case <-next:
				break stepper_delay
			}
		}
	}
}

// Compute new target step counter.
func (cfg *Config) targetSteps(p posUpdate, targetAngle int32, speed int64) int64 {
	da := -int64(targetAngle - p.angle)
	if da < 10 {
		// Reduce speed if reaching destination.
		speed = speed / 2
	}
	pSteps := p.pos + da*speed
	return pSteps
}

func abs[T constraints.Integer](val T) T {
	if val < 0 {
		return -val
	}
	return val
}
