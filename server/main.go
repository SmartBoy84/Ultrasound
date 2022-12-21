package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/warthog618/gpiod"
	"github.com/warthog618/gpiod/device/rpi"
)

const (
	activate_code   = 4
	deactivate_code = 5
)

func main() {

	if len(os.Args) != 2 {
		fmt.Println("./server [1-65535]")
		return
	}

	server, err := net.ResolveUDPAddr("udp", "0.0.0.0:"+os.Args[1])
	if err != nil {
		panic(err)
	}

	subscribers := Subscribers{wait_n: 3, wait_ms: 1000, tolerance: 3}

	go subscribers.Registrar(server)

	led, err := gpiod.RequestLine("gpiochip0", rpi.GPIO4, gpiod.AsOutput(0))
	if err != nil {
		panic(err)
	}
	for {
		time.Sleep(time.Duration(100) * time.Millisecond)
		led.SetValue(1)
		time.Sleep(time.Duration(100) * time.Millisecond)
		led.SetValue(0)

	}

	// sensor.Echo()

	// state := false
	// for {
	// 	time.Sleep(time.Duration(500) * time.Millisecond)

	// 	if state = !state; state {
	// 		subscribers.SendCode(byte(activate_code))
	// 	} else {
	// 		subscribers.SendCode(byte(deactivate_code))
	// 	}

	// }
}
