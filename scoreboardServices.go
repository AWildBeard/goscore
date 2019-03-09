package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

type ConfigError string

func (err ConfigError) Error() string {
	return string(err)
}

type ScoreboardConfig struct {
	pingHosts bool
	TimeBetweenPingChecks time.Duration
	PingTimeout time.Duration
	TimeBetweenServiceChecks time.Duration
	ServiceTimeout time.Duration
}

// Struct to hold an update to a service held by ScoreboardState
type ServiceUpdate struct {
	Ip string // Unique identifier that can be used to group services to a single key.
	ServiceUpdate bool // If true, this ServiceUpdate contains data on an update to a service, otherwise, this is a ping update
	ServiceUp bool // bool to represent if the service at belows index is up.
	ServiceUpIndex int // This variable contains the index of the port that holds a service that is now up
}

// Struct to represent Multiple Services for on machine
type Service struct {
	Names []string
	Ports []string
	SendStrings[] string
	ResponseRegexs[] string
	Protocols[] string
	ServicesUp []bool
	Ip string
	PingUp bool
}

// Struct to represent the scoreboard
type ScoreboardState struct {
	Services []Service
	config ScoreboardConfig
	lock sync.RWMutex
}

// Function to serve the index.html for the scoreboard.
func (sbd *ScoreboardState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sbd.lock.RLock()
	returnString, _ := json.MarshalIndent(sbd.Services,"", "  ")
	sbd.lock.RUnlock()

	fmt.Fprintf(w, string(returnString))
}

// Helper function to return a new scoreboard
func NewScoreboard() ScoreboardState {
	return ScoreboardState{
		make([]Service, 0),
		ScoreboardConfig {
			false,
			time.Duration(0),
			time.Duration(0),
			time.Duration(0),
			time.Duration(0),
		},
		sync.RWMutex{},
	}
}

func parseConfigToScoreboard(config *Config, scoreboard *ScoreboardState) error {
	// Determine if the user has set the ping option in the config file.
	if config.Config["pingHosts"] != "yes" {
		scoreboard.config.pingHosts = false // Deactivates all the ping functionality of the program

	} else {
		scoreboard.config.pingHosts = true // Activates the ping functionality of the program

		// Determine the required pingInterval option from the config file
		if pingDuration, err := time.ParseDuration(config.Config["pingInterval"]) ; err == nil {
			scoreboard.config.TimeBetweenPingChecks = pingDuration

		} else { // The option was not found
			return ConfigError(fmt.Sprint("Failed to parse pingInterval from config file:", err))
		}

		// Determine the required pingTimeout option from the config file
		if ptimeout, err := time.ParseDuration(config.Config["pingTimeout"]) ; err == nil {
			scoreboard.config.PingTimeout = ptimeout

		} else { // The option was not found
			return ConfigError(fmt.Sprint("Failed to parse pingTimeout in config file:", err))
		}
	}

	// Determine the required serviceInterval option from the config file
	if serviceDuration, err := time.ParseDuration(config.Config["serviceInterval"]) ; err == nil {
		scoreboard.config.TimeBetweenServiceChecks = serviceDuration

	} else { // The option was not found
		return ConfigError(fmt.Sprint("Failed to parse serviceInterval from config file:", err))
	}

	if stimeout, err := time.ParseDuration(config.Config["serviceTimeout"]) ; err == nil {
		scoreboard.config.ServiceTimeout = stimeout

	} else {
		return ConfigError(fmt.Sprint("Failed to parse serviceTimeout from config file:", err))
	}

	// Check that at least one service is defined in the config file
	if len(config.Services) < 1 {
		return ConfigError("There must be at least one service defined in the config file!")
	}

	// Create the services for the ScoreboardState
	for _, mp := range config.Services {
		hostToAdd := mp["ip"] // Use IP as a unique identifier for machines
		shouldAdd := true

		for i := range scoreboard.Services {
			if hostToAdd == scoreboard.Services[i].Ip { // There are more services to test from this machine
				shouldAdd = false
				scoreboard.Services[i].Names = append(scoreboard.Services[i].Names, mp["service"])
				scoreboard.Services[i].Ports = append(scoreboard.Services[i].Ports, mp["port"])
				scoreboard.Services[i].ServicesUp = append(scoreboard.Services[i].ServicesUp, true)
				scoreboard.Services[i].SendStrings = append(scoreboard.Services[i].SendStrings, mp["send_string"])
				scoreboard.Services[i].ResponseRegexs = append(scoreboard.Services[i].ResponseRegexs,
					mp["response_regex"])
				scoreboard.Services[i].Protocols = append(scoreboard.Services[i].Protocols,
					mp["connection_protocol"])

				break
			} else { // This is the first occurence of this IP, so we should set this flag
				shouldAdd = true
			}
		}

		// Test the regex for services
		if _, err := regexp.Match(mp["response_regex"], []byte("")) ; err != nil {
			ilog.Println("Invalid regular expression:", err)
			ilog.Println("For more information see https://github.com/google/re2/wiki/Syntax")
			os.Exit(1)
		}

		// If the above loop got this far, then we are looking at a service we haven't seen yet, so add it here
		if shouldAdd {
			scoreboard.Services = append(scoreboard.Services, Service{
				[]string{mp["service"]}, // Array of len 1 that holds the service name we haven't added yet
				[]string{mp["port"]}, // Array of 1 that holds the services port
				[]string{mp["send_string"]}, // String to send to remote service
				[]string{mp["response_regex"]}, // Regex of the expected response
				[]string{mp["connection_protocol"]}, // The protocol to use to connect to the remote service
				[]bool{true}, // Default to up
				mp["ip"], // Key to ID the machine that may have many services
				true, // Default to up
			})
		}
	}

	return nil
}
