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
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	standardScoreboardDoc = `<!DOCTYPE HTML>
<html>
	<head>
		<meta charset="UTF-8">
		<title>{{ .Title }}</title>
		<style>
body {
  display: flex;
  font-family: arial, serif;
  justify-content: center;
  background-color: #133f7c;
  height: 100%;
  margin: 0;
  padding: 0;
  insets: 0;
}
h2 {
  margin: 5vh 0 5vh 0;
  display: flex;
  flex: 0;
  justify-content: center;
}
.serviceTable {
  height: calc(100vh - 4vh);
  padding: 0 10vw 0 10vw;
  display: flex;
  justify-content: flex-start;
  margin: 2vh 0 2vh 0;
  flex-direction: column;
  background-color: white;
  border-radius: 2vmin;
  box-shadow: 0 0 1vmin #133f7c;
}
.footer {
  display: flex;
  flex: 2;
  align-self: flex-end;
  width: 100%;
  justify-content: center;
  font-size: 10pt;
}
.footer i {
  align-self: flex-end;
  margin: 3vh 0 3vh 0;
}
.serviceTable table {
  display: flex;
  flex: 1;
  justify-content: center;
  border-collapse: collapse;
  border: none;
}
.serviceTable th {
  border: solid thin black;
  background-color: black;
  color: white;
  padding: 0.5vmax 3vmax 0.5vmax 3vmax;
  vertical-align: middle;
}
.serviceTable td {
  border: solid thin black;
  vertical-align: top;
  padding: 0.5vh 1vw;
}
.up {
  background-color: green;
}
.down {
  background-color: red;
}
		</style>
	</head>
	<body>
		<div class="serviceTable">
		<h2>{{.Title}} Scoreboard</h2>
		<table>
			<tr>
				<th>Host</th>
				<th>Service</th>
				<th>State</th>
			</tr>
			{{ $pingHosts := .PingHosts }} {{ range $hostIndex, $host := .Hosts }} {{ range $serviceIndex, $service := index $host.Services }} <tr>
				<td>{{ $host.Name }}</td>
				<td>{{ $service.Name }}</td> {{ if $pingHosts }} {{ if and $host.IsUp $service.IsUp }}
				<td class="up">Online</td> {{ else }}
				<td class="down">Offline</td> {{ end }} {{ else }} {{ if $service.IsUp }}
				<td class="up">Online</td> {{ else }}
				<td class="down">Offline</td> {{ end }} {{ end }}
			</tr> {{ end }} {{ end }}
		</table>
		<div class="footer">
		<i>Created by Michael Mitchell for the UWF CyberSecurity Club</i>
		</div>
		</div>
	</body>
</html>
`
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

	// The name of the competition. Used in the web interface.
	Name string

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

	// The default service state for all services and hosts.
	// If the user is wanting to test services that will all be up
	// at the beginning of the CTF, setting this to true will give
	// accurate uptimes and downtimes to that situation. If the user
	// expects the services to be down at the beginning, setting this
	// to false will give accurate uptimes and wontimes for that
	// usecase.
	DefaultServiceState bool
}

type UptimeTracking interface {
	SetUp(state bool)
	IsUp() bool
	GetUptime() time.Duration
	GetDowntime() time.Duration
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
			true,
		},
		"",
		sync.RWMutex{},
	}
}

func (sbd *State) StartScoring() {
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
}

// Function to serve the `index.html` for the scoreboard.
// Implements ServeHTTP for ScoreboardState
func (sbd *State) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if tmplt, err := template.New("scoreboard").Parse(standardScoreboardDoc); err == nil {
		// Establish a read-only lock to the scoreboard to retrieve data,
		// then drop the lock after we have retrieved that data we need.
		sbd.lock.RLock()
		data := struct {
			Title     string
			Hosts     []Host
			PingHosts bool
		}{
			sbd.Name,
			sbd.Hosts,
			sbd.Config.PingHosts,
		}
		sbd.lock.RUnlock() // Drop the lock

		// Respond to the client
		if err := tmplt.Execute(w, data); err != nil {
			fmt.Println("ERRORED ON HTML TEMPLATE EXECUTE:", err)
		}
	} else {
		fmt.Println("ERRORED ON HTML TEMPLATE CREATION:", err)
		os.Exit(1)
	}
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
			for indexOfHosts := range sbd.Hosts {
				// Get a reference to the host
				host := &sbd.Hosts[indexOfHosts]

				if update.Ip == host.Ip {
					// Found the correct host

					if update.ServiceUpdate { // Is the update a service update, or an ICMP update?

						// It's a service update so iterate down to the service that needs to be updated.
						for indexOfServices := range host.Services {

							// Get a reference to the service
							service := &host.Services[indexOfServices]

							if service.Name == update.ServiceName {
								// Found the correct service

								// Decide if the update contradicts the current Scoreboard State.
								// If it does, we need to establish a Write lock before changing
								// the service state.
								if service.isUp != update.IsUp {
									if !isWriteLocked { // If we already have a RW lock, don't que another
										sbd.lock.RUnlock() // Unlock our Read lock before Write Locking
										isReadLocked = false
										sbd.lock.Lock() // WRITE LOCK
										isWriteLocked = true
									}

									// Update that services state
									service.SetUp(update.IsUp)

									// Debug that we received a service update
									output <- fmt.Sprintf("Received a service update for %v on %v.\n"+
										"\tStatus: %v -> Needed to update scoreboard\n"+
										"\tUptime: %v, Downtime: %v", service.Name,
										host.Name, update.IsUp,
										service.GetUptime(), service.GetDowntime())

								} else {
									// Debug that we received a service update
									output <- fmt.Sprintf("Received a service update for %v on %v.\n"+
										"\tStatus: %v -> Didn't need to update scoreboard\n"+
										"\tUptime: %v, Downtime: %v", service.Name,
										host.Name, update.IsUp,
										service.GetUptime(), service.GetDowntime())

								}

								break // We found the correct service so stop searching
							}
						}
					} else {

						// We are dealing with an ICMP update. We need to determine if the
						// Scoreboard State needs to be updated.
						if host.isUp != update.IsUp { // We need to establish a write lock
							if !isWriteLocked { // If we already have a RW lock, don't que another
								sbd.lock.RUnlock()
								isReadLocked = false
								sbd.lock.Lock() // WRITE LOCK
								isWriteLocked = true
							}

							host.SetUp(update.IsUp)

							// Debug print the service update
							output <- fmt.Sprintf("Received a ping update for %v on %v.\n"+
								"\tStatus: %v -> Needed to update scoreboard.\n"+
								"\tUptime: %v, Downtime: %v", host.Ip,
								host.Name, host.isUp,
								host.GetUptime(), host.GetDowntime())

						} else {
							// Debug print the service update
							output <- fmt.Sprintf("Received a ping update for %v on %v.\n"+
								"\tStatus: %v -> Didn't need to update scoreboard.\n"+
								"\tUptime: %v, Downtime: %v", host.Ip,
								host.Name, host.isUp,
								host.GetUptime(), host.GetDowntime())
						}
					}

					break // We found the correct host, so stop searching
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
