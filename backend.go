package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

/*

Each client causes server to make a separate connection for it
When client wants to send something to the server, it spins up a new connection on a new port
Server receives request and offloads it to a goroutine
The goroutine checks if the request is a ping message, if so it echoes the message
Otherwise if the request is a ping response (not tagged with a ping header), it adds it to a channel on the struct (doesn't matter if it deadlocks until Heartbeat() runs again)
Finally, if the request is none of the above, it runs the callback function in another go routine

*/

func createTempConn() (*net.UDPConn, error) {
	tempRemote, err := net.ListenUDP("udp", resolve(":0")) // :0 gets as [::]:[random free port (in legal range obv)]
	if err != nil {
		return nil, err
	}

	return tempRemote, nil
}

func resolve(addr string) *net.UDPAddr {
	server, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		panic(err)
	}
	return server
}

// func verifyRemote(old *net.UDPAddr, new *net.UDPAddr) bool {
// 	return old.IP.String() == new.IP.String() && old.Port == new.Port
// }

func (settings *Settings) GetWait() time.Duration {
	return settings.wait_time + (time.Duration(rand.Intn(1000)))*time.Millisecond
}

func TimeMeOut(conn *net.UDPConn, timeLimit time.Duration) {
	conn.SetDeadline(time.Now().Add(timeLimit)) // UDP is connectionless, remember? This is all up to us
}

type MiddleMan struct {
	Target      *net.UDPAddr
	PrimaryConn *net.UDPConn

	settings *Settings

	lostConnection func(error)
	end            chan bool // token to termine digest()
	pings          chan bool
}

type Settings struct {
	mu        sync.Mutex
	wait_time time.Duration
	tolerance int // max amount of connection failures before client is removed all together

	failCounter       int
	messageReceivedFn func(target *net.UDPAddr, state []byte)
}

func (middleMan *MiddleMan) SendMessage(message byte) (err error) {

	tempRemote, err := createTempConn()
	if err != nil {
		return err
	}
	defer tempRemote.Close()

	for i := 0; i < middleMan.settings.tolerance; i++ {
		if err := middleMan.PingPong(message, tempRemote); err != nil {
			break
		}
	}

	return err
}

func (middleMan *MiddleMan) read_status(status byte, tempRemote *net.UDPConn) error {

	read := []byte{0, 0}

	var err error
	var rlen int

	TimeMeOut(tempRemote, time.Duration(middleMan.settings.GetWait()))
	if rlen, _, err = tempRemote.ReadFromUDP(read[:]); err != nil {
		return err
	}

	if rlen == 0 {
		return errors.New("incoming message length cannot be 0")
	}

	if read[1] != status {
		return errors.New("malformed message")
	}

	return nil
}

func (middleMan *MiddleMan) PingPong(message byte, tempRemote *net.UDPConn) (err error) {

	fmt.Printf("Messaging on %s\n", tempRemote.LocalAddr())

	TimeMeOut(tempRemote, time.Duration(middleMan.settings.GetWait()))
	if _, err = tempRemote.WriteToUDP([]byte{ping_code, message}, middleMan.Target); err == nil {
		if err = middleMan.read_status(message, tempRemote); err == nil {
			return nil
		}
	}

	if err, ok := err.(net.Error); err != nil && !(ok && err.Timeout()) { // ugh, it's SO DAMN UGLY!
		time.Sleep(middleMan.settings.GetWait())
	}

	fmt.Print(err)

	return err
}

func (middleMan *MiddleMan) KillConn(err error) {
	middleMan.end <- true
	middleMan.lostConnection(err)
}

func (middleMan *MiddleMan) Heartbeat() {

	var err error
	middleMan.end = make(chan bool, 1)
	middleMan.pings = make(chan bool, 1)

	heartbeatConn, err := createTempConn()
	if err != nil {
		middleMan.KillConn(err)
		return
	}
	defer heartbeatConn.Close()

	for {

		time.Sleep(middleMan.settings.GetWait())
		middleMan.settings.mu.Lock()

		if err = middleMan.PingPong(ping_code, heartbeatConn); err != nil {
			middleMan.settings.failCounter++
			fmt.Printf("[WARNING] %s issued warning %d/%d!\n", middleMan.Target.String(), middleMan.settings.failCounter, middleMan.settings.tolerance)

		} else {
			fmt.Printf("Responded?")
			if middleMan.settings.failCounter > 0 {
				middleMan.settings.failCounter = 0

				fmt.Printf("Phew, %s recovered!\n", middleMan.PrimaryConn.LocalAddr().String())
			}
		}

		middleMan.settings.mu.Unlock()

		if middleMan.settings.failCounter >= middleMan.settings.tolerance {
			middleMan.KillConn(err)
			return
		}
	}
}

// this function blocks
func (middleMan *MiddleMan) Digest() error {

	read := []byte{0, 0}

	// go middleMan.monitor()

	// readUDP variables
	var rlen int
	var incoming *net.UDPAddr
	var err error

	for {

		if len(middleMan.end) > 0 {
			fmt.Println("Deregistration!")
			return errors.New("deregistered")
		}

		if rlen, incoming, err = middleMan.PrimaryConn.ReadFromUDP(read); err != nil {

			go func() {
				if err != nil {
					if err, ok := err.(net.Error); err != nil && (!ok || !err.Timeout()) { // ugh, it's SO DAMN UGLY!
						fmt.Printf("error: %s\n", err.Error())

					}
				}
			}()

			continue
		}

		go func() {
			if rlen == 0 {

				fmt.Println("incoming message length cannot be 0")

			}

			if middleMan.settings.messageReceivedFn != nil {
				go middleMan.settings.messageReceivedFn(incoming, read[1:])
			}

			if int(read[0]) == ping_code {
				fmt.Printf("Ping from %s", incoming)
				var err error

				for i := 0; i < middleMan.settings.tolerance; i++ {

					tempRemote, err := createTempConn()
					if err != nil {
						fmt.Print(err)
					}

					TimeMeOut(tempRemote, time.Duration(middleMan.settings.GetWait()))
					if _, err = tempRemote.WriteToUDP([]byte{0, read[1]}, incoming); err == nil {
						return
					}

					if err, ok := err.(net.Error); err != nil && (!ok || !err.Timeout()) { // ugh, it's SO DAMN UGLY!
						time.Sleep(middleMan.settings.GetWait())
					}
				}

				if err != nil {
					fmt.Printf("pong error: %s", err.Error())
				}
			}
		}()
	}
}
