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
	CallbackFn func(code int)

	Target *net.UDPAddr
	Remote *net.UDPConn
}

func verifyRemote(old *net.UDPAddr, new *net.UDPAddr) bool {
	return old.IP.String() == new.IP.String() && old.Port == new.Port
}

func TimeMeOut(conn *net.UDPConn, timer time.Duration) {
	conn.SetDeadline(time.Now().Add(timer)) // UDP is connectionless, remember? This is all up to us
}

func (sensor *Sensor) monitor(ping <-chan bool, end chan<- bool) {

	for {
		time.Sleep(time.Duration(1000+rand.Intn(1000)) * time.Millisecond)

		select {

		case <-ping:
			continue

		default:

			fmt.Println("Connection to server lost!")
			end <- true
		}
	}
}

func (sensor *Sensor) pong() error {

	fmt.Printf("Done! Connection to server established\n\n")

	read := []byte{0}
	status := make(chan bool, 1)
	end := make(chan bool, 1)

	go sensor.monitor(status, end)

	// readUDP variables
	var rlen int
	var incoming *net.UDPAddr
	var err error

	for {

		if len(end) > 0 {
			fmt.Println("Deregistration!")
			return errors.New("deregistered")
		}

		if err != nil {
			if err, ok := err.(net.Error); err != nil && (!ok || !err.Timeout()) { // ugh, it's SO DAMN UGLY!
				fmt.Printf("error: %s\n", err.Error())
			}

			err = nil
		}

		if rlen, incoming, err = sensor.Remote.ReadFromUDP(read); err != nil {
			continue
		}

		if rlen == 0 {

			err = errors.New("incoming message length cannot be 0")
			continue
		}

		if !verifyRemote(incoming, sensor.Target) {

			err = errors.New("this ain't the server or me talkin boss")
			continue
		}

		switch message := int(read[0]); message {

		case ping_code:

			if len(status) == 0 {
				status <- true // calm the watchdog
			}

		case deregister_code:

			fmt.Println("Deregistration!")
			return errors.New("deregistered")

		default:

			if sensor.CallbackFn != nil {
				go sensor.CallbackFn(message)
			}
		}

		go func() {
			TimeMeOut(sensor.Remote, time.Second)
			if _, err = sensor.Remote.WriteToUDP(read, sensor.Target); err != nil {
				fmt.Printf("pong error: %s", err.Error())
			}
		}()
	}
}

func GetFreePort() (port int, err error) {
	for {
		var a *net.TCPAddr
		if a, err = net.ResolveTCPAddr("tcp", "0.0.0.0:0"); err != nil {
			break
		}

		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err != nil {
			defer l.Close()
			break
		}

		if port := l.Addr().(*net.TCPAddr).Port; port > 300 {
			break
		}
	}

	return port, err
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

		TimeMeOut(server, time.Second)
		if _, err = server.WriteToUDP([]byte{ping_code}, remote); err != nil {
			break
		}

		TimeMeOut(server, time.Second)
		if _, _, err = server.ReadFromUDP(message[:]); err != nil {
			break
		}

		if int(message[0]) == ping_code {
			break
		} else {
			err = errors.New("wrong probe reply")
		}
	}

	if err != nil {
		return err
	}

	// establish connection
	fmt.Print("Waiting for registration to go through")

	for i := 0; i < 5; i++ {

		fmt.Print(".")

		if err != nil {
			time.Sleep(time.Millisecond) // for loop only goes past 1 when err != nil

			fmt.Println(err.Error())
			err = nil
		}

		if _, err = server.WriteToUDP([]byte{register_code}, remote); err != nil {
			continue
		}

		fmt.Println("\nRequest sent!")

		if _, remote, err = server.ReadFromUDP(message[:]); err != nil { // yes, I am aware I'm overriding function argument
			continue
		}

		if message[0] != register_code {

			err = errors.New("server didn't respond to registration request")
			continue
		}

		fmt.Printf("Acknowledgement received: %s\n", remote.String())

		if _, err = server.WriteToUDP([]byte{register_code}, remote); err != nil {
			continue
		}

		server.SetReadDeadline(time.Time{}) // reset timeout

		sensor.Remote = server
		sensor.Target = remote

		sensor.pong() // this blocks, start it up ASAP to send back final registration ping to server
		return nil
	}

	if err == nil {
		err = errors.New("timed out")
	}

	fmt.Println()
	return err
}
