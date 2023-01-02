package main

import (
	"fmt"
	"net"
	"os"
	"time"
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

	subscribers := Subscribers{wait_time: time.Millisecond * time.Duration(500), tolerance: 5}
	go subscribers.Registrar(server)

	for {
		time.Sleep(time.Duration(100) * time.Millisecond)
		subscribers.SendCode(byte(activate_code))
		time.Sleep(time.Duration(100) * time.Millisecond)
		subscribers.SendCode(byte(deactivate_code))

	}
}
