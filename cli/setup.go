package cli

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/aliher1911/blinds/actuator"
	"github.com/aliher1911/blinds/controller"
	i2cdev "github.com/aliher1911/blinds/i2c"
	"github.com/aliher1911/blinds/input"
	"github.com/aliher1911/blinds/sensor"
	"github.com/aliher1911/blinds/ui"

	"github.com/stianeikeland/go-rpio/v4"
)

const int_pin = 4

const logTimeFmt = "15:04:05.999999999"

func GetAngle(bus uint) {
	m, err := sensor.NewMagnetometer(sensor.Default(bus))
	if err != nil {
		fmt.Printf("failed to init magnetometer: %s\n", err)
		return
	}
	defer m.Close()
	p := sensor.NewPositionSensor(m, 0)
	for i := 0; i < 5; i++ {
		a, err := p.Read()
		if err != nil {
			fmt.Printf("failed to read position value\n")
		} else {
			fmt.Printf("current angle is %f\n", a)
		}
		<-time.After(time.Second)
	}
}

func SetAngle(bus uint, base, angle int32) {
	fmt.Printf("Set angle to %d with base %d\n", angle, base)

	m, err := sensor.NewMagnetometer(sensor.Default(bus))
	if err != nil {
		fmt.Printf("failed to init magnetometer: %s\n", err)
		return
	}
	defer m.Close()
	s := actuator.NewStepper(actuator.DefaultPins)
	defer s.PowerOff()

	ccfg := controller.Defaults()
	p := sensor.NewPositionSensor(m, float32(base))
	ctrl := controller.NewController(&s, &p, ccfg)
	go ctrl.Run(context.Background())
	ctrl.SetTarget(angle)

	for {
		if ctrl.AtTarget() {
			fmt.Printf("reached target %d\n", angle)
			break
		}
		<-time.After(time.Second)
	}
}

func handleInterrupts(ctx context.Context, intr <-chan time.Time, fb func(t time.Time, count int) bool) {
	var count int
	defer func() {
		fmt.Printf("intr: finished after %d\n", count)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case t := <-intr:
			if !fb(t, count) {
				return
			}
		}
		count++
	}
}

func logInterrupts(t time.Time, count int) bool {
	fmt.Printf("intr: interrupt %d at %s handled at %s\n", count, t.Format(logTimeFmt), time.Now().Format(logTimeFmt))
	return true
}

func intDetector(ctx context.Context, c chan<- time.Time, pin i2cdev.IntPin) {
	var count, tick int
	t := time.NewTicker(20 * time.Millisecond)
	defer t.Stop()

	for {
		if pin.EdgeDetected() {
			now := time.Now()
			select {
			case c <- now:
				fmt.Printf("int_det: notification fired: %d/%d at %s\n", count, tick, now.Format(logTimeFmt))
				count++
			default:
				fmt.Printf("int_det: notification already pending: %d/%d\n", count, tick)
			}
		}
		select {
		case <-t.C:
		case <-ctx.Done():
			fmt.Println("int_det: context cancelled, terminating")
			return
		}
	}
}

var colorSeq = []input.Color{
	input.Red, input.Green, input.Blue, input.White,
}

