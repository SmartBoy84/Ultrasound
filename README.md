# Ultrasound
Personal note: Pi is dead, wait a couple of days for polyfuse to fix otherwise try replacing it

Simple started golang project to interface with an ultrasound sensor and send its status to connected clients via UDP

Many interfaces written in Golang for Raspberrypi exist but I found gpiod to be the only one that seems to be actively maintained and works reliably
j8 is a preset of offsets which map the physical location of a pin on the PI's J8 header (long strip on pins one side), counting from top right -> bottom left, to their kernel index

Implementation details: 
1. Server is started and the subscribers list is created
2. Sensor is initialised: trigger set as output and echo set as input (pulled down)
3. Echo average time-to-return calculated and stored
2. Registrar method starts, waiting for UDP registration requests on the pre-defined port (run on 0.0.0.0!)

3. Client connects, sending burst of registration packets to server
4. Registrar starts a registration go routine
5. Register() starts a new UDP listener connection on a random port and echoes back the register packet
6. Client receives confirmation and sets it's remote address to be the same as the newly formed connection
7. Client starts a blocking pong goroutine to respond to ping and other requests
8. Register() finishes, adds client to subscriber list
9. Ping() goroutine started on server side to send ping requests periodically
10. Pulse() goroutine started on server side

10. Upon receiving a packet from server client echoes it back
11. If client fails to echo ping request, then after 3 more tries server deregisters client, decrementing subscriber count
12. Once subscriber count reacees 0, ping() and pulse() is stopped until a register() go routine restarts it

11. On client side, another goroutine runs to ensure that ping requests are being received
12. If client fails to receive a ping request, it will "deregister" itself, the goroutine will end, fall through until the program crashs

13. Pulse() runs continuously until atleast one subscriber is connected and waits until the time-to-return is lower than the average that was stored at the start
14. If so, it sends the active packet to the client which runs the call back function - in this case it will run the command supplied to it when motion is detected
