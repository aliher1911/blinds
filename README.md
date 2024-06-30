# Controller for window blinds

## Hardware

- RaspberryPI Zero W
- Some random stepper via driver on GPIO pins
- Adafruit rotary encoder breakout (using seesaw) i2c
- Adafruit magnetometer i2C

### Power
- PSU jack: center positive (vcc)
- Micro USB: from flat side facing down, vcc on right, ground on left

## Software

Included ;-)

### Calibration
cli commands to:
- read angle
- set angle
- once done, set desired values in config file
