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

	if len(os.Args) < 2 || len(os.Args) != 4 || len(strings.Split(os.Args[1], ":")) != 2 {
		fmt.Println("./server \n    [x.x.x.x]:[1-65535]\n    activate:\"{[bash program] [args]}\"\n    deactivate:\"{[program] [args]}\"")
		return
	}

	cmds := [...][]string{strings.Split(os.Args[2], " "), strings.Split(os.Args[3], " ")}

	server, err := net.ResolveUDPAddr("udp", os.Args[1])
	if err != nil {
		fmt.Printf("Unable to resolve ip:port - %s\n", err.Error())
		return
	}

	runCmd := func(cmd []string) {
		var command *exec.Cmd

		if len(cmd) == 1 {
			command = exec.Command(cmd[0])
		} else if len(cmd) > 1 {
			command = exec.Command(cmd[0], cmd[1:]...)
		}

		fmt.Printf("Running command: %s\n", strings.Join(cmd, " "))
		err := command.Run()

		if err != nil {
			fmt.Println(err.Error())
		}
	}

	myroom := Sensor{CallbackFn: func(state int) {

		if state == activate_code {
			go runCmd(cmds[1])
		} else if state == deactivate_code {
			go runCmd(cmds[0])
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
