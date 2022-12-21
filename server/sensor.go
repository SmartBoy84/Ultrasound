package main

import (
	"fmt"
	"time"

	"github.com/warthog618/gpiod"
)

func delayNs(d time.Duration) {
	time.Sleep(d * time.Nanosecond)
}

func delayMs(d time.Duration) {
	time.Sleep(d * time.Millisecond)
}

type Sensor struct {
	trig *gpiod.Line
	echo *gpiod.Line
}

func NewSensor(trig int, echo int) (*Sensor, error) {

	/* trigger := rpi.J8p7
	echo := rpi.J8p11
	j8p7 (the seventh pin on the long j8 header) could also use rpi.GPIO4
	the raspberry pi GPIO header is referred to has J8 */

	fmt.Printf("Initialising sensor")

	sensor := &Sensor{}
	var err error

	sensor.echo, err = gpiod.RequestLine("gpiochip0", echo, gpiod.AsOutput(0))
	if err != nil {
		return nil, err
	}

	sensor.trig, err = gpiod.RequestLine("gpiochip0", trig, gpiod.WithPullDown, gpiod.AsInput)
	if err != nil {
		return nil, err
	}

	return sensor, nil
}

func (sensor *Sensor) Echo() (err error) {

	fmt.Printf("Echo!")

	// pulse trigger
	sensor.trig.SetValue(0) // in case it was set high somewhere and not set low again
	if err != nil {         // probably safe to only handle once?
		return err
	}

	delayNs(1)
	sensor.trig.SetValue(1) // trigger must be low, for high switch to be detected
	delayNs(1)
	sensor.trig.SetValue(0) // set it low again
	// delayUs(1)

	// pulse will be sent any moment now...
	var i, state int

	for i = 0; state != 1; i++ {

		if state, err = sensor.echo.Value(); err != nil {
			return err
		}

		delayMs(1)
	}
	fmt.Printf("Echo became high after %dms", i) // should be a low value

	for i = 0; state != 0; i++ {

		if state, err = sensor.echo.Value(); err != nil {
			return err
		}

		delayMs(1)
	}
	fmt.Printf("Echo became high after %dms", i) // should be a low value

	return nil
}

func (sensor *Sensor) Close() {

	// revert line to input on the way out.
	sensor.trig.Reconfigure(gpiod.AsInput)
	sensor.echo.Reconfigure(gpiod.AsInput)

	// free up gpio memory
	sensor.trig.Close()
	sensor.echo.Close()
}
