// Copyright 2019 Michael Mitchell
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// State represents the scoreboard's state. This type holds
// The hosts that are scored and the config by witch to score them.
// There is also a RW serviceLock to control reading and writing to
// the services contained within this type in addition to a
// scoreboardPageLock and an adminPageLock. These locks serve to
// control access to the individual resources. Updating between these
// resources is done by dedicated timed threads.
type State struct {
	// Hosts is an array of Host that this scoreboard scores
	Hosts []Host

	// Config is the Scoreboard config to dictate
	// how to check services as well as miscellaneous
	// options for the program and scoreboard
	Config Config

	// Name is the name of the competition. This can be used in the web interface.
	Name string

	// The webTemplate that get's updated periodically
	scoreboardPage []byte

	// serviceLock is the RW serviceLock that will allow updating the scoreboard
	// quickly without locking out web clients
	serviceLock sync.RWMutex

	// Template serviceLock is the serviceLock associated with the webTemplate.
	scoreboardPageLock sync.RWMutex

	adminPageLock sync.RWMutex
}

// Config represents the configuration for the scoreboard.
// Namely, the timeouts for checking host's services and ICMP, etc.
type Config struct {
	// PingHosts is a config option that represents whether the scoreboard should
	// ICMP test Hosts
	PingHosts bool

	// TimeBetweenPingChecks is the config option that signifies
	// the duration to wait before trying to ping all the hosts
	// that were defined in the config file.
	TimeBetweenPingChecks time.Duration

	// PingTimeout is the duration to wait on hosts to respond to this programs
	// Ping requests
	PingTimeout time.Duration

	// TimeBetweenServiceChecks is the duration to wait before trying to
	// check the services that were defined in the config file.
	TimeBetweenServiceChecks time.Duration

	// ServiceTimeout is the duration to wait for all services (not ICMP) to
	// respond to this program.
	ServiceTimeout time.Duration

	// DefaultServiceState is the default service state for all
	// services and hosts. If the user is wanting to test services
	// that will all be up at the beginning of the CTF, setting this
	// to true will give accurate uptimes and downtimes to that
	// situation. If the user expects the services to be down at the
	// beginning, setting this to false will give accurate uptimes
	// and downtimes for that usecase.
	DefaultServiceState bool

	// ScoreboardDoc represents a custom HTML template for sending to a HTTP client.
	ScoreboardDoc string

	// ListenAddress represents the address to bind the HTTP server to
	ListenAddress string

	// CompetitionDuration represents the duration to run the competition for.
	CompetitionDuration time.Duration

	// AdminName is the username for the management account
	AdminName string

	// AdminPassword is the password for the management account
	AdminPassword string

	// StartTime represents the time that the Start() function is called which as a result
	// represents the time the competition started.
	StartTime time.Time

	// StopTime represents the precomputed timepoint of when the competition should end.
	StopTime time.Time

	// CompetitionEnded represents whether the competition has ended
	CompetitionEnded bool
}

// UptimeTracking is implemented on types that have a state that needs to be changed, and need to track
// how long that state has been in place. UptimeTracking is implemented on Service and Host. Types that
// implement this type should store the timepoints that should be incremented in SetUp so that Uptime and
// Downtime can be calculated in relation to timepoints provided to GetUptime and GetDowntime
type UptimeTracking interface {
	// SetUp will change the state of a tracker, and perform any necessary timing calculations
	// and changes based on the new state
	SetUp(state bool)

	// IsUp returns whether the state of the tracker is up or down.
	IsUp() bool

	// GetUptime will return the uptime of a tracker in relation to the referenceTime provided to it.
	GetUptime(referenceTime time.Time) time.Duration

	// GetDowntime will return the downtime of a tracker in relation to the referenceTime provided to it.
	GetDowntime(referenceTime time.Time) time.Duration
}

// GetUptime for State returns the time that a host or service have been up and accounts for special timing
// calculations that need to be made at the end of the competition.
func (sbd *State) GetUptime(tracker UptimeTracking) time.Duration {
	var duration time.Duration

	if sbd.Config.CompetitionEnded {
		duration = tracker.GetUptime(sbd.Config.StopTime)
	} else {
		duration = tracker.GetUptime(time.Now())
	}

	return duration
}

