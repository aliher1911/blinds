package i2cdev

import "github.com/stianeikeland/go-rpio/v4"

type IntPin struct {
	set bool
	pin rpio.Pin
}

func NewIntPin(pin_num int, edge rpio.Edge) IntPin {
	pin := rpio.Pin(pin_num)
	pin.Mode(rpio.Input)
	pin.Pull(rpio.PullUp)
	pin.Detect(edge)
	return IntPin{
		set: true,
		pin: pin,
	}
}

func (p IntPin) Read() bool {
	return p.set && p.pin.Read() == rpio.Low
}

func (p IntPin) EdgeDetected() bool {
	return p.set && p.pin.EdgeDetected()
}
