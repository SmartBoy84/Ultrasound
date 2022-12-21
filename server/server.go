package main

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	ping_code       = 1
	register_code   = 2
	deregister_code = 3
)

type Subscriber struct {
	Remote *net.UDPConn
}

type Subscribers struct {
	mu   sync.Mutex
	list map[*Subscriber]int // list of subscribers
	// pointers as map is scary but go is super-safe when it comes to exposing raw pointers (from what I can tell)
	// also allows changing remote and stuff without affecting map hash

	wait_n, wait_ms int  // timeout values
	tolerance       int  // max amount of connection failures before client is removed all together
	pinging         bool // if currently pinging
}

func (subscriber *Subscriber) ReadStatus(status byte, ms int) error {
	read := []byte{0}

	var err error
	var rlen int

	subscriber.Remote.SetDeadline(time.Now().Add(time.Duration(ms) * time.Millisecond))
	rlen, _, err = subscriber.Remote.ReadFromUDP(read[:])

	if err == nil {

		if rlen == 0 {
			err = errors.New("incoming message length cannot be 0")

		} else if read[0] != byte(status) {
			err = errors.New("malformed message")

		} else {
			return nil

		}
	}

	return err
}

func (subscriber *Subscriber) wait(status byte, n int, ms int, failures chan<- *Subscriber, sendWait *sync.WaitGroup) (err error) {
	defer func() { sendWait.Done() }()

	for i := 0; i < n; i++ {

		if _, err = subscriber.Remote.Write([]byte{status}); err == nil {
			if err = subscriber.ReadStatus(status, ms); err == nil {

				return nil
			}
		}
	}

	if failures != nil {
		failures <- subscriber
	}

	return err
}

func (subscribers *Subscribers) SendCode(message byte) (err error) {

	failures := make(chan *Subscriber, len(subscribers.list))
	var sendWait sync.WaitGroup

	for subscriber := range subscribers.list {
		sendWait.Add(1)
		go subscriber.wait(message, subscribers.wait_n, subscribers.wait_ms, failures, &sendWait)
	}

	sendWait.Wait()

	subscribers.mu.Lock()
	defer subscribers.mu.Unlock()

	for len(failures) > 0 {
		failure := <-failures

		if subscribers.list[failure]++; subscribers.list[failure] > subscribers.tolerance {

			fmt.Println("Enough is enough! Evicting unstable client from list") // "unstable" because I'm too lazy to figure out a way to reset tolerance values
			delete(subscribers.list, failure)
		}

	}

	if len(subscribers.list) == 0 {
		return errors.New("no subscribers remaining")
	}

	return nil
}

func (subscribers *Subscribers) Ping() (err error) {

	subscribers.pinging = true

	for {
		time.Sleep(time.Duration(1000) * time.Millisecond)

		if err = subscribers.SendCode(ping_code); err != nil {
			fmt.Println(err.Error())

			subscribers.pinging = false
			return err
		}
	}
}

func (subscribers *Subscribers) Register(target *net.UDPAddr, receiver *net.UDPConn) (err error) {
	// even if this is called more than once for the same client, Ping() is implemented such that the ensuing binding errors will automagically filter out the duds

	fmt.Printf("Registering client: %s\n", target.String())

	for i := 0; i < subscribers.wait_n; i++ {

		var remote *net.UDPConn
		if remote, err = net.DialUDP("udp", nil, target); err == nil {

			if _, err = remote.Write([]byte{register_code}); err == nil {

				subscriber := Subscriber{Remote: remote}
				message := []byte{0}

				remote.SetDeadline(time.Now().Add(time.Duration(subscribers.wait_ms) * time.Millisecond))
				if _, _, err = remote.ReadFromUDP(message); err == nil {

					if message[0] == register_code {
						fmt.Println("Successfully registered!")

						subscribers.mu.Lock()
						subscribers.list[&subscriber] = 0
						subscribers.mu.Unlock()

						break
					}
				}
			}
		}

		fmt.Print(err.Error())
		time.Sleep(time.Millisecond * time.Duration(subscribers.wait_ms))
	}

	if err == nil && !subscribers.pinging {
		go subscribers.Ping()
	}

	if err != nil {
		fmt.Print(err)
	}

	return err
}

func (subscribers *Subscribers) Registrar(local *net.UDPAddr) error {

	conn, err := net.ListenUDP("udp", local)
	if err != nil {
		return err
	}
	defer conn.Close() // this will never happen unless ListenUDP() fails

	fmt.Printf("Server listening - %s\n", conn.LocalAddr().String())

	message := []byte{0}
	subscribers.list = make(map[*Subscriber]int)

	for {
		rlen, remote, err := conn.ReadFromUDP(message[:])

		if err == nil && rlen > 0 {

			if rlen > 1 {
				fmt.Println("[WARNING] message of greater length than expected")
			}

			if message[0] == register_code {
				go subscribers.Register(remote, conn)

			} else if message[0] == ping_code {
				if _, err = conn.WriteToUDP([]byte{ping_code}, remote); err != nil {
					fmt.Printf("Failed to respond to probe: %s", err.Error())
				}
			}
		}

		if err != nil {
			// considering panicing?
			fmt.Print(err)
		}
	}
}
