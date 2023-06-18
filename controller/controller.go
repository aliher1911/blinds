package controller

import (
	"fmt"
	"math"
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
	Delay             time.Duration
	IdleDelay         time.Duration
	PosUpdateInterval time.Duration
	StopMotionAfter   time.Duration
	StepsPerDegree    int64
	PositionAccuracy  int32

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
		MinAngle:          -75,
		MaxAngle:          75,
		MinRate:           0.0001,
		MaxRate:           0.01,
		IntPin:            -1,
	}
}

type Controller struct {
	Config
	s *actuator.Stepper
	p *sensor.Position

	intPin i2cdev.IntPin
	intC   chan interface{}

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
		startedC:    make(chan interface{}),
		stoppedC:    make(chan interface{}),
	}
	if cfg.IntPin >= 0 {
		// Maybe make this external?
		c.intC = make(chan interface{})
		c.intPin = i2cdev.NewIntPin(cfg.IntPin, rpio.FallEdge)
	}
	return c
}

func (c *Controller) Start() {
	select {
	case <-c.startedC:
	default:
		go c.run()
		close(c.startedC)
	}
}

func (c *Controller) Stop() {
	select {
	case <-c.stoppedC:
	default:
		close(c.stoppedC)
	}
}

func (c *Controller) Pos() int32 {
	return atomic.LoadInt32(&c.lastAngle)
}

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

func (c *Controller) AtTarget() bool {
	return abs(c.targetAngle-c.Pos())*2 < c.PositionAccuracy
}

func (c *Controller) Target() int32 {
	return c.targetAngle
}

func (c *Controller) InterruptC() <-chan interface{} {
	return c.intC
}

type posUpdate struct {
	timeStamp time.Time
	pos       int64
	angle     int32 // TODO: maybe change to float
}

func (c *Controller) run() {
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
	var updatePending = false
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
	}

	var reachedTarget bool
	var safetyStop bool

	for {
		select {
		case <-c.stoppedC:
			// Terminate as needed.
			if updatePending {
				// Wait for pos reader to terminate.
				<-readPosC
			}
			c.s.PowerOff()
			return
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
				fmt.Printf("update: pos=%d, targetPos=%d, readPos=%d(dp=%d), "+
					"angle=%d, targetAngle=%d\n",
					pos, targetPos, newShaftPos.pos, pos-newShaftPos.pos, newShaftPos.angle,
					targetAngle)
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
				go updateFn()
			}
			if sinceUpdate > c.StopMotionAfter {
				// Prevent any movements to avoid crashing into stops.
				fmt.Printf("No updates for %s, stopping motion\n", sinceUpdate)
				safetyStop = true
			}
		}

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
					go updateFn()
				}
				next = time.After(c.Delay)
			} else {
				next = time.After(c.IdleDelay)
			}
			reachedTarget = true
		}

		// Handle interrupt polling.
		if c.intPin.EdgeDetected() {
			select {
			case c.intC <- struct{}{}:
			default:
			}
		}

		// Wait for next loop time.
		<-next
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
