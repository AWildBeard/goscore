package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// Struct to represent Multiple Services for on machine
type Service struct {
	Names []string
	Ports []string
	ServicesUp []bool
	Ip string
	PingUp bool
}

// Struct to represent the scoreboard
type ScoreboardState struct {
	Services []Service
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
		sync.RWMutex{},
	}
}

// Struct to hold an update to a service held by ScoreboardState
type ServiceUpdate struct {
	Ip string // Unique identifier that can be used to group services to a single key.
	ServiceUpdate bool // If true, this ServiceUpdate contains data on an update to a service, otherwise, this is a ping update
	ServiceUp bool // bool to represent if the service at belows index is up.
	ServiceUpIndex int // This variable contains the index of the port that holds a service that is now up
}

