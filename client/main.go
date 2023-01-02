package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	activate_code   = 4
	deactivate_code = 5
)

func main() {
	if len(os.Args) < 2 || len(strings.Split(os.Args[1], ":")) != 2 {
		fmt.Println("./server [x.x.x.x]:[1-65535] {[bash program] [args]}")
		return
	}

	server, err := net.ResolveUDPAddr("udp", os.Args[1])
	if err != nil {
		fmt.Printf("Unable to resolve ip:port - %s\n", err.Error())
		return
	}

	runCmd := func() {
		var cmd *exec.Cmd

		if len(os.Args) == 3 {
			cmd = exec.Command(os.Args[2])
		} else {
			cmd = exec.Command(os.Args[2], os.Args[3:]...)
		}

		fmt.Printf("Running command: %s\n", cmd.String())
		err := cmd.Run()

		if err != nil {
			fmt.Println(err.Error())
		}
	}

	myroom := Sensor{CallbackFn: func(state int) {
		if state == activate_code {

			if len(os.Args) > 3 {

				go runCmd()
			}
		}
	}}

	for {
		if err := myroom.Register(server); err != nil {
			fmt.Println(err)
		}

		fmt.Println("Trying to reconnect")
		time.Sleep(time.Duration(1000) * time.Millisecond)
	}
}
