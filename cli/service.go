package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aliher1911/blinds/actuator"
	"github.com/aliher1911/blinds/controller"
	"github.com/aliher1911/blinds/input"
	"github.com/aliher1911/blinds/sensor"
)

func Service(bus uint, baseAngle int32, sigs <-chan os.Signal) {
	s := actuator.NewStepper(actuator.DefaultPins)
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

	ccfg := controller.Defaults()
	ccfg.IntPin = int_pin
	p := sensor.NewPositionSensor(m, float32(baseAngle))
	ctrl := controller.NewController(&s, &p, ccfg)
	ctrl.Start()
	ctrl.SetTarget(0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go logInterrupts(ctx, ctrl.InterruptC())

	func() {
		steps := []int32{0, 30, -30, 10, -50}
		i := 0
		delay := time.Second
		for {
			select {
			case <-sigs:
				fmt.Println("received interrupt, winding down")
				return
			case <-time.After(delay):
				p := ctrl.Pos()
				fmt.Printf("position: %d\n", p)
				if ctrl.AtTarget() {
					i++
					if i == len(steps) {
						return
					}
					fmt.Printf("setting new target: %d\n", steps[i])
					ctrl.SetTarget(steps[i])
				}
				delay = 10 * time.Second
			}
		}
	}()

	ctrl.Stop()
	r.LED(input.Color(0))
}
