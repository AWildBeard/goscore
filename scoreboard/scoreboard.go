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

package scoreboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Struct to represent the scoreboard's state. This type holds
// The hosts that are scored and the config by witch to score them.
// There is also a RW lock to control reading and writing to this
// type. This is implemented mainly because this type implements
// ServeHTTP so it can serve it's data over HTTP
type State struct {
	// The hosts this scoreboard scores
	Hosts []Host

	// Scoreboard specific config to dictate
	// how to check services
	Config Config

	// The RW lock that will allow updating the scoreboard
	// quickly without locking out web clients
	lock sync.RWMutex
}

// This struct represents the configuration for the scoreboard.
// Namely, the timeouts for checking host's services and ICMP, etc.
type Config struct {
	// Config option that represents whether the scoreboard should
	// ICMP test Hosts
	PingHosts bool

	// Config option that signifies the duration to wait before
	// trying to ping all the hosts defined in the Scoreboard State.
	TimeBetweenPingChecks time.Duration

	// The duration to wait on hosts to respond to this programs
	// Ping requests
	PingTimeout time.Duration

	// The duration to wait before trying to check the services
	// as they are defined in the Scoreboard State.
	TimeBetweenServiceChecks time.Duration

	// The duration to wait for all services (not ICMP) to
	// respond to this program.
	ServiceTimeout time.Duration
}

// Helper function to return a new scoreboard
func NewScoreboard() State {
	return State{
		make([]Host, 0),
		Config{
			false,
			time.Duration(0),
			time.Duration(0),
			time.Duration(0),
			time.Duration(0),
		},
		sync.RWMutex{},
	}
}

// Function to serve the `index.html` for the scoreboard.
// Implements ServeHTTP for ScoreboardState
func (sbd *State) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Establish a read-only lock to the scoreboard to retrieve data,
	// then drop the lock after we have retrieved that data we need.
	sbd.lock.RLock()
	returnString, _ := json.MarshalIndent(sbd.Hosts, "", "  ")
	sbd.lock.RUnlock() // Drop the lock

	// Respond to the client
	fmt.Fprintf(w, string(returnString))
}

// Thread to read service updates and write the updates to ScoreboardState. We do this so
// we don't have to give every status checking thread the ability to
// RW lock the ScoreboardState. This lets us test services without locking.
// This function read locks for determining if an update should be applied to the
// Scoreboard State. If an update needs to be applied, the function drops its read lock
// and establishes a write lock to update the data. The write lock is maintained for as long as there
// are service updates that need to be analyzed. If no write lock is established, the function maintains
// it's read lock as long as there are service updates that need to be analyzed.
//
// The end goal of this complex locking is to minimize the time spent holding a
// write lock. however, once this function has establish a write lock,
// don't drop it because it might need to be re-established nano-seconds later.
// This function read locks for safety reasons.
func (sbd *State) StateUpdater(updateChannel chan ServiceUpdate, output chan string) {

	// These two flags are mutually exclusive. One being set does not rely on the other
	// which is why we have two of them, instead of expressing their logic with a single flag.
	// This function will drop it's read lock when it's in a sleeping state,
	// and only establishes a read lock when needing to find data that might be
	// changed, and only then establishing a write lock **if** that data needs to be
	// changed. A write lock or a read lock is kept until there is no more
	// data to be parsed through.
	var (
		isWriteLocked = false // Flag to hold whether we already have a lock or not.
		isReadLocked  = false // Flag to hold whether we have a read lock.
	)

	for {
		// A service update that we are waiting for
		var update ServiceUpdate

		// Test for there being another service update on the line
		select {
		case update = <-updateChannel: // There is another update on the line

			// Read-Lock to be safe.
			if !isWriteLocked && !isReadLocked {
				sbd.lock.RLock()
				isReadLocked = true
			}

			// Interate down to the Service or Host that needs to be updated
			for indexOfHosts, host := range sbd.Hosts {
				if update.Ip == sbd.Hosts[indexOfHosts].Ip {
					if update.ServiceUpdate { // Is the update a service update, or an ICMP update?
						// It's a service update so iterate down to the service that needs to be updated.
						for indexOfServices, service := range host.Services {
							if service.Name == update.ServiceName {
								// Decide if the update contradicts the current Scoreboard State.
								// If it does, we need to establish a Write lock before changing
								// the service state.
								if sbd.Hosts[indexOfHosts].Services[indexOfServices].IsUp != update.IsUp {
									if !isWriteLocked { // If we already have a RW lock, don't que another
										sbd.lock.RUnlock() // Unlock our Read lock before Write Locking
										isReadLocked = false
										sbd.lock.Lock() // WRITE LOCK
										isWriteLocked = true
									}

									// Debug that we received a service update
									output <- fmt.Sprintf("Received a service update for %v on %v. Status: %v. "+
										"Updating Scoreboard", update.ServiceName,
										sbd.Hosts[indexOfHosts].Name, update.IsUp)

									// Update that services state
									sbd.Hosts[indexOfHosts].Services[indexOfServices].IsUp = update.IsUp
								} else {
									// Debug that we received a service update
									output <- fmt.Sprintf("Received a service update for %v on %v. Status: %v. "+
										"Not Updating Scoreboard", update.ServiceName,
										sbd.Hosts[indexOfHosts].Name, update.IsUp)
								}
							}
						}
					} else {
						// We are dealing with an ICMP update. We need to determine if the
						// Scoreboard State needs to be updated.
						if sbd.Hosts[indexOfHosts].PingUp != update.IsUp { // We need to establish a write lock
							if !isWriteLocked { // If we already have a RW lock, don't que another
								sbd.lock.RUnlock()
								isReadLocked = false
								sbd.lock.Lock() // WRITE LOCK
								isWriteLocked = true
							}

							// Debug print the service update
							output <- fmt.Sprintf("Received a ping update for %v on %v. Status: %v. "+
								"Updating Scoreboard", update.Ip,
								sbd.Hosts[indexOfHosts].Name, update.IsUp)

							sbd.Hosts[indexOfHosts].PingUp = update.IsUp
						} else {
							// Debug print the service update
							output <- fmt.Sprintf("Received a ping update for %v on %v. Status: %v. "+
								"Not Updating Scoreboard", update.Ip,
								sbd.Hosts[indexOfHosts].Name, update.IsUp)
						}
					}
				}
			}
		default: // There is not another update on the line, so we'll wait for one
			// If we have a write lock because we changed the ScoreboardState
			// because of an ServiceUpdate, release the Write lock so clients
			// can view content. Otherwise, we had a read lock that needs to
			// be released because we don't need it any longer.
			if isWriteLocked {
				sbd.lock.Unlock()
				isWriteLocked = false
			} else if isReadLocked { // This isn't a else case because this default case might be ran quickly in succession
				sbd.lock.RUnlock()
				isReadLocked = false
			}

			// Wait 1 second, then check for ServiceUpdates again!
			time.Sleep(1 * time.Second)
		}
	}

}
