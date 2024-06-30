package i2cdev

import (
	"fmt"

	i2c "github.com/aliher1911/go-i2c"
)

type Field struct {
	Addr  int
	Shift byte
	Mask  byte
}

type Registers []Field

// BulkDevice allows reading packed registers of arbitrary sizes from
// I2C bus.
// Read ops first read packed byte array from the bus, then
// use configured mapping to extract packed values.
// Write ops first need to pack data into buffer then write it as a bus
// op.
type BulkDevice struct {
	bus       *i2c.I2C
	readRegs  Registers
	writeRegs Registers

	readBuf  []byte
	writeBuf []byte
}

func NewBulkDevice(bus *i2c.I2C, readRegs Registers, writeRegs Registers) *BulkDevice {
	return &BulkDevice{
		bus:       bus,
		readRegs:  readRegs,
		writeRegs: writeRegs,
		readBuf:   regSlice(readRegs),
		writeBuf:  regSlice(writeRegs),
	}
}

func regSlice(regs Registers) []byte {
	rs := 0
	for _, f := range regs {
		if f.Addr > rs {
			rs = f.Addr
		}
	}
	return make([]byte, rs+1)
}

func (d *BulkDevice) ReadReg(id int) byte {
	f := d.readRegs[id]
	v := d.readBuf[f.Addr]
	return (v & f.Mask) >> f.Shift
}

func (d *BulkDevice) WriteReg(id int, v byte) {
	f := d.writeRegs[id]
	v = (v << f.Shift) & f.Mask
	d.writeBuf[f.Addr] = (d.writeBuf[f.Addr] & ^f.Mask) | v
}

func (d *BulkDevice) ReadBus() error {
	c, err := d.bus.ReadBytes(d.readBuf)
	if err != nil {
		return err
	}
	if exp := len(d.readBuf); exp != c {
		return fmt.Errorf("expected to read %d bytes, read %d", exp, c)
	}
	return nil
}

func (d *BulkDevice) WriteBus() error {
	c, err := d.bus.WriteBytes(d.writeBuf)
	if err != nil {
		return err
	}
	if exp := len(d.writeBuf); exp != c {
		return fmt.Errorf("expected to write %d bytes, wrote %d", exp, c)
	}
	return nil
}

func (d *BulkDevice) Close() {
	d.bus.Close()
}
