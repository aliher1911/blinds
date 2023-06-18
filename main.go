package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/aliher1911/blinds/cli"

	logger "github.com/d2r2/go-logger"
	rpio "github.com/stianeikeland/go-rpio/v4"
)

func LogInterrupts(ctx context.Context, intr <-chan interface{}) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-intr:
			fmt.Println("interrupt fired")
		}
	}
}

func main() {
	logger.ChangePackageLogLevel("i2c", logger.InfoLevel)
	var bus uint
	var angle int
	var baseAngle int

	flag.UintVar(&bus, "bus", 1, "provide i2c bus id")
	flag.IntVar(&angle, "angle", 0, "rotate to desired angle")
	flag.IntVar(&baseAngle, "base-angle", 0, "physical angle that is treated as zero (-180, 180)")

	flag.Parse()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt)

	err := rpio.Open()
	if err != nil {
		fmt.Printf("failed to open GPIO: %s\n", err)
		return
	}
	defer rpio.Close()

	switch flag.Arg(0) {
	case "read":
		cli.GetAngle(bus)
	case "set":
		cli.SetAngle(bus, int32(baseAngle), int32(angle))
	case "ui-test":
		cli.CliTest(bus, sigs)
	case "service":
		cli.Service(bus, int32(baseAngle), sigs)
	case "int-debug":
		cli.IntDebug(bus, sigs)
	case "":
		fmt.Print(`supported commands:

read      - read absolute angle of shaft
set       - set shaft angle using angle and optional base-angle flags to rotate to new position
ui-test   - run ui test to check controls, led and interrupts
service   - run service which sets shaft angle in response to rotary controls
int-debug - debug interrupt handling
`)
	default:
		fmt.Printf("unknown command: %s\n", flag.Arg(0))
	}
}
