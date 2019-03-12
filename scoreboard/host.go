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
	"github.com/sparrc/go-ping"
	"time"
)

// Struct to represent a Host that contains Services
type Host struct {
	// The name of the host give in the config file
	Name string `yaml:"host"`

	// The service(s) provided on the host
	Services []Service `yaml:"services"`

	// The IP Address of a Host
	Ip string `yaml:"ip"`

	// A flag used to represent whether a Host is responding to ICMP
	PingUp bool
}

// Function to ping a host at an IP. Results are shipped as ServiceUpdates through
// updateChannel. This function gives the remote host three chances to respond.
// As long as one response is received, the host is marked as up.
func (host *Host) PingHost(updateChannel chan ServiceUpdate, timeout time.Duration) {
	pingSuccess := false
	hostToPing := host.Ip

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
