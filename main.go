package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	ping_code       = 1
	register_code   = 2
	deregister_code = 3

	activate_code   = 4
	deactivate_code = 5
)

func main() {

	if len(os.Args) < 2 || len(os.Args) != 4 || len(strings.Split(os.Args[1], ":")) > 2 {
		fmt.Println("./server \n    [x.x.x.x]:[1-65535] *only providing a port runs this in server mode\n    activate:\"{[bash program] [args]}\"\n    deactivate:\"{[program] [args]}\"")
		return
	}

	cmds := [...][]string{strings.Split(os.Args[2], " "), strings.Split(os.Args[3], " ")}

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

	notify := func(target *net.UDPAddr, message []byte) {

		fmt.Printf("State %d, client %s\n", message, target.String())

		if message[0] == activate_code {
			go runCmd(cmds[1])
		} else if message[0] == deactivate_code {
			go runCmd(cmds[0])
		}

	}

	settings := &Settings{wait_time: time.Millisecond * time.Duration(500), tolerance: 1, messageReceivedFn: notify}

	// var runner func(byte) error

	var sendWait sync.WaitGroup

	if len(strings.Split(os.Args[1], ":")) == 1 {

		fmt.Println("I am the captain now!")

		server := resolve("0.0.0.0:" + os.Args[1])
		subscribers := NewSubscriberList(settings)

		// runner = subscribers.SendCode

		sendWait.Add(1)
		go subscribers.StartRegistrar(server)

		for {
			time.Sleep(subscribers.settings.GetWait())
			subscribers.SendMessage(activate_code)
		}

	} else {

		fmt.Println("Oh! I am but a humble peasant")

		server := resolve(os.Args[1])
		myself := NewClient(settings)
		// runner = myself.SendCode

		sendWait.Add(1)
		go func() {
			fmt.Printf("Connecting to %s\n", server)

			for i := 0; i < myself.settings.tolerance; i++ {

				if err := myself.Register(server); err != nil {
					fmt.Println(err)
				} else {
					i = 0
				}

				time.Sleep(settings.GetWait())
			}

			fmt.Println("\nI'm tired of waiting, goodbye!")
			sendWait.Done()
		}()

	}

	// r := bufio.NewReader(os.Stdin)
	// buf := make([]byte, 1)
	// // buf := make([]byte, 2)
	// // received := false

	// for {
	// 	n, err := r.Read(buf)

	// 	if len(subscribers.list) == 0 {
	// 		continue
	// 	}

	// 	switch string(buf[0]) {
	// 	case "0":
	// 		fmt.Printf("False!")
	// 		subscribers.SendCode(byte(deactivate_code))
	// 	case "1":
	// 		fmt.Printf("True!")
	// 		subscribers.SendCode(byte(activate_code))
	// 	}

	// 	if n == 0 {
	// 		if err == nil {
	// 			continue
	// 		}

	// 		if err == io.EOF {
	// 			break
	// 		}

	// 		fmt.Print(err)
	// 	}
	//}

	sendWait.Wait()
}
