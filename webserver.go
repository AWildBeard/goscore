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
	"html/template"
	"io"
	"net/http"
	"os"
	"time"
)

// WebContentUpdater is a thread that is started be Start() to update the web interface.
// It updates the template every 5 seconds by default right now.
func (sbd *State) WebContentUpdater(update, shutdown chan bool) {
	// TODO: create sub templates for timers?
	// By doing this we might save some compute power on regenerating
	// the entire web content. We might not though, and this would just
	// be a feel good change. If timers are segmented to a subtemplate,
	// then the correct place to execute the subtemplate would be in scoreboardResponder

	ilog.Println("Started the Webpage Content Updater")

	data := struct {
		Title     string
		Hosts     []Host
		PingHosts bool
		TimeLeft  time.Duration
	}{}

	sbd.lock.RLock()

	data.Title = sbd.Name

	data.Hosts = make([]Host, len(sbd.Hosts))
	copy(data.Hosts, sbd.Hosts)

	for i := range data.Hosts {
		host := &(data.Hosts[i])
		host.Services = make([]Service, len(sbd.Hosts[i].Services))
		copy(host.Services, sbd.Hosts[i].Services)
	}

	data.PingHosts = sbd.Config.PingHosts
	data.TimeLeft = sbd.TimeLeft()

	sbd.lock.RUnlock()

	byteBuf := bytes.Buffer{}

	upFunc := func(tracker interface{}) time.Duration {
		var duration time.Duration
		switch tracker.(type) {
		case Host:
			host := tracker.(Host)
			duration = sbd.GetUptime(&host)
		case Service:
			service := tracker.(Service)
			duration = sbd.GetUptime(&service)
		default:
			ilog.Println("Invalid use of Uptime function")
			os.Exit(1)
		}

		return duration
	}

	downFunc := func(tracker interface{}) time.Duration {
		var duration time.Duration
		switch tracker.(type) {
		case Host:
			host := tracker.(Host)
			duration = sbd.GetDowntime(&host)
		case Service:
			service := tracker.(Service)
			duration = sbd.GetDowntime(&service)
		default:
			ilog.Println("Invalid use of Downtime function")
			os.Exit(1)
		}

		return duration
	}

	tmplt := template.Template{}

	// Put a few basic functions into the template to make using templates easier
	if newTemplate, err := template.New("scoreboard").Funcs(template.FuncMap{
		"Uptime":         upFunc,
		"Downtime":       downFunc,
		"FormatDuration": fmtDuration,
	}).Parse(sbd.Config.ScoreboardDoc); err == nil {
		tmplt = *newTemplate
	} else {
		fmt.Println("ERRORED ON HTML TEMPLATE CREATION:", err)
		os.Exit(1)
	}

	if err := tmplt.Execute(&byteBuf, data); err != nil {
		fmt.Println("ERRORED ON HTML TEMPLATE EXECUTE:", err)
		os.Exit(1)
	}

	for {
		// Update the web sheet with new data
		sbd.webContentLock.Lock()
		sbd.webSheet = byteBuf.Bytes()
		sbd.webContentLock.Unlock()

		time.Sleep(1 * time.Second)

		// Clear the buffer for new data
		byteBuf.Reset()

		select {
		case <-shutdown:
			// Establish a read-only lock to the scoreboard to retrieve data,
			// then drop the lock after we have retrieved that data we need.
			sbd.lock.RLock()

			copy(data.Hosts, sbd.Hosts)
			for i := range data.Hosts {
				host := &(data.Hosts[i])
				copy(host.Services, sbd.Hosts[i].Services)
			}
			data.TimeLeft = sbd.TimeLeft()

			sbd.lock.RUnlock()

			// Update the template with the new data
			tmplt.Execute(&byteBuf, data)

			// Update the web sheet with that data
			sbd.webContentLock.Lock()
			sbd.webSheet = byteBuf.Bytes()
			sbd.webContentLock.Unlock()

			// Exit
			ilog.Println("Shutting down the Webpage Content Updater")
			return
		case <-update:
			// Establish a read-only lock to the scoreboard to retrieve data,
			// then drop the lock after we have retrieved that data we need.
			sbd.lock.RLock()

			copy(data.Hosts, sbd.Hosts)
			for i := range data.Hosts {
				host := &(data.Hosts[i])
				copy(host.Services, sbd.Hosts[i].Services)
			}

			sbd.lock.RUnlock()
		default:
			// Do nothing, just don't hang.
		}

		// Safe because TimeLeft() is a read only function on data that
		// doesn't change for the life of program.
		data.TimeLeft = sbd.TimeLeft()

		// Update the template with the new data
		tmplt.Execute(&byteBuf, data)
	}
}

// scoreboardResponder serves the `index.html` for the scoreboard.
// Implements scoreboardResponder for State
func (sbd *State) scoreboardResponder(w http.ResponseWriter, r *http.Request) {
	sbd.webContentLock.RLock()
	io.Copy(w, bytes.NewReader(sbd.webSheet))
	sbd.webContentLock.RUnlock()
}
