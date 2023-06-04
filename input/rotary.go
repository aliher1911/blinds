package input

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/aliher1911/go-i2c"
)

type Rotary struct {
	bus *i2c.I2C
}

const (
	GPIO_BASE = 0x01

	GPIO_DIRSET_BULK = 0x02
	GPIO_DIRCLR_BULK = 0x03
	GPIO_BULK        = 0x04
	GPIO_BULK_SET    = 0x05
	GPIO_BULK_CLR    = 0x06
	GPIO_PULLENSET   = 0x0B
	GPIO_PULLENCLR   = 0x0C

	ENCODER_BASE = 0x11

	ENCODER_STATUS   = 0x00
	ENCODER_INTENSET = 0x10
	ENCODER_INTENCLR = 0x20
	ENCODER_POSITION = 0x30
	ENCODER_DELTA    = 0x40

	NEOPIXEL_BASE = 0x0E

	NEOPIXEL_PIN        = 0x01
	NEOPIXEL_BUF_LENGTH = 0x03
	NEOPIXEL_BUF        = 0x04
	NEOPIXEL_SHOW       = 0x05
)

const delay = 8 * time.Millisecond
const neopixel_pin = 6
const button_pin = 24

func NewRotary() (*Rotary, error) {
	bus, err := i2c.NewI2C(0x36, 1)
	if err != nil {
		return nil, err
	}

	s := &Rotary{
		bus: bus,
	}

	// Setup button pin to INPUT_PULLUP
	mask := uint32(1) << button_pin
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

	// Setup neopixel
	if err := s.write(NEOPIXEL_BASE, NEOPIXEL_PIN, []byte{neopixel_pin}); err != nil {
		return nil, err
	}
	// buf length is 3 = 1 LED with 3 bpp, encoded as short big endian
	//if err := s.write(NEOPIXEL_BASE, NEOPIXEL_BUF_LENGTH, []byte{3, 0}); err != nil {
	//	return nil, err
	//}

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

func (r *Rotary) Button() (bool, error) {
	buf := make([]byte, 4)
	if err := r.read(GPIO_BASE, GPIO_BULK, buf, delay); err != nil {
		return false, err
	}
	mask := uint32(1) << button_pin
	return (binary.BigEndian.Uint32(buf) & mask) == 0, nil
}

func (r *Rotary) LED(red, green, blue uint) error {
	buf := []byte{0, 0, byte(green), byte(red), byte(blue)}
	if err := r.write(NEOPIXEL_BASE, NEOPIXEL_BUF, buf); err != nil {
		return err
	}
	if err := r.write(NEOPIXEL_BASE, NEOPIXEL_SHOW, nil); err != nil {
		return err
	}
	return nil
}

func (r *Rotary) Close() {
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
