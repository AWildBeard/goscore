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
func (sbd *State) WebContentUpdater(shutdown chan bool) {
	ilog.Println("Started the Webpage Content Updater")

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

	for {
		select {
		case <-shutdown:
			ilog.Println("Shutting down the Webpage Content Updater")
			return
		default:
			/* TODO: re-implement a more complex way to determine if an update
			 * to the webSheet needs to be performed. Currently an update is performed
			 * every time because of changes to timers that are displayed in the web content
			 */
			byteBuf := bytes.Buffer{}

			// Put a few basic functions into the template to make using templates easier
			tmplt, err := template.New("scoreboard").Funcs(template.FuncMap{
				"Uptime":         upFunc,
				"Downtime":       downFunc,
				"FormatDuration": fmtDuration,
			}).Parse(sbd.Config.ScoreboardDoc)

			if err != nil {
				fmt.Println("ERRORED ON HTML TEMPLATE CREATION:", err)
				os.Exit(1)
			}

			sbd.lock.RLock()

			// Establish a read-only lock to the scoreboard to retrieve data,
			// then drop the lock after we have retrieved that data we need.
			data := struct {
				Title     string
				Hosts     []Host
				PingHosts bool
				TimeLeft  time.Duration
			}{
				sbd.Name,
				sbd.Hosts,
				sbd.Config.PingHosts,
				sbd.TimeLeft(),
			}

			// Respond to the client
			if err = tmplt.Execute(&byteBuf, data); err != nil {
				fmt.Println("ERRORED ON HTML TEMPLATE EXECUTE:", err)
				os.Exit(1)
			}

			sbd.lock.RUnlock()

			newContent := byteBuf.Bytes()
			updated := false

			sbd.templateLock.RLock()
			if len(sbd.webSheet) != len(newContent) {
				sbd.templateLock.RUnlock()
				sbd.templateLock.Lock()

				sbd.webSheet = newContent

				sbd.templateLock.Unlock()

				updated = true
			} else {
				for _, websheetByte := range sbd.webSheet {
					for _, newContentByte := range newContent {
						if websheetByte != newContentByte { // A change is needed
							sbd.templateLock.RUnlock()
							sbd.templateLock.Lock()

							sbd.webSheet = newContent

							sbd.templateLock.Unlock()

							updated = true
							break
						}
					}

					if updated {
						break
					}
				}
			}

			if !updated {
				sbd.templateLock.RUnlock()
			}

			time.Sleep(5 * time.Second)
		}
	}
}

// ServeHTTP serves the `index.html` for the scoreboard.
// Implements ServeHTTP for State
func (sbd *State) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sbd.templateLock.RLock()
	io.Copy(w, bytes.NewReader(sbd.webSheet))
	sbd.templateLock.RUnlock()
}
