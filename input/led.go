package input

import (
	"context"
	"time"
)

type LedOp struct {
	c Color
	t time.Duration
	n *LedOp
}

func NewLedOp(c Color, t time.Duration, next ...*LedOp) *LedOp {
	root := &LedOp{
		c: c,
		t: t,
	}
	last := root
	for _, n := range next {
		last.n = n
		for nn := n.n; nn != nil; nn = nn.n {
			last = nn
		}
	}
	return root
}

type LED struct {
	r    *Rotary
	outC chan *LedOp
}

func NewLED(r *Rotary) (*LED, chan *LedOp) {
	c := make(chan *LedOp, 10)
	return &LED{
		r:    r,
		outC: c,
	}, c
}

// Run is a LED work loop and should be started in a separate
// goroutine.
func (l *LED) Run(ctx context.Context) error {
	never := time.Duration(1<<63 - 1)
	t := time.NewTimer(never)
	next := &LedOp{
		c: Off,
		t: never,
	}
	for {
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case next = <-l.outC:
			if !t.Stop() {
				<-t.C
			}
		case <-t.C:
		}
		l.r.LED(next.c)
		t.Reset(next.t)
		if next.n != nil {
			next = next.n
		} else {
			// Turn off at the end of sequence.
			next = &LedOp{
				c: Off,
				t: never,
			}
		}
	}
}
