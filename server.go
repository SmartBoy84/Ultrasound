package main

import (
	"errors"
	"fmt"
	"net"
	"sync"
)

type Subscriber struct {
	middleMan *MiddleMan
	settings  *Settings
}

type Subscribers struct {
	mu sync.Mutex

	Registrar *MiddleMan
	settings  *Settings // these values are "copied" to each client

	list map[*Subscriber]interface{} // list of subscribers
	// pointers as map is scary but go is super-safe when it comes to exposing raw pointers (from what I can tell)
	// also allows changing remote and stuff without affecting map hash
}

func NewSubscriberList(settings *Settings) *Subscribers {

	subscribers := Subscribers{settings: settings, Registrar: &MiddleMan{settings: settings}}
	subscribers.list = make(map[*Subscriber]interface{})

	return &subscribers
}

func (subscribers *Subscribers) SendMessage(message byte) (err error) {

	if len(subscribers.list) == 0 {
		return errors.New("no subscribers remaining")
	}

	var sendWait sync.WaitGroup

	for subscriber := range subscribers.list {
		sendWait.Add(1)

		go func(subscriber *Subscriber) {
			defer func() { sendWait.Done() }()
			subscriber.middleMan.SendMessage(message)
		}(subscriber)
	}

	sendWait.Wait()
	return nil
}

func (subscribers *Subscribers) Register(target *net.UDPAddr, receiver *net.UDPConn) (err error) {
	// even if this is called more than once for the same client, Ping() is implemented such that the ensuing binding errors will automagically filter out the duds
	fmt.Println("Registration requested")

	var remote *net.UDPConn

	if remote, err = net.ListenUDP("udp", resolve(":0")); err != nil { // step 1
		return err
	}
	defer remote.Close()

	subscriber := &Subscriber{middleMan: &MiddleMan{Remote: remote, Target: target, settings: subscribers.settings}, settings: subscribers.settings}

	if err = subscriber.middleMan.PingPong(register_code); err != nil { // step 2
		return err
	}

	if err = subscriber.middleMan.PingPong(ping_code); err != nil { // step 3
		return err
	}

	subscribers.mu.Lock()
	subscribers.list[subscriber] = nil
	subscribers.mu.Unlock()

	var sendWait sync.WaitGroup

	sendWait.Add(1)

	subscriber.middleMan.lostConnection = func(err error) {
		defer sendWait.Done()

		fmt.Print(err)
		fmt.Println("Enough is enough! EXTERMINATE")
		delete(subscribers.list, subscriber)
	}

	// start subscriber routines
	go subscriber.middleMan.Heartbeat()
	go subscriber.middleMan.Digest() // start this subcriber's responder routine and then forget about it

	sendWait.Wait()
	return nil // remote closes here as well
}

func (subscribers *Subscribers) StartRegistrar(local *net.UDPAddr) error {

	conn, err := net.ListenUDP("udp", local)
	if err != nil {
		return err
	}
	defer conn.Close() // this will never happen unless ListenUDP() or middleMan fails

	subscribers.Registrar.Remote = conn

	subscribers.Registrar.settings.messageReceivedFn = func(target *net.UDPAddr, message []byte) {
		if message[0] == register_code {
			go subscribers.Register(target, conn)
		}
	}

	return subscribers.Registrar.Digest() // this blocks
}