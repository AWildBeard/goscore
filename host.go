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
	"github.com/sparrc/go-ping"
	"time"
)

// Host represents a Host that contains Services
type Host struct {
	// Name is the name of the host give in the config file
	Name string `yaml:"host"`

	// Services are the service(s) provided on the host
	Services []Service `yaml:"services"`

	// IP is the IP address of a Host
	IP string `yaml:"ip"`

	// A flag used to represent whether a Host is responding to ICMP
	isUp bool

	// Time to represent how long the host has been responding to ICMP
	uptime time.Duration

	// Time to represent how long the host has not been responding to ICMP
	downtime time.Duration

	// Variable to represent the last time the Host's service state
	// (isUp) was updated.
	previousUpdateTime time.Time
}

// IsUp implements UptimeTracking for Host. This method provides
// a public way to access the Host's up state
func (host *Host) IsUp() bool {
	return host.isUp
}

// SetUp implements UptimeTracking for Host. This method provides
// a way to change the state of the Host's up state. At the same
// time this method also deals with changes to the uptime and
// downtime tracking functionality.
func (host *Host) SetUp(state bool) {
	if host.isUp != state {
		now := time.Now()
		host.isUp = state

		if host.isUp { // Service is up so calculate how long it was down
			host.downtime = host.downtime + now.Sub(host.previousUpdateTime)
		} else { // Service is down, so calculate how long it was up
			host.uptime = host.uptime + now.Sub(host.previousUpdateTime)
		}

		host.previousUpdateTime = now
	}

}

// GetUptime implements UptimeTracking for Host. GetUptime allows for
// querying and returning accurate durations of uptime with respect
// to the referenceTime provided to the function for the Host.
func (host Host) GetUptime(referenceTime time.Time) time.Duration {
	if host.isUp {
		return host.uptime + referenceTime.Sub(host.previousUpdateTime)
	}

	return host.uptime
}

// GetDowntime implements UptimeTracking for Host. GetDowntime
// allows for querying accurate durations of downtime with respect
// to the referenceTime provided to the function for the Host.
func (host Host) GetDowntime(referenceTime time.Time) time.Duration {
	if !host.isUp {
		return host.downtime + referenceTime.Sub(host.previousUpdateTime)
	}

	return host.downtime
}

// PingHost allows for checking if a host is online by using ICMP.
// Results are shipped as ServiceUpdates through updateChannel.
// This function gives the remote host three chances to respond
// before the timeout specified is reached. As long as one response
// is received in this time period, the host is marked as up.
func (host *Host) PingHost(updateChannel chan ServiceUpdate, timeout time.Duration) {
	pingSuccess := false
	hostToPing := host.IP

	if pinger, err := ping.NewPinger(hostToPing); err == nil {
		pinger.Timeout = timeout
		pinger.SetPrivileged(true)
		pinger.Count = 3
		pinger.Run() // Run the pinger

		stats := pinger.Statistics() // Get the statistics for the ping from the pinger

		pingSuccess = stats.PacketsRecv != 0 // Test if packets were received
	}

	updateChannel <- ServiceUpdate{
		hostToPing,
		false,       // This is an ICMP update
		pingSuccess, // Whether the ping was successful
		"",          // Set this to an empty string.
	}
}
