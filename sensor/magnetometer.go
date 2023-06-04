package sensor

import (
	"fmt"

	"github.com/aliher1911/blinds/i2c"

	"github.com/aliher1911/go-i2c"
)

const i2cAddress = 0x5e

type Magnetometer struct {
	dev *i2cdev.BulkDevice
}

const (
	BXL int = iota
	BXH
	BYL
	BYH
	BZL
	BZH
	TEMPL
	TEMPH
	FRAME_COUNTER
	CHANNEL
	TEST_MODE
	POWER_DOWN
	RREZ1
	RREZ2
	RREZ3
)

var readRegs = i2cdev.Registers{
	// BX
	i2cdev.Field{4, 4, 0b11110000},
	i2cdev.Field{0, 0, 0b11111111},
	// BY
	i2cdev.Field{4, 0, 0b00001111},
	i2cdev.Field{1, 0, 0b11111111},
	// BZ
	i2cdev.Field{5, 4, 0b00001111},
	i2cdev.Field{2, 0, 0b11111111},
	// TEMP
	i2cdev.Field{6, 0, 0b11111111},
	i2cdev.Field{3, 4, 0b00001111},
	// FRAME_COUNTER
	i2cdev.Field{3, 2, 0b00001100},
	// CHANNEL
	i2cdev.Field{3, 0, 0b00000011},
	// TEST_MODE
	i2cdev.Field{5, 6, 0b01000000},
	// POWER_DOWN
	i2cdev.Field{5, 4, 0b00010000},
	// RESERVED
	i2cdev.Field{7, 3, 0b00011000},
	i2cdev.Field{8, 0, 0b11111111},
	i2cdev.Field{9, 0, 0b00011111},
}

const (
	PARITY int = iota
	IICADDR
	INT_ENABLED
	FAST_MODE
	LOW_POWER_MODE
	TEMP_DISABLED
	LOW_POWER_PERIOD
	PARITY_TEST
	WREZ1
	WREZ2
	WREZ3
)

var writeRegs = i2cdev.Registers{
	// PARITY
	i2cdev.Field{1, 7, 0b10000000},
	// IICADDR
	i2cdev.Field{1, 5, 0b01100000},
	// INT_ENABLED
	i2cdev.Field{1, 2, 0b00000100},
	// FAST_MODE
	i2cdev.Field{1, 1, 0b00000010},
	// LOW_POWER_MODE
	i2cdev.Field{1, 0, 0b00000001},
	// TEMP_DISABLED
	i2cdev.Field{3, 7, 0b10000000},
	// LOW_POWER_PERIOD
	i2cdev.Field{3, 6, 0b01000000},
	// PARITY_TEST
	i2cdev.Field{3, 5, 0b00100000},
	// RESERVED
	i2cdev.Field{1, 3, 0b00011000},
	i2cdev.Field{2, 0, 0b11111111},
	i2cdev.Field{3, 0, 0b00011111},
}

func NewMagnetometer() (*Magnetometer, error) {
	fmt.Printf("creating magnetometer\n")

	bus, err := i2c.NewI2C(0x5e, 1)
	if err != nil {
		return nil, err
	}

	m := &Magnetometer{
		dev: i2cdev.NewBulkDevice(bus, readRegs, writeRegs),
	}

	// Copy reserved first.
	if err := m.dev.ReadBus(); err != nil {
		return nil, err
	}
	m.dev.WriteReg(WREZ1, m.dev.ReadReg(RREZ1))
	m.dev.WriteReg(WREZ2, m.dev.ReadReg(RREZ2))
	m.dev.WriteReg(WREZ3, m.dev.ReadReg(RREZ3))

	// Set up registers.
	m.dev.WriteReg(IICADDR, 0)
	m.dev.WriteReg(PARITY, 1)
	m.dev.WriteReg(FAST_MODE, 1)
	m.dev.WriteReg(LOW_POWER_MODE, 1)

	// Initialize sensor.
	if err := m.dev.WriteBus(); err != nil {
		return nil, err
	}

	return m, nil
}

const scale = 98

func (m *Magnetometer) Read() (float32, float32, float32, error) {
	if err := m.dev.ReadBus(); err != nil {
		return 0, 0, 0, err
	}

	// Ideally check that we are in correct reading phase.

	readM := func(hi, lo int) float32 {
		hv := m.dev.ReadReg(hi)
		lv := m.dev.ReadReg(lo)
		iv := (int16(hv)<<8 | int16(lv)<<4) >> 4
		return scale * float32(iv)
	}

	return readM(BXH, BXL), readM(BYH, BYL), readM(BZH, BZL), nil
}

func (m *Magnetometer) Close() {
	m.dev.Close()
}
