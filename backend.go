package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

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

func (settings *Settings) GetWait() time.Duration {
	return settings.wait_time + (time.Duration(rand.Intn(1000)))*time.Millisecond
}

func TimeMeOut(conn *net.UDPConn, timeLimit time.Duration) {
	conn.SetDeadline(time.Now().Add(timeLimit)) // UDP is connectionless, remember? This is all up to us
}

type MiddleMan struct {
	mu sync.Mutex

	Target      *net.UDPAddr
	PrimaryConn *net.UDPConn

	settings *Settings

	lostConnection func(error)
	failCounter    int
	end            chan bool // token to termine digest()
}

type Settings struct {
	wait_time time.Duration
	tolerance int // max amount of connection failures before client is removed all together

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
			fmt.Print(err)
		} else {
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

	TimeMeOut(tempRemote, time.Duration(middleMan.settings.GetWait()))
	if _, err = tempRemote.WriteToUDP([]byte{ping_code, message}, middleMan.Target); err == nil {
		if err = middleMan.read_status(message, tempRemote); err == nil {
			return nil
		}
	}

	if err, ok := err.(net.Error); err != nil && !(ok && err.Timeout()) { // ugh, it's SO DAMN UGLY!
		time.Sleep(middleMan.settings.GetWait())
	}

	return err
}

func (middleMan *MiddleMan) KillConn(err error) {
	middleMan.end <- true
	middleMan.PrimaryConn.Close() // order matters!
	if middleMan.lostConnection != nil {
		middleMan.lostConnection(err) // placing this last ensures user doesn't accidently use a confirmed dead connection
	}
}

func (middleMan *MiddleMan) Heartbeat() {

	var err error
	middleMan.end = make(chan bool, 1)

	heartbeatConn, err := createTempConn()
	if err != nil {
		middleMan.KillConn(err)
		return
	}
	defer heartbeatConn.Close()

	for {

		time.Sleep(middleMan.settings.GetWait())
		middleMan.mu.Lock()

		if err = middleMan.PingPong(ping_code, heartbeatConn); err != nil {
			middleMan.failCounter++
			fmt.Printf("[WARNING] %s issued warning %d/%d!\n", middleMan.Target.String(), middleMan.failCounter, middleMan.settings.tolerance)

		} else if middleMan.failCounter > 0 {
			middleMan.failCounter = 0

			fmt.Printf("Phew, %s recovered!\n", middleMan.Target)
		}

		middleMan.mu.Unlock()

		if middleMan.failCounter >= middleMan.settings.tolerance {
			heartbeatConn.Close() // for some reason I have to do it here as well
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

	middleMan.PrimaryConn.SetDeadline(time.Time{}) // this line is the fix to a bug that took me three hours to find

	for {
		if rlen, incoming, err = middleMan.PrimaryConn.ReadFromUDP(read); err != nil {

			if len(middleMan.end) > 0 {
				fmt.Println("Deregistration!")
				return errors.New("deregistered")
			}

			go func() {
				if err, ok := err.(net.Error); err != nil && (!ok || !err.Timeout()) { // ugh, it's SO DAMN UGLY!
					fmt.Printf("error: %s\n", err.Error())
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
