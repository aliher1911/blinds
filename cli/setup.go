package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aliher1911/blinds/actuator"
	"github.com/aliher1911/blinds/controller"
	i2cdev "github.com/aliher1911/blinds/i2c"
	"github.com/aliher1911/blinds/input"
	"github.com/aliher1911/blinds/sensor"

	"github.com/stianeikeland/go-rpio/v4"
)

const int_pin = 4

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
	if angle > 170 || angle < -170 {
		fmt.Printf("")
	}

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
	ctrl.Start()
	ctrl.SetTarget(0)

	for {
		if ctrl.AtTarget() {
			fmt.Printf("reached target %d\n", angle)
			break
		}
		<-time.After(time.Second)
	}
}

func logInterrupts(ctx context.Context, intr <-chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-intr:
			fmt.Println("interrupt fired")
		}
	}
}

func intDetector(ctx context.Context, c chan<- interface{}, pin i2cdev.IntPin) {
	for {
		if pin.EdgeDetected() {
			select {
			case c <- struct{}{}:
				fmt.Println("notification fired")
			default:
				fmt.Println("already pending")
			}
		}
		select {
		case <-time.After(time.Millisecond * 20):
		case <-ctx.Done():
			fmt.Println("context cancelled")
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
	intC := make(chan interface{})
	go intDetector(ctx, intC, intPin)
	go logInterrupts(ctx, intC)

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
