package sensor

import "math"

type Position struct {
	m         *Magnetometer
	baseAngle float32
}

// Angle is typically the missle of the range.
// For our case is the horizontal position.
func NewPositionSensor(m *Magnetometer, angle float32) Position {
	return Position{
		m:         m,
		baseAngle: angle,
	}
}

func (p Position) Read() (float32, error) {
	x, y, _, err := p.m.Read()
	if err != nil {
		return 0, err
	}
	sensorRadians := math.Atan2(float64(y), float64(x))
	sensorDegrees := sensorRadians * 180 / math.Pi
	shaftAngle := sensorDegrees - float64(p.baseAngle)
	switch {
	case shaftAngle > 180:
		shaftAngle -= 360
	case shaftAngle < -180:
		shaftAngle += 360
	}
	return float32(shaftAngle), nil
}
