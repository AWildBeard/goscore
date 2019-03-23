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
	"bytes"
	"fmt"
	"io"
	"net"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// Service represents an individual Service that is contained
// by a particular Host. Service implements UptimeTracking
type Service struct {
	// Name is the name of the Service this struct represents
	Name string `yaml:"service"`

	// Port is the Port that the Service is hosted on
	Port string `yaml:"port"`

	// Command is the string to write to the remote Service.
	// This is optional and can be an empty string
	// if Protocol is not host-command. It can also represent
	// a host-level command to be used instead of manually opening
	// a socket and manually writing a connection string.
	Command string `yaml:"command"`

	// Response is a regular expression that can match the expected
	// response from the remote Service or command. This is optional
	// if protocol is not 'host-command'.
	Response string `yaml:"response"`

	// Protocol is the layer 4 protocol used to connect to the Service
	// or it can be 'host-command' to signify that running a system
	// level command should occur in the place of this program opening
	// a socket and manually testing the service.
	// I.E. 'tcp', 'udp', or 'host-command' to run a system command
	Protocol string `yaml:"protocol"`

	// Boolean flag to represent whether the service is currently up
	isUp bool

	// Time to represent how long the Service has been responding to Command
	uptime time.Duration

	// Time to represent how long the Service has not been responding to Command
	downtime time.Duration

	// Variable to represent the last time the Service's service state
	// (isUp) was updated.
	previousUpdateTime time.Time
}

// ServiceUpdate is the type used to ship updates from update functions
// to the StateUpdater thread.
type ServiceUpdate struct {
	// IP is the IP of the host who's service update this is for.
	// This is used as a unique identifier to identify hosts.
	IP string

	// ServiceUpdate is a flag that if true, represents data
	// on an update to a service, otherwise, this is a ICMP update.
	ServiceUpdate bool

	// IsUp is a flag to represent whether the Service is up,
	// or if ServiceUpdate is false, this flag represents if
	// ICMP is up for the remote host
	IsUp bool

	// ServiceName is the name of the service to update.
	// This is used to uniquely identify services contained
	// within hosts for the StateUpdater
	ServiceName string
}

// IsUp implements UptimeTracking for Service. This method provides
// a public way to access the Services's up state
func (service *Service) IsUp() bool {
	return service.isUp
}

// SetUp implements UptimeTracking for Service. This method provides
// a way to change the state of the Service's up state. At the same
// time this method also deals with changes to the uptime and
// downtime tracking functionality.
func (service *Service) SetUp(state bool) {
	if service.isUp != state {
		now := time.Now()
		service.isUp = state

		if service.isUp { // Service is up so calculate how long it was down
			service.downtime = service.downtime + now.Sub(service.previousUpdateTime)
		} else { // Service is down, so calculate how long it was up
			service.uptime = service.uptime + now.Sub(service.previousUpdateTime)
		}

		service.previousUpdateTime = now
	}

}

// GetUptime implements UptimeTracking for Service. GetUptime allows for
// querying and returning accurate durations of uptime with respect
// to the referenceTime provided to the function for the Service.
func (service *Service) GetUptime(referenceTime time.Time) time.Duration {
	if service.isUp {
		return service.uptime + referenceTime.Sub(service.previousUpdateTime)
	}

	return service.uptime
}

// GetDowntime implements UptimeTracking for Service. GetDowntime
// allows for querying accurate durations of downtime with respect
// to the referenceTime provided to the function for the Service.
func (service *Service) GetDowntime(referenceTime time.Time) time.Duration {
	if !service.isUp {
		return service.downtime + referenceTime.Sub(service.previousUpdateTime)
	}

	return service.downtime
}

// CheckService is a method called as a thread to check a specific service on a specific host.
// This function checks a single service in the predefined manner contained within the
// Service type. Results are shipped as the ServiceUpdate type via the updateChannel.
func (service *Service) CheckService(updateChannel chan ServiceUpdate, ip string, timeout time.Duration) {
	serviceUp := false

	if service.Protocol == "host-command" {
		var (
			command      = strings.Split(service.Command, " ")
			regexToMatch = fmt.Sprint(service.Response)
			sig          = make(chan bool, 1)
			cmd          *exec.Cmd
			stdout       = bytes.Buffer{}
			stderr       = bytes.Buffer{}
		)

		if len(command) > 1 {
			cmd = exec.Command(command[0], command[1:]...)
		} else {
			cmd = exec.Command(command[0])
		}

		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		cmd.Start()

		time.AfterFunc(timeout, func() {
			select {
			case <-sig:
				return
			default:
				if cmd.Process != nil {
					syscall.Kill(cmd.Process.Pid, syscall.SIGKILL)
				}
			}
		})

		cmd.Wait()
		sig <- true

		foundInStdout, _ := regexp.Match(regexToMatch, stdout.Bytes())
		foundInStderr, _ := regexp.Match(regexToMatch, stderr.Bytes())

		serviceUp = foundInStdout || foundInStderr
	} else {
		if conn, err := net.DialTimeout(service.Protocol,
			fmt.Sprintf("%v:%v", ip, service.Port), timeout); err == nil {

			stringToSend := fmt.Sprint(service.Command)
			regexToMatch := fmt.Sprint(service.Response)

			conn.SetDeadline(time.Now().Add(timeout))

			if len(stringToSend) > 0 {
				io.Copy(conn, strings.NewReader(stringToSend)) // Write what we need to write.
			}

			// No sense of even bothering to read the response if we aren't
			// going to do anything with it.
			if len(regexToMatch) > 0 {
				buffer := bytes.Buffer{}
				io.Copy(&buffer, conn) // Read the response
				serviceUp, _ = regexp.Match(regexToMatch, buffer.Bytes())
			} else {
				serviceUp = true
			}

			conn.Close()
		}
	}

	// Write the service update
	updateChannel <- ServiceUpdate{
		ip,
		true,
		serviceUp,
		service.Name,
	}
}
