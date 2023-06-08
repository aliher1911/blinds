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

func main() {
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)

	fmt.Println("blinds control")
	err := rpio.Open()
	if err != nil {
		fmt.Printf("failed to open GPIO: %s\n", err)
		return
	}
	defer rpio.Close()

	s := actuator.NewStepper([]int{26, 13, 6, 5})
	defer s.PowerOff()

	m, err := sensor.NewMagnetometer(sensor.Default())
	if err != nil {
		fmt.Printf("failed to init magnetometer: %s\n", err)
		return
	}
	defer m.Close()

	r, err := input.NewRotary(input.Default())
	if err != nil {
		fmt.Printf("failed to init rotatore: %s\n", err)
		return
	}
	defer r.Close()

	for i := 0; i < 10000; i++ {
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
		}
		s.Step(1)
		<-time.After(75 * time.Microsecond)
	}
}
