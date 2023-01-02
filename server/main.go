package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

const activate_code = 4

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

	r := bufio.NewReader(os.Stdin)
	buf := make([]byte, 2)
	received := false

	for {
		n, err := r.Read(buf)

		if n > 0 && !received { // if this is the first time receiving data after last "end of data"
			fmt.Printf("Data!")
			subscribers.SendCode(byte(activate_code))

		}

		if n == 2 && !received { // if we have received a "whole" "packet" and was first time we received data after last "end of data"
			received = true
		}

		if n == 1 && received { // length == 1, means we received 1 character or we have reached end of current data stream
			received = false
		}

		if n == 0 {
			if err == nil {
				continue
			}

			if err == io.EOF {
				break
			}

			fmt.Print(err)
		}
	}
}
