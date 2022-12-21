package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"time"
)

const (
	ping_code       = 1
	register_code   = 2
	deregister_code = 3
)

type Sensor struct {
	State      int
	CallbackFn func(code int)

	Target *net.UDPAddr
	Local  *net.UDPAddr
	Remote *net.UDPConn
}

func (sensor *Sensor) monitor(ping <-chan bool) {
	for {
		time.Sleep(time.Duration(5000+rand.Intn(1000)) * time.Millisecond)

		select {
		case <-ping:
			continue
		default:
			fmt.Println("Connection to server lost!")

			var err error
			var localConn *net.UDPConn

			if localConn, err = net.DialUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")}, sensor.Local); err == nil {
				if _, err = localConn.Write([]byte{deregister_code}); err == nil {
					return // only stop monitoring NOW
				}
			}

			fmt.Print(err.Error())
		}
	}
}

func (sensor *Sensor) pong() error {

	fmt.Printf("Done! Connection to server established\n\n")

	read := []byte{0}
	status := make(chan bool, 1)

	go sensor.monitor(status)

	for {
		rlen, incoming, err := sensor.Remote.ReadFromUDP(read)

		if err == nil {

			if rlen == 0 {
				err = errors.New("incoming message length cannot be 0")

			} else if !(incoming.IP.String() == "127.0.0.1" || (incoming.IP.String() == sensor.Target.IP.String() && incoming.Port == sensor.Target.Port)) {
				err = errors.New("this ain't the server or me talkin boss")

			} else {

				sensor.State = int(read[0])

				if sensor.State == ping_code {

					if len(status) == 0 {
						status <- true
					}

				} else if sensor.State == deregister_code {
					fmt.Println("Deregistration!")
					return errors.New("deregistered")

				} else {

					if sensor.CallbackFn != nil {
						sensor.CallbackFn(sensor.State)
					}
				}

				if _, err = sensor.Remote.WriteToUDP(read, sensor.Target); err != nil {
					err = fmt.Errorf("pong error: %s", err.Error())

				}
			}
		}

		if err != nil {
			fmt.Printf("error: %s\n", err.Error())

		}
	}
}

func GetFreePort() (port int, err error) {
	for {
		var a *net.TCPAddr
		if a, err = net.ResolveTCPAddr("tcp", "0.0.0.0:0"); err == nil {
			var l *net.TCPListener
			if l, err = net.ListenTCP("tcp", a); err == nil {
				defer l.Close()
				if port := l.Addr().(*net.TCPAddr).Port; port > 300 {
					return port, nil
				}
			}
		}
	}
}

func (sensor *Sensor) Register(remote *net.UDPAddr) (err error) {

	// create new local listener address
	local := &net.UDPAddr{IP: net.ParseIP("0.0.0.0")}
	if local.Port, err = GetFreePort(); err != nil {
		fmt.Printf("No free port? %s\n", err.Error())
		return err
	}

	// check server status before continuing
	var server *net.UDPConn
	server, err = net.ListenUDP("udp", local)
	if err != nil {
		return err
	}
	defer server.Close()

	message := []byte{0}
	for i := 0; i < 5; i++ {

		if _, err = server.WriteToUDP([]byte{ping_code}, remote); err == nil {

			server.SetDeadline(time.Now().Add(time.Duration(1000) * time.Millisecond)) // UDP is connectionless, remember? This is all up to us
			if _, _, err = server.ReadFromUDP(message[:]); err == nil {

				if int(message[0]) == ping_code {
					break
				} else {
					err = errors.New("wrong probe reply")
				}
			}
		}
	}
	if err != nil {
		return err
	}

	// establish connection
	fmt.Print("Waiting for registration to go through")

	for i := 0; i < 5; i++ {

		if _, err = server.WriteToUDP([]byte{register_code}, remote); err == nil {
			fmt.Println("\nRequest sent!")

			message := []byte{0}
			if _, remote, err = server.ReadFromUDP(message[:]); err == nil {

				if message[0] == register_code {

					fmt.Printf("Acknowledgement received: %s\n", remote.String())
					server.Close()

					if server, err = net.ListenUDP("udp", local); err == nil {
						fmt.Println("Contact!")

						if _, err = server.WriteToUDP([]byte{register_code}, remote); err == nil {

							sensor.Remote = server
							sensor.Target = remote
							sensor.Local = local // for when I need to stop pong()

							sensor.pong() // // I.e., this will block
							return nil
						}
					}

				} else {
					err = errors.New("server didn't respond")

				}
			}
		}

		if err != nil {
			fmt.Println(err.Error())
		}

		time.Sleep(time.Millisecond * 1000)
		fmt.Print(".")
	}
	fmt.Println()

	if err == nil {
		return errors.New("timed out")
	}

	return err
}
