package input

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aliher1911/blinds/i2c"

	"github.com/aliher1911/go-i2c"
)

type Rotary struct {
	bus *i2c.I2C
	c   Conf
}

type Color uint32

func RGB(r, g, b uint8) Color {
	return Color(g)<<16 | Color(r)<<8 | Color(b)
}

func RGBB(r, g, b uint8, brightness float32) Color {
	return Color(float32(g)*brightness)<<16 | Color(float32(r)*brightness)<<8 | Color(float32(b)*brightness)
}

// Adjust brightness. Hue could drift after multiple operations.
func (c Color) Scale(coef float32) Color {
	scale := func(val Color) Color {
		r := Color(float32(val&0xff) * coef)
		if r > 255 {
			r = 255
		}
		return r
	}
	return scale(c>>16)<<16 | scale(c>>8)<<8 | scale(c)
}

const (
	Green Color = 0xff0000
	Red   Color = 0x00ff00
	Blue  Color = 0x0000ff
	White Color = 0xffffff
)

const (
	GPIO_BASE = 0x01

	GPIO_DIRSET_BULK = 0x02
	GPIO_DIRCLR_BULK = 0x03
	GPIO_BULK        = 0x04
	GPIO_BULK_SET    = 0x05
	GPIO_BULK_CLR    = 0x06
	GPIO_BULK_TOGGLE = 0x07
	GPIO_INTENSET    = 0x08
	GPIO_INTENCLR    = 0x09
	// Read to reset the all gpio int flags.
	GPIO_INTFLAG   = 0x0A
	GPIO_PULLENSET = 0x0B
	GPIO_PULLENCLR = 0x0C

	ENCODER_BASE = 0x11

	ENCODER_STATUS   = 0x00
	ENCODER_INTENSET = 0x10
	ENCODER_INTENCLR = 0x20
	// Read any to reset rotary int flag.
	ENCODER_POSITION = 0x30
	ENCODER_DELTA    = 0x40

	NEOPIXEL_BASE = 0x0E

	NEOPIXEL_PIN        = 0x01
	NEOPIXEL_BUF_LENGTH = 0x03
	NEOPIXEL_BUF        = 0x04
	NEOPIXEL_SHOW       = 0x05
)

const delay = 8 * time.Millisecond
const neopixelPin = 6
const buttonPin = 24
const defaultAddr = 0x36

type Conf struct {
	i2cdev.Conf
	NeopixelPin int
	ButtonPin   int
}

func Default(bus uint) Conf {
	return Conf{
		Conf: i2cdev.Conf{
			Addr: defaultAddr,
			Bus:  int(bus),
		},
		NeopixelPin: neopixelPin,
		ButtonPin:   buttonPin,
	}
}

func NewRotary(c Conf) (*Rotary, error) {
	bus, err := i2c.NewI2C(c.Addr, 1)
	if err != nil {
		return nil, err
	}

	s := &Rotary{
		bus: bus,
		c:   c,
	}

	// Setup rotary interrupt
	if err := s.write(ENCODER_BASE, ENCODER_INTENSET, []byte{0x01}); err != nil {
		return nil, err
	}

	// Setup button pin to INPUT_PULLUP
	mask := uint32(1) << uint32(c.ButtonPin)
	cmd := make([]byte, 4)
	binary.BigEndian.PutUint32(cmd, mask)
	if err := s.write(GPIO_BASE, GPIO_DIRCLR_BULK, cmd); err != nil {
		return nil, err
	}
	if err := s.write(GPIO_BASE, GPIO_PULLENSET, cmd); err != nil {
		return nil, err
	}
	if err := s.write(GPIO_BASE, GPIO_BULK_SET, cmd); err != nil {
		return nil, err
	}
	// Enable button interrupt.
	if err := s.write(GPIO_BASE, GPIO_INTENSET, cmd); err != nil {
		return nil, err
	}

	// Setup neopixel
	fmt.Printf("setting neopixel pin to %d\n", c.NeopixelPin)
	if err := s.write(NEOPIXEL_BASE, NEOPIXEL_PIN, []byte{byte(c.NeopixelPin)}); err != nil {
		return nil, err
	}
	// buf length is 3 = 1 LED with 3 bpp, encoded as short big endian
	fmt.Printf("setting neopixel buffer size to %d\n", 3)
	if err := s.write(NEOPIXEL_BASE, NEOPIXEL_BUF_LENGTH, []byte{0, 3}); err != nil {
		return nil, err
	}

	return s, nil
}

// Use delta instead.
func (r *Rotary) Position() (int, error) {
	buf := make([]byte, 4)
	if err := r.read(ENCODER_BASE, ENCODER_POSITION, buf, delay); err != nil {
		return 0, err
	}
	return int(binary.BigEndian.Uint32(buf)), nil
}

func (r *Rotary) SetPosition(newPos int) error {
	return nil
}

func (r *Rotary) Button() (bool, bool, error) {
	flags := make([]byte, 4)
	if err := r.read(GPIO_BASE, GPIO_INTFLAG, flags, delay); err != nil {
		return false, false, err
	}
	buf := make([]byte, 4)
	if err := r.read(GPIO_BASE, GPIO_BULK, buf, delay); err != nil {
		return false, false, err
	}
	mask := uint32(1) << r.c.ButtonPin
	return (binary.BigEndian.Uint32(buf) & mask) == 0, (binary.BigEndian.Uint32(flags) & mask) != 0, nil
}

func (r *Rotary) LED(c Color) error {
	buf := []byte{0, 0, byte((c >> 16) & 0xff), byte((c >> 8) & 0xff), byte(c & 0xff)}
	if err := r.write(NEOPIXEL_BASE, NEOPIXEL_BUF, buf); err != nil {
		return err
	}
	if err := r.write(NEOPIXEL_BASE, NEOPIXEL_SHOW, nil); err != nil {
		return err
	}
	return nil
}

func (r *Rotary) Close() {
	r.write(ENCODER_BASE, ENCODER_INTENCLR, []byte{0x01})
	mask := uint32(1) << uint32(r.c.ButtonPin)
	cmd := make([]byte, 4)
	binary.BigEndian.PutUint32(cmd, mask)
	r.write(GPIO_BASE, GPIO_INTENCLR, cmd)
	r.bus.Close()
}

func (r *Rotary) write(base, reg byte, extra []byte) error {
	b := make([]byte, 2, 2+len(extra))
	b[0], b[1] = base, reg
	b = append(b, extra...)
	c, err := r.bus.WriteBytes(b)
	if err != nil {
		return err
	}
	if exp := len(b); exp != c {
		return fmt.Errorf("expected to write %d bytes, wrote %d", exp, c)
	}
	return nil
}

func (r *Rotary) read(base, reg byte, buf []byte, delay time.Duration) error {
	if err := r.write(base, reg, nil); err != nil {
		return err
	}
	<-time.After(delay)
	c, err := r.bus.ReadBytes(buf)
	if err != nil {
		return err
	}
	if exp := len(buf); exp != c {
		return fmt.Errorf("expected to read %d bytes, read %d", exp, c)
	}
	return nil
}
