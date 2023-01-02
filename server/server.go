package main

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	ping_code        = 1
	register_code    = 2
	deregister_code  = 3
)

func TimeMeOut(conn *net.UDPConn, timer time.Duration) {
	conn.SetDeadline(time.Now().Add(timer)) // UDP is connectionless, remember? This is all up to us
}

type Subscriber struct {
	Remote      *net.UDPConn
	failCounter int

	wait_time time.Duration
	tolerance int // max amount of connection failures before client is removed all together

	mu sync.Mutex
}

type Subscribers struct {
	mu   sync.Mutex
	list map[*Subscriber]interface{} // list of subscribers
	// pointers as map is scary but go is super-safe when it comes to exposing raw pointers (from what I can tell)
	// also allows changing remote and stuff without affecting map hash

	wait_time time.Duration
	tolerance int // max amount of connection failures before client is removed all together

	pinging bool // if currently pinging
}

func (subscriber *Subscriber) ReadStatus(status byte) error {

	read := []byte{0}

	var err error
	var rlen int

	TimeMeOut(subscriber.Remote, subscriber.wait_time)
	if rlen, _, err = subscriber.Remote.ReadFromUDP(read[:]); err != nil {
		return err
	}

	if rlen == 0 {
		return errors.New("incoming message length cannot be 0")
	}

	if read[0] != byte(status) {
		return errors.New("malformed message")
	}

	return nil
}

func (subscriber *Subscriber) PingPong(status byte) (err error) {

	TimeMeOut(subscriber.Remote, subscriber.wait_time)
	if _, err = subscriber.Remote.Write([]byte{status}); err == nil {

		if err = subscriber.ReadStatus(status); err == nil {
			return nil
		}
	}

	if err, ok := err.(net.Error); err != nil && (!ok || !err.Timeout()) { // ugh, it's SO DAMN UGLY!
		time.Sleep(subscriber.wait_time)
	}

	return err
}

func (subscribers *Subscribers) SendCode(message byte) (err error) {

	if len(subscribers.list) == 0 {
		return errors.New("no subscribers remaining")
	}

	var sendWait sync.WaitGroup

	for subscriber := range subscribers.list {
		sendWait.Add(1)

		go func(subscriber *Subscriber) {
			defer func() { sendWait.Done() }()

			subscriber.PingPong(message)
		}(subscriber)
	}

	sendWait.Wait()
	return nil
}

func (subscribers *Subscribers) Ping() {

	subscribers.pinging = true
	var sendWait sync.WaitGroup

	for {
		time.Sleep(subscribers.wait_time)

		// weed out any unresponsive subscribers
		subscribers.mu.Lock()

		// start main routine
		for subscriber := range subscribers.list {

			sendWait.Add(1)

			go func(subscriber *Subscriber) {
				subscriber.mu.Lock()

				defer func() {
					sendWait.Done()
					subscriber.mu.Unlock()
				}()

				if err := subscriber.PingPong(ping_code); err == nil {

					if subscriber.failCounter > 0 {
						fmt.Printf("Phew, %s recovered!\n", subscriber.Remote.LocalAddr().String())
						subscriber.failCounter = 0
					}

					return
				}

				subscriber.failCounter++

				if subscriber.failCounter > subscribers.tolerance {

					fmt.Println("Enough is enough! EXTERMINATE")
					delete(subscribers.list, subscriber)

					return
				}

				fmt.Printf("%s issued warning %d/%d!\n", subscriber.Remote.RemoteAddr().String(), subscriber.failCounter, subscriber.tolerance)
			}(subscriber)
		}

		subscribers.mu.Unlock()
		sendWait.Wait() // wait for all clients to respond (or not!)

		if len(subscribers.list) == 0 {
			fmt.Println("No subscribers remaining")

			subscribers.mu.Lock()
			subscribers.pinging = false
			subscribers.mu.Unlock()

			return
		}
	}
}

func (subscribers *Subscribers) Register(target *net.UDPAddr, receiver *net.UDPConn) (err error) {
	// even if this is called more than once for the same client, Ping() is implemented such that the ensuing binding errors will automagically filter out the duds

	var remote *net.UDPConn

	if remote, err = net.DialUDP("udp", nil, target); err != nil { // step 1
		return err
	}

	subscriber := Subscriber{Remote: remote, tolerance: subscribers.tolerance, wait_time: subscribers.wait_time}

	if err = subscriber.PingPong(register_code); err != nil { // step 2
		return err
	}

	if err = subscriber.PingPong(ping_code); err != nil { // step 3
		return err
	}

	subscribers.mu.Lock()

	subscribers.list[&subscriber] = nil

	subscribers.mu.Unlock()

	if !subscribers.pinging {
		go subscribers.Ping()
	}

	return nil
}

func (subscribers *Subscribers) Registrar(local *net.UDPAddr) error {

	conn, err := net.ListenUDP("udp", local)
	if err != nil {
		return err
	}
	defer conn.Close() // this will never happen unless ListenUDP() fails

	fmt.Printf("Server listening - %s\n", conn.LocalAddr().String())

	message := []byte{0}
	subscribers.list = make(map[*Subscriber]interface{})

	// readUDP variables
	var rlen int
	var incoming *net.UDPAddr

	for {

		if err != nil {
			// considering panicing?
			fmt.Print(err)
			err = nil
		}

		if rlen, incoming, err = conn.ReadFromUDP(message[:]); err != nil {
			continue
		}

		if rlen == 0 {
			fmt.Println("Message cannot be empty, right?!")
			continue
		}

		if rlen > 1 {
			fmt.Println("[WARNING] message of greater length than expected")
		}

		switch message[0] {

		case register_code:

			fmt.Println("Registration requested")
			go func() {

				if err := subscribers.Register(incoming, conn); err != nil {
					fmt.Print(err)
				} else {
					fmt.Println("Success!")
				}

			}()

		case ping_code:

			if _, err = conn.WriteToUDP([]byte{ping_code}, incoming); err != nil {
				fmt.Printf("Failed to respond to probe: %s", err.Error())
			}
		}
	}
}
