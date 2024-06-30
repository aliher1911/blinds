package cli

import (
	"context"
	"fmt"
	"math"
	"os"
	"sync"

	"github.com/aliher1911/blinds/actuator"
	"github.com/aliher1911/blinds/controller"
	"github.com/aliher1911/blinds/input"
	"github.com/aliher1911/blinds/sensor"
	"github.com/aliher1911/blinds/ui"
)

func Service(bus uint, baseAngle int32, sigs <-chan os.Signal) {
	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctrl.Run(ctx)
	}()

	l, lC := input.NewLED(r)
	wg.Add(1)
	go func() {
		defer wg.Done()
		l.Run(ctx)
	}()

	a := &DocAdapter{ctrl: ctrl}
	ui := ui.New(r, ctrl.InterruptC(), lC, a)
	wg.Add(1)
	go func() {
		defer wg.Done()
		ui.Run(ctx)
	}()

	<-sigs
	fmt.Println("service: Received interrupt signal. Aborting.")

	r.LED(input.Off)
}

type DocAdapter struct {
	ctrl        *controller.Controller
	initialized bool
}

func (a *DocAdapter) SetAngle(angle int32) {
	if a.initialized {
		a.ctrl.SetTarget(angle)
	}
}

func (a *DocAdapter) SetAuto(auto bool) {
}

func (a *DocAdapter) GetState() ui.State {
	ct := a.ctrl.Target()
	pos := a.ctrl.Pos()
	if !a.initialized {
		if pos == controller.NoAngle {
			ct = 0
			pos = 0
		} else {
			a.initialized = true
			// Round current angle to closest 10 degree step.
			ct = int32(math.Round(float64(pos)/10)) * 10
		}
	}
	return ui.State{
		SetAngle:     ct,
		CurrentAngle: pos,
		Auto:         false,
	}
}
