package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/aliher1911/blinds/input"
)

type State struct {
	// Set angle
	SetAngle int32
	// Current blinds angle
	CurrentAngle int32
	// Control mode (if external system should adaptively control blinds)
	Auto bool
}

// Update
type Update interface {
	SetAngle(angle int32)
	SetAuto(auto bool)
	GetState() State
}

// UI performs user interaction.
type UI struct {
	// UI interrupt
	intC <-chan time.Time
	rot  *input.Rotary
	ledC chan *input.LedOp

	// Document
	doc Update
}

func New(rotary *input.Rotary, intC <-chan time.Time, led chan *input.LedOp, doc Update) *UI {
	return &UI{
		intC: intC,
		rot:  rotary,
		ledC: led,
		doc:  doc,
	}
}

type state int

const (
	// waiting for user input
	idle state = iota
	// after receiving interrupt, wait for a while to mask spurious changes
	debounce
	// wait for more input before applying state to underlying document
	edit
)

const debounceT = 100 * time.Millisecond
const uiApplyT = 3 * time.Second

// Note we can use negative step to invert cw/ccw rotation.
// If we do, we also need to adjust LED color formula and swap color rates.
const clickAngle = int32(-10)

const maxAngle = int32(140)
const minAngle = int32(-140)

// UI State machine loop.
func (u *UI) Run(ctx context.Context) error {
	never := time.Duration(1<<63 - 1)
	s := idle
	btn := false
	base := int32(0)
	t := time.NewTimer(never)
	resetT := func(d time.Duration) {
		if !t.Stop() {
			select {
			case <-t.C:
			default:
			}
		}
		t.Reset(d)
	}

	for {
		fmt.Printf("ui: State is %d\n", s)
		switch s {
		case idle:
			// wait for interrupt and cancel
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-u.intC:
				s = debounce
				resetT(debounceT)
				base = u.doc.GetState().SetAngle
				fmt.Printf("ui: Starting edit with base angle %d\n", base)
			}
		case debounce:
			// wait for timer and cancel
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-t.C:
				// Handle input event
				s = edit
				if b, _, err := u.rot.Button(); err == nil {
					if b != btn && b {
						// Process button press here
						fmt.Printf("Button is pressed\n")
					}
					btn = b
				} else {
					fmt.Printf("ui: Err reading button state: %s\n", err)
				}
				if d, err := u.rot.Delta(); err == nil {
					base += int32(d) * clickAngle
					switch {
					case base > maxAngle:
						base = maxAngle
					case base < minAngle:
						base = minAngle
					}
					// change color
					u.ledC <- input.NewLedOp(angleColor(base), uiApplyT)
				} else {
					fmt.Printf("ui: Err reading button state: %s\n", err)
				}
				resetT(uiApplyT)
			case <-u.intC:
				fmt.Printf("ui: Ignoring handling interrupts while debouncing\n")
			}
		case edit:
			// wait for interrupt, cancel or timeout
			select {
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			case <-u.intC:
				s = debounce
				resetT(debounceT)
			case <-t.C:
				s = idle
				// no activity, apply change to angle
				u.doc.SetAngle(base)
				fmt.Printf("ui: Finish edit with new angle %d\n", base)
			}
		}
	}
}

func angleColor(angle int32) input.Color {
	const fullRange = maxAngle - minAngle
	zeroBased := (angle - minAngle)
	ratio := float32(zeroBased) / float32(fullRange)
	b := input.Blue.Scale(1 - ratio)
	g := input.Green.Scale(ratio)
	c := b.Add(g)
	return c
}

type LoggerUpdate struct {
	d int32
	a bool
}

func (l *LoggerUpdate) SetAngle(angle int32) {
	fmt.Printf("Setting angle to %d\n", angle)
	l.d = angle
}

func (l *LoggerUpdate) SetAuto(auto bool) {
	fmt.Printf("Setting auto to %t\n", auto)
	l.a = auto
}

func (l *LoggerUpdate) GetState() State {
	return State{
		SetAngle: l.d,
		Auto:     l.a,
	}
}
