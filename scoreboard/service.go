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

// An individual Service that is contained by a Host.
// Service implements UptimeTracking
type Service struct {
	// The name of the Service this struct represents
	Name string `yaml:"service"`

	// The Port that the Service is hosted on
	Port string `yaml:"port"`

	// The String to write to the remote Service.
	// This is optional and can be an empty string
	// if Protocol is not host-command
	Command string `yaml:"command"`

	// A Regular Expression that can match the expected
	// response from the remote Service. This is optional
	// and can be and empty string.
	Response string `yaml:"response"`

	// The Layer 4 Protocol used to connect to the Service.
	// I.E. 'tcp', 'udp', or 'host-command' to run a system command
	Protocol string `yaml:"protocol"`

	// Boolean flag to represent whether the service is currently up
	isUp bool

	// Times to detail how long the service has been up or down
	uptime time.Duration

	downtime time.Duration

	previousUpdateTime time.Time
}

// Struct to hold an update to a service held by ScoreboardState
type ServiceUpdate struct {
	// The IP of the host who's service update this is for.
	// This is used as a unique identifier to identify hosts.
	Ip string

	// If true, this ServiceUpdate contains data on an update to a service,
	// otherwise, this is a ICMP update report.
	ServiceUpdate bool

	// Flag to represent whether the Service is up, or if ServiceUpdate is
	// false, this flag represents if ICMP is up for the remote host
	IsUp bool

	// This variable contains the name of the service to update.
	// This is used to uniquely identify services contained
	// within hosts for the Scoreboard State Updater
	ServiceName string
}

func (service *Service) IsUp() bool {
	return service.isUp
}

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

func (service *Service) GetUptime() time.Duration {
	if service.isUp {
		return service.uptime + time.Now().Sub(service.previousUpdateTime)
	}

	return service.uptime
}

func (service *Service) GetDowntime() time.Duration {
	if !service.isUp {
		return service.downtime + time.Now().Sub(service.previousUpdateTime)
	}

	return service.downtime
}

// This function checks a single service in the predefined
// manner contained in the Service type.
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
