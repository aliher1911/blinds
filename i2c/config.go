package i2cdev

type Conf struct {
	Bus  int
	Addr uint8
}

func (c *Conf) Default(a uint8) {
	if c.Addr == 0 {
		c.Addr = a
	}
}