func CliTest(bus uint, sigs <-chan os.Signal) {
	intPin := i2cdev.NewIntPin(int_pin, rpio.FallEdge)

	// Close int routines before stopping gpio.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	intC := make(chan time.Time)
	go intDetector(ctx, intC, intPin)
	go handleInterrupts(ctx, intC, logInterrupts)

	m, err := sensor.NewMagnetometer(sensor.Default(bus))
	if err != nil {
		fmt.Printf("failed to init magnetometer: %s\n", err)
		return
	}
	defer m.Close()

	r, err := input.NewRotary(input.Default(bus))
	if err != nil {
		fmt.Printf("failed to init rotatore: %s\n", err)
		return
	}
	defer r.Close()
	defer r.LED(input.Color(0))

	for i := 0; i < 50000; i++ {
		if i%100 == 0 {
			if x, y, z, err := m.Read(); err != nil {
				fmt.Printf("failed to read magnetometer: %s\n", err)
			} else {
				fmt.Printf("step=%7d, x=%6.0f, y=%6.0f, z=%6.0f\n", i, x, y, z)
			}

			pi := intPin.Read()
			b, bi, e2 := r.Button()
			p, e1 := r.Position()

			if e1 != nil || e2 != nil {
				fmt.Printf("pos=%3d, butt=%t, err1=%s, err2=%s\n", p, b, e1, e2)
			} else {
				fmt.Printf("pos=%3d, butt=%t, piInt=%t, int=%t\n", p, b, pi, bi)
			}

			c := colorSeq[(i/1000)%len(colorSeq)]
			switch {
			case p < 0:
				c = c.Scale(0.5)
			case p == 0:
				c = 0
			}
			r.LED(c)
		}
		select {
		case <-time.After(10 * time.Millisecond):
		case s := <-sigs:
			fmt.Printf("stopping on signal %s\n", s)
			return
		}
	}
}

func IntDebug(bus uint, sigs <-chan os.Signal) {
	intPin := i2cdev.NewIntPin(int_pin, rpio.FallEdge)

	r, err := input.NewRotary(input.Default(bus))
	if err != nil {
		fmt.Printf("failed to init rotatore: %s\n", err)
		return
	}
	defer r.Close()

	reset := func() {
		r.Button()
		r.Position()
	}
	defer reset()

	// Close int routines before stopping gpio.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	intC := make(chan time.Time)
	go intDetector(ctx, intC, intPin)
	go handleInterrupts(ctx, intC, func(t time.Time, count int) bool {
		logInterrupts(t, count)
		reset()
		return true
	})

	var count int
	for {
		select {
		case <-sigs:
			return
		case <-time.After(10 * time.Second):
		}
		fmt.Printf("main: periodic reset interrupt %d\n", count)
		count++
		reset()
	}
}

func randChoice[T any](choice []T) T {
	return choice[rand.Intn(len(choice))]
}

func LEDDemo(bus uint, sigs <-chan os.Signal) {
	r, err := input.NewRotary(input.Default(bus))
	if err != nil {
		fmt.Printf("failed to init rotatore: %s\n", err)
		return
	}
	defer r.Close()

	l, lC := input.NewLED(r)
	ctx, cancel := context.WithCancel(context.Background())
	go l.Run(ctx)

	cChoice := []input.Color{
		input.Red, input.Green, input.Blue, input.Cyan, input.Yellow, input.Off, input.Magenta, input.White,
	}
	tChoice := []time.Duration{
		time.Second, time.Millisecond * 500, time.Second * 5,
	}

	fmt.Println("Starting generation loop")
	t := time.NewTicker(time.Second * 10)
	for {
		select {
		case <-sigs:
			cancel()
			return
		case <-t.C:
			d := randChoice(tChoice)
			fmt.Printf("Pushing seq with delay %s\n", d)
			lC <- input.NewLedOp(randChoice(cChoice), d,
				input.NewLedOp(randChoice(cChoice), d),
				input.NewLedOp(randChoice(cChoice), d))
		}
	}
}

func UIDemo(bus uint, sigs <-chan os.Signal) {
	r, err := input.NewRotary(input.Default(bus))
	if err != nil {
		fmt.Printf("failed to init rotatore: %s\n", err)
		return
	}
	defer r.Close()

	ctx, cancel := context.WithCancel(context.Background())

	intPin := i2cdev.NewIntPin(int_pin, rpio.FallEdge)
	intC := make(chan time.Time)
	go intDetector(ctx, intC, intPin)

	l, lC := input.NewLED(r)
	go l.Run(ctx)

	u := ui.New(r, intC, lC, &ui.LoggerUpdate{})
	go u.Run(ctx)

	for {
		select {
		case <-sigs:
			cancel()
			return
		}
	}
}