// GetDowntime for State returns the time that a host or service have been down and accounts for special timing
// calculations that need to be made at the end of the competition.
func (sbd *State) GetDowntime(tracker UptimeTracking) time.Duration {
	var duration time.Duration

	if sbd.Config.CompetitionEnded {
		duration = tracker.GetDowntime(sbd.Config.StopTime)
	} else {
		duration = tracker.GetDowntime(time.Now())
	}

	return duration
}

// TimeLeft returns the amount of time left for the entire competition
func (sbd *State) TimeLeft() time.Duration {
	timeRemaining := sbd.Config.CompetitionDuration - time.Now().Sub(sbd.Config.StartTime)

	if timeRemaining < 0 {
		return time.Duration(0)
	}

	return timeRemaining
}

// NewScoreboard is a helper function to return a new scoreboard
func NewScoreboard() State {
	return State{
		Hosts: make([]Host, 0),
	}
}

// Start is the definitive way to start the competition scoreboard. This starts a timer based off of the
// configuration file that determines when to stop judging services. This function also starts the threads
// used to judge services and the webserver. When competition scoring has finished, the webserver is left running
// with the scoring data until the program is killed.
func (sbd *State) Start() {

	func() {
		connection := strings.Split(sbd.Config.ListenAddress, ":")
		index := 0
		if len(connection) > 1 {
			index = 1
		}

		port, _ := strconv.Atoi(connection[index])

		testPrivileges(port, sbd.Config.PingHosts)
	}()

	// HTTP Server
	mux := http.NewServeMux()
	mux.HandleFunc("/", sbd.scoreboardResponder)
	mux.HandleFunc("/admin", sbd.adminPanel)

	server := http.Server{
		Addr:    sbd.Config.ListenAddress,
		Handler: mux,
	}

	// Make a buffered channel to write service updates over. These updates will get read by a thread
	// that will write serviceLock ScoreboardState
	updateChannel := make(chan ServiceUpdate, 10)
	newUpdateSignal := make(chan bool, 1)

	// Make channels to write shutdown signals over
	shutdownPingSignal := make(chan bool, 1)
	shutdownServiceSignal := make(chan bool, 1)
	shutdownStateUpdaterSignal := make(chan bool, 1)
	shutdownTemplateUpdaterSignal := make(chan bool, 1)

	time.AfterFunc(sbd.Config.CompetitionDuration, func() {
		ilog.Println("The competition duration has been reached. Shutting down scoring services.")
		shutdownPingSignal <- true
		shutdownServiceSignal <- true
		shutdownStateUpdaterSignal <- true
		shutdownTemplateUpdaterSignal <- true
		sbd.serviceLock.Lock()
		sbd.Config.CompetitionEnded = true
		sbd.serviceLock.Unlock()
	})

	sbd.startScoring()

	go sbd.PingChecker(updateChannel, shutdownPingSignal)

	go sbd.ServiceChecker(updateChannel, shutdownServiceSignal)

	go sbd.StateUpdater(updateChannel, newUpdateSignal, shutdownStateUpdaterSignal)

	go sbd.WebContentUpdater(newUpdateSignal, shutdownTemplateUpdaterSignal)

	ilog.Println("Started Scoreboard")

	// Start the webserver and serve content
	ilog.Fatal(server.ListenAndServe())
}

// startScoring initializes all the times for hosts and services, and initializes the start time and end time
// for the scoreboard.
func (sbd *State) startScoring() {
	newTime := time.Now()

	for hostIndex := range sbd.Hosts {
		host := &sbd.Hosts[hostIndex]

		host.previousUpdateTime = newTime
		host.isUp = sbd.Config.DefaultServiceState

		for serviceIndex := range host.Services {
			service := &host.Services[serviceIndex]

			service.previousUpdateTime = newTime
			service.isUp = sbd.Config.DefaultServiceState
		}
	}

	sbd.Config.StartTime = newTime
	sbd.Config.StopTime = sbd.Config.StartTime.Add(sbd.Config.CompetitionDuration)
	sbd.Config.CompetitionEnded = false
}

