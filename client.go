package main

import (
	"errors"
	"fmt"
	"net"
	"time"
)

type Client struct {
	listener *MiddleMan
	settings *Settings
}

func NewClient(settings *Settings) *Client {
	return &Client{settings: settings, listener: &MiddleMan{settings: settings}}
}

func (client *Client) pong() error {

	fmt.Printf("Done! Connection to server established\n\n")

	client.listener.lostConnection = func(err error) {
		fmt.Print(err)
	}

	go client.listener.Heartbeat()
	client.listener.Digest() // this blocks until connection is lost

	return errors.New("connection lost")
}

func (client *Client) Register(remoteAddr *net.UDPAddr) (err error) {

	// create new local listener address
	client.listener.Remote, err = net.ListenUDP("udp", resolve(":0")) // :0 gets as [::]:[random free port (in legal range obv)]
	client.listener.Target = remoteAddr

	if err != nil {
		return err
	}
	defer client.listener.Remote.Close() // after a successful registration, this occurs after Pong() fails which happens when MiddleMan.Digest() fails

	message := []byte{0, 0}

	for i := 0; i < client.settings.tolerance; i++ {
		if err = client.listener.PingPong(ping_code); err != nil {
			continue
		}
	}

	if err != nil {
		return err
	}

	// establish connection
	fmt.Print("Waiting for server to respond to our request")

	for i := 0; i < client.settings.tolerance; i++ {

		if err != nil {
			time.Sleep(time.Millisecond) // for loop only goes past 1 when err != nil

			fmt.Println(err.Error())
			err = nil
		}

		if _, err = client.listener.Remote.WriteToUDP([]byte{0, register_code}, remoteAddr); err != nil {
			continue
		}

		fmt.Println("\nRequest sent!")

		if _, client.listener.Target, err = client.listener.Remote.ReadFromUDP(message[:]); err != nil { // we set target here as the server is now listening on a separate port
			continue
		}

		if message[1] != register_code {

			err = errors.New("server sent an unexpected response")
			continue
		}

		fmt.Printf("We've shaken hands with %s, let's go!\n", remoteAddr.String())

		// server replies via a PingPong() request so we also need to confirm our existence by doing this manually
		if _, err = client.listener.Remote.WriteToUDP([]byte{0, register_code}, client.listener.Target); err != nil {
			continue
		}

		client.pong() // this blocks, start it up ASAP to send back final registration ping to server
		return nil
	}

	if err == nil {
		err = errors.New("timed out")
	}

	fmt.Println()
	return err
}
