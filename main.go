package main

import (
	"fmt"
	"time"

	"github.com/aliher1911/blinds/actuator"
	"github.com/aliher1911/blinds/input"
	"github.com/aliher1911/blinds/sensor"

	logger "github.com/d2r2/go-logger"
	rpio "github.com/stianeikeland/go-rpio/v4"
)

var colorSeq = []input.Color{
	input.Red, input.Green, input.Blue, input.White,
}

func main() {
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	const bus = 1

	fmt.Println("blinds control")
	err := rpio.Open()
	if err != nil {
		fmt.Printf("failed to open GPIO: %s\n", err)
		return
	}
	defer rpio.Close()

	s := actuator.NewStepper([]int{26, 13, 6, 5})
	defer s.PowerOff()

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

	for i := 0; i < 100000; i++ {
		if i%100 == 0 {
			if x, y, z, err := m.Read(); err != nil {
				fmt.Printf("failed to read magnetometer: %s\n", err)
			} else {
				fmt.Printf("step=%7d, x=%6.0f, y=%6.0f, z=%6.0f\n", i, x, y, z)
			}
			p, e1 := r.Position()
			b, e2 := r.Button()

			if e1 != nil || e2 != nil {
				fmt.Printf("pos=%3d, butt=%t, err1=%s, err2=%s\n", p, b, e1, e2)
			} else {
				fmt.Printf("pos=%3d, butt=%t\n", p, b)
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
		s.Step(1)
		<-time.After(75 * time.Microsecond)
	}
}