// StateUpdater is a thread to read service updates and write the updates to ScoreboardState. We do this so
// we don't have to give every status checking thread the ability to
// RW serviceLock the ScoreboardState. This lets us test services without locking.
// This function read locks for determining if an update should be applied to the
// Scoreboard State. If an update needs to be applied, the function drops its read serviceLock
// and establishes a write serviceLock to update the data. The write serviceLock is maintained for as long as there
// are service updates that need to be analyzed. If no write serviceLock is established, the function maintains
// it's read serviceLock as long as there are service updates that need to be analyzed.
//
// The end goal of this complex locking is to minimize the time spent holding a
// write serviceLock. however, once this function has establish a write serviceLock,
// don't drop it because it might need to be re-established nano-seconds later.
// This function read locks for safety reasons.
func (sbd *State) StateUpdater(updateChannel chan ServiceUpdate, updateSignal, shutdownUpdaterSignal chan bool) {

	// These two flags are mutually exclusive. One being set does not rely on the other
	// which is why we have two of them, instead of expressing their logic with a single flag.
	// This function will drop it's read serviceLock when it's in a sleeping state,
	// and only establishes a read serviceLock when needing to find data that might be
	// changed, and only then establishing a write serviceLock **if** that data needs to be
	// changed. A write serviceLock or a read serviceLock is kept until there is no more
	// data to be parsed through.
	var (
		isWriteLocked = false // Flag to hold whether we already have a serviceLock or not.
		isReadLocked  = false // Flag to hold whether we have a read serviceLock.
	)

	ilog.Println("Started the Service State Updater")

	for {
		// A service update that we are waiting for
		var update ServiceUpdate

		// Test for there being another service update on the line
		select {
		case <-shutdownUpdaterSignal:
			ilog.Println("Shutting down the Service State Updater")
			return
		case update = <-updateChannel: // There is another update on the line

			// Read-Lock to be safe.
			if !isWriteLocked && !isReadLocked {
				sbd.serviceLock.RLock()
				isReadLocked = true
			}

			// Interate down to the Service or Host that needs to be updated
			for indexOfHosts := range sbd.Hosts {
				// Get a reference to the host
				host := &sbd.Hosts[indexOfHosts]

				if update.IP == host.IP {
					// Found the correct host

					if update.ServiceUpdate { // Is the update a service update, or an ICMP update?

						// It's a service update so iterate down to the service that needs to be updated.
						for indexOfServices := range host.Services {

							// Get a reference to the service
							service := &host.Services[indexOfServices]

							if service.Name == update.ServiceName {
								// Found the correct service

								// Decide if the update contradicts the current Scoreboard State.
								// If it does, we need to establish a Write serviceLock before changing
								// the service state.
								if service.isUp != update.IsUp {
									if !isWriteLocked { // If we already have a RW serviceLock, don't que another
										sbd.serviceLock.RUnlock() // Unlock our Read serviceLock before Write Locking
										isReadLocked = false
										sbd.serviceLock.Lock() // WRITE LOCK
										isWriteLocked = true
									}

									// Update that services state
									service.SetUp(update.IsUp)

									// Debug that we received a service update
									dlog.Printf("Received a service update for %v on %v.\n"+
										"\tStatus: %v -> Needed to update scoreboard\n"+
										"\tUptime: %v, Downtime: %v", service.Name,
										host.Name, update.IsUp,
										fmtDuration(sbd.GetUptime(service)), fmtDuration(sbd.GetDowntime(service)))

								} else {
									// Debug that we received a service update
									dlog.Printf("Received a service update for %v on %v.\n"+
										"\tStatus: %v -> Didn't need to update scoreboard\n"+
										"\tUptime: %v, Downtime: %v", service.Name,
										host.Name, update.IsUp,
										fmtDuration(sbd.GetUptime(service)), fmtDuration(sbd.GetDowntime(service)))

								}

								break // We found the correct service so stop searching
							}
						}
					} else {

						// We are dealing with an ICMP update. We need to determine if the
						// Scoreboard State needs to be updated.
						if host.isUp != update.IsUp { // We need to establish a write serviceLock
							if !isWriteLocked { // If we already have a RW serviceLock, don't que another
								sbd.serviceLock.RUnlock()
								isReadLocked = false
								sbd.serviceLock.Lock() // WRITE LOCK
								isWriteLocked = true
							}

							host.SetUp(update.IsUp)

							// Debug print the service update
							dlog.Printf("Received a ping update for %v on %v.\n"+
								"\tStatus: %v -> Needed to update scoreboard.\n"+
								"\tUptime: %v, Downtime: %v", host.IP,
								host.Name, host.isUp,
								fmtDuration(sbd.GetUptime(host)), fmtDuration(sbd.GetDowntime(host)))

						} else {
							// Debug print the service update
							dlog.Printf("Received a ping update for %v on %v.\n"+
								"\tStatus: %v -> Didn't need to update scoreboard.\n"+
								"\tUptime: %v, Downtime: %v", host.IP,
								host.Name, host.isUp,
								fmtDuration(sbd.GetUptime(host)), fmtDuration(sbd.GetDowntime(host)))
						}
					}

					break // We found the correct host, so stop searching
				}
			}
		default: // There is not another update on the line, so we'll wait for one
			// If we have a write serviceLock because we changed the ScoreboardState
			// because of an ServiceUpdate, release the Write serviceLock so clients
			// can view content. Otherwise, we had a read serviceLock that needs to
			// be released because we don't need it any longer.
			if isWriteLocked {
				updateSignal <- true // Signal the WebContentUpdater to re-evaluate the web content
				sbd.serviceLock.Unlock()
				isWriteLocked = false
			} else if isReadLocked { // This isn't a else case because this default case might be ran quickly in succession
				sbd.serviceLock.RUnlock()
				isReadLocked = false
			}

			// Wait 1 second, then check for ServiceUpdates again!
			time.Sleep(1 * time.Second)
		}
	}
}

