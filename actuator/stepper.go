package actuator

import (
	"fmt"

	"github.com/stianeikeland/go-rpio/v4"
)

const stepperPins = 4

const coilSteps = 8

var coilSeq = [8]int{
	0b0001,
	0b0011,
	0b0010,
	0b0110,
	0b0100,
	0b1100,
	0b1000,
	0b1001,
}

type Stepper struct {
	// GPIO pins stepper is connected to.
	pins [stepperPins]rpio.Pin
	// Current position (in coil sequence).
	pos int
}

func NewStepper(pinNums []int) Stepper {
	fmt.Printf("creating new stepper at pins %d\n", pinNums)
	if c := len(pinNums); c != stepperPins {
		panic(fmt.Sprintf("stepper: incorrect number of pins in definition. found %d expected %d", c, stepperPins))
	}

	var pins [stepperPins]rpio.Pin
	for i, p := range pinNums {
		pins[i] = rpio.Pin(p)
		pins[i].Output()
		pins[i].Low()
	}

	return Stepper{pins: pins}
}

// delta should be +1/-1 only, otherwise it will just skip.
// +1 - clockwise if looking from sensor side
func (s *Stepper) Step(delta int) {
	s.pos = (s.pos + delta) % coilSteps
	s.setPins()
}

func (s *Stepper) setPins() {
	v := coilSeq[s.pos]
	for i := 0; i < stepperPins; i++ {
		if v&1 == 0 {
			s.pins[i].Low()
		} else {
			s.pins[i].High()
		}
		v = v >> 1
	}
}

func (s *Stepper) PowerOn() {
	s.setPins()
}

func (s *Stepper) PowerOff() {
	for i := 0; i < stepperPins; i++ {
		s.pins[i].Low()
	}
}
