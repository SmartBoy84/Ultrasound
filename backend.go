package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

func resolve(addr string) *net.UDPAddr {
	server, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		panic(err)
	}
	return server
}

func verifyRemote(old *net.UDPAddr, new *net.UDPAddr) bool {
	return old.IP.String() == new.IP.String() && old.Port == new.Port
}

func (settings *Settings) GetWait() time.Duration {
	return settings.wait_time + (time.Duration(rand.Intn(1000)))*time.Millisecond
}

func TimeMeOut(conn *net.UDPConn, timeLimit time.Duration) {
	conn.SetDeadline(time.Now().Add(timeLimit)) // UDP is connectionless, remember? This is all up to us
}

type MiddleMan struct {
	Target *net.UDPAddr
	Remote *net.UDPConn

	settings *Settings

	lostConnection func(error)
	end            chan bool // token to termine digest()
}

type Settings struct {
	mu        sync.Mutex
	wait_time time.Duration
	tolerance int // max amount of connection failures before client is removed all together

	failCounter       int
	messageReceivedFn func(target *net.UDPAddr, state []byte)
}

func (middleMan *MiddleMan) SendMessage(message byte) (err error) {

	for i := 0; i < middleMan.settings.tolerance; i++ {
		if err := middleMan.PingPong(message); err != nil {
			break
		}
	}

	return err
}

func (middleMan *MiddleMan) ReadStatus(status byte) error {

	read := []byte{0, 0}

	var err error
	var rlen int

	TimeMeOut(middleMan.Remote, time.Duration(middleMan.settings.GetWait()))
	if rlen, _, err = middleMan.Remote.ReadFromUDP(read[:]); err != nil {
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

func (middleMan *MiddleMan) PingPong(message byte) (err error) {

	TimeMeOut(middleMan.Remote, time.Duration(middleMan.settings.GetWait()))
	if _, err = middleMan.Remote.WriteToUDP([]byte{ping_code, message}, middleMan.Target); err == nil {

		if err = middleMan.ReadStatus(message); err == nil {
			return nil
		}
	}

	if err, ok := err.(net.Error); err != nil && !(ok && err.Timeout()) { // ugh, it's SO DAMN UGLY!
		time.Sleep(middleMan.settings.GetWait())
	}

	return err
}

func (middleMan *MiddleMan) Heartbeat() {

	var err error
	middleMan.end = make(chan bool, 1)

	for {

		time.Sleep(middleMan.settings.GetWait())
		middleMan.settings.mu.Lock()

		if err = middleMan.PingPong(ping_code); err != nil {
			middleMan.settings.failCounter++

			fmt.Printf("[WARNING] %s issued warning %d/%d!\n", middleMan.Target.String(), middleMan.settings.failCounter, middleMan.settings.tolerance)

		} else if middleMan.settings.failCounter > 0 {
			middleMan.settings.failCounter = 0

			fmt.Printf("Phew, %s recovered!\n", middleMan.Remote.LocalAddr().String())
		}

		middleMan.settings.mu.Unlock()

		if middleMan.settings.failCounter >= middleMan.settings.tolerance {
			middleMan.end <- true
			middleMan.lostConnection(err)
			return
		}
	}
}

// this function blocks
func (middleMan *MiddleMan) Digest() error {

	middleMan.Remote.SetReadDeadline(time.Time{}) // reset timeout

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

		if err != nil {

			if err, ok := err.(net.Error); err != nil && (!ok || !err.Timeout()) { // ugh, it's SO DAMN UGLY!
				fmt.Printf("error: %s\n", err.Error())
			}

			err = nil
		}

		if rlen, incoming, err = middleMan.Remote.ReadFromUDP(read); err != nil {
			continue
		}

		if rlen == 0 {

			err = errors.New("incoming message length cannot be 0")
			continue
		}

		if middleMan.Target != nil && !verifyRemote(incoming, middleMan.Target) {

			err = errors.New("stranger danger")
			continue
		}

		if middleMan.settings.messageReceivedFn != nil {
			go middleMan.settings.messageReceivedFn(incoming, read[1:])
		}

		if int(read[0]) == ping_code {

			go func() {

				var err error

				for i := 0; i < middleMan.settings.tolerance; i++ {

					TimeMeOut(middleMan.Remote, time.Duration(middleMan.settings.GetWait()))
					if _, err = middleMan.Remote.WriteToUDP([]byte{0, read[1]}, incoming); err == nil {
						return
					}

					if err, ok := err.(net.Error); err != nil && (!ok || !err.Timeout()) { // ugh, it's SO DAMN UGLY!
						time.Sleep(middleMan.settings.GetWait())
					}
				}

				if err != nil {
					fmt.Printf("pong error: %s", err.Error())
				}
			}()
		}
	}
}