// ServiceChecker is a thread for querying services. Results are shipped to the
// ScoreboardStateUpdater as ServiceUpdates
func (sbd *State) ServiceChecker(updateChannel chan ServiceUpdate, shutdownServiceSignal chan bool) {

	ilog.Println("Started the Service Check Provider")

	for {
		select {
		case <-shutdownServiceSignal:
			ilog.Println("Shutting down the Service Check Provider")
			return
		default:
			sbd.serviceLock.RLock()
			// Go ahead and test these bad guys before going to sleep.
			for hostIndex := range sbd.Hosts { // Check each host
				host := sbd.Hosts[hostIndex]
				for serviceIndex := range host.Services { // Check each service
					service := host.Services[serviceIndex]

					// Asyncronously check services so we can check a lot of them
					// and don't have to wait on service timeout durations
					// which might be lengthy.
					go service.CheckService(updateChannel,
						host.IP, sbd.Config.ServiceTimeout)
				}
			}
			sbd.serviceLock.RUnlock()

			// Sleep before testing these services again.
			time.Sleep(sbd.Config.TimeBetweenServiceChecks)
		}
	}
}

// PingChecker is a thread for pinging hosts. Results are shipped to the
// ScoreboardStateUpdater as ServiceUpdates.
func (sbd *State) PingChecker(updateChannel chan ServiceUpdate, shutdownPingSignal chan bool) {
	if sbd.Config.PingHosts { // The ping option was set
		ilog.Println("Started the Ping Check Provider")

		for {
			select {
			case <-shutdownPingSignal:
				ilog.Println("Shutting down the Ping Check Provider")
				return
			default:
				sbd.serviceLock.RLock()
				for i := range sbd.Hosts {
					host := sbd.Hosts[i]
					// Asyncronously ping hosts so we don't wait full timeouts and can ping faster.
					go host.PingHost(updateChannel, sbd.Config.PingTimeout)
				}

				sbd.serviceLock.RUnlock()

				// Sleep before testing these hosts again
				time.Sleep(sbd.Config.TimeBetweenPingChecks)
			}
		}
	}
}
