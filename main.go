package main

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"

	"github.com/codegangsta/negroni"
	"github.com/gorilla/mux"
	"github.com/hybridgroup/gobot"
	"github.com/hybridgroup/gobot/platforms/firmata"
	"github.com/hybridgroup/gobot/platforms/gpio"
)

// TODO: Consider converting to envflag
var (
	// "/dev/cu.usbserial-A5027JS7"
	port        = flag.String("port", "", "device port to connect to")
	steeringPin = flag.Int("steering-pin", 10, "pin steering control is connected to")
	speedPin    = flag.Int("speed-pin", 11, "pin speed control is connected to")
)

// TODO: Graceful shutdown
func main() {
	flag.Parse()

	if *port == "" {
		panic("Must provide --port argument")
	}

	// Initialize connection to firmata on the specified port
	fmt.Printf("Connecting to %s\n", *port)
	firmataAdaptor := firmata.NewFirmataAdaptor("firmata", *port)
	fmt.Printf("Using pin %d as steering\n", *steeringPin)
	steeringServo := gpio.NewServoDriver(firmataAdaptor, "servo", strconv.Itoa(*steeringPin))
	fmt.Printf("Using pin %d as speed\n", *speedPin)
	speedServo := gpio.NewServoDriver(firmataAdaptor, "servo", strconv.Itoa(*speedPin))

	// Create channels to talk to hardware so we only send one message at a time
	steeringChannel := createServoChannel(steeringServo)
	speedChannel := createServoChannel(speedServo)

	// Work is to start an HTTP server
	work := func() {
		router := mux.NewRouter()
		router.HandleFunc("/steering/{value:[0-9]+}", createUint8Handler(steeringChannel)).Methods("POST")
		router.HandleFunc("/speed/{value:[0-9]+}", createUint8Handler(speedChannel)).Methods("POST")

		n := negroni.Classic()
		n.UseHandler(router)
		n.Run(":3000")
	}

	robot := gobot.NewRobot("servoBot",
		[]gobot.Connection{firmataAdaptor},
		//[]gobot.Device{steeringServo, speedServo},
		[]gobot.Device{speedServo, steeringServo},
		work,
	)

	gbot := gobot.NewGobot()
	gbot.AddRobot(robot)
	gbot.Start()
}

func createServoChannel(servo *gpio.ServoDriver) chan<- uint8 {
	c := make(chan uint8)
	// TODO: Handle shutdown
	go func() {
		for {
			select {
			case value := <-c:
				fmt.Printf("Sending servo value: %d\n", value)
				servo.Move(value)
			}
		}
	}()
	return c
}

func createUint8Handler(c chan<- uint8) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		sValue := vars["value"]
		fmt.Printf("Received value: %s\n", sValue)
		iValue, _ := strconv.Atoi(sValue)
		value := uint8(iValue)
		c <- value
	}
}
