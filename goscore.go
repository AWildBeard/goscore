package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"time"
)

var (
    // Variables
    defaultConfigFile = "config.yaml"
    configFileString string
    debug bool
    buildCfg bool

    // Logger
    ilog *log.Logger
    dlog *log.Logger
)

func init() {
	execPath, _ := os.Executable()
	configFileString = fmt.Sprintf("%v/%v", path.Dir(execPath), defaultConfigFile)
	cwd, _ := os.Getwd()

	flag.StringVar(&configFileString, "c", configFileString,
	    "Specify a custom config file location")
	flag.BoolVar(&debug, "d", false, "Print debug messages")
	flag.BoolVar(&buildCfg, "buildcfg", false, "Output an example configuration file " +
		"to " + cwd + "/config.yaml")
}

func main() {
	// Read command line flags
    flag.Parse()

    // Initialize logging devices
	ilog = log.New(os.Stdout, "", 0)

	// Initialize debug output if relevant
    if debug {
    	// We want debug, so output to STDERR
        dlog = log.New(os.Stderr, "DBG: ", log.Ltime)
    } else {
    	// We don't wand debug so write to a void
        dlog = log.New(ioutil.Discard, "", 0)
    }

	if buildCfg { //buildcfg flag was set so write a config and exit
		buildConfig()
		os.Exit(0)
	}

	var (
		// Create a new scoreboard
		scoreboard = NewScoreboard()

		// Make a buffered channel to write service updates over. These updates will get read by a thread
		// that will write lock ScoreboardState
		updateChannel = make(chan ServiceUpdate, 10)
	)

	// Read and parse the config file
    if config, err := initConfig() ; err == nil { // Initialize the config

    	// Parse the config to the scoreboard
    	if err := parseConfigToScoreboard(&config, &scoreboard) ; err != nil { // Failed to parse config
    		ilog.Println("Failed to parse config:", err)
    		os.Exit(1)

		} else { // Successfully parsed, now debug print the details
			if scoreboard.config.pingHosts {
				dlog.Println("Ping hosts:", boolToWord(scoreboard.config.pingHosts))
				dlog.Println("Ping timeout:", scoreboard.config.PingTimeout)
				dlog.Println("Time between ping checking hosts:", scoreboard.config.TimeBetweenPingChecks)
			}

			dlog.Println("Service timeout:", scoreboard.config.ServiceTimeout)
			dlog.Println("Time between service checking hosts:", scoreboard.config.TimeBetweenServiceChecks)
		}

	} else {
		ilog.Println("Config not found, if you would like to generate one, " +
			"run this program again with the -buildcfg flag or use the -c flag to " +
			"specify your own config!")
		os.Exit(1)

	}

	// Test privileges for ICMP and opening port 80. Exit uncleanly if incorrect privileges are used.
	testPrivileges()

    if scoreboard.config.pingHosts { // The ping option was set

    	// Thread for pinging hosts. Results are shipped to the ScoreboardStateUpdater as ServiceUpdate's
		go func (channel chan ServiceUpdate, services []Service, scoreboardConfig ScoreboardConfig) {

			ilog.Println("Started the Ping Check Provider")

			for {
				// Sleep for the configured amount of time before trying to ping hosts
				time.Sleep(scoreboardConfig.TimeBetweenPingChecks)

				for _, service := range services {
					// Asyncronously ping services so we don't wait full timeouts and can ping faster.
					go pingHost(channel, service.Ip, scoreboardConfig.PingTimeout)
				}
			}
		} (updateChannel, scoreboard.Services, scoreboard.config)
	}

	// Thread for querying services. Results are shipped to the ScoreboardStateUpdater as ServiceUpdate's
    go func (channel chan ServiceUpdate, services []Service, config ScoreboardConfig) {

    	ilog.Println("Started the Service Check Provider")

    	for {
    		// Wait the configured amount of time before initiating threads to query services.
    		time.Sleep(config.TimeBetweenServiceChecks)

			for _, service := range services {
				go checkService(channel, service, config.ServiceTimeout)
			}
		}
	} (updateChannel, scoreboard.Services, scoreboard.config)

	// Thread to read service updates and write the updates to ScoreboardState. We do this so
	// we don't have to give every thread the ability to RW lock the ScoreboardState. This lets us test services
	// without locking and only locks for updates to the ScoreboardState, as it should be :D. The RW lock is maintained
	// for as long as there are service updates to be written to ScoreboardState.
	go func (sbd *ScoreboardState) {

		ilog.Println("Started the Scoreboard State Updater")

		var (
			wasLocked = false // Flag to hold wether we already have a lock or not.
		)

		for {
			// A service update that we are waiting for
			var update ServiceUpdate

			// Test for there being another service update on the line
			select {
			case update = <- updateChannel: // There is another update on the line
				if !wasLocked { // If we already have a RW lock, don't que another
					sbd.lock.Lock()
					wasLocked = true
				}

				// Write the update
				for i := range sbd.Services { // Classify the update and change variables as needed
					if update.Ip == sbd.Services[i].Ip {
						if update.ServiceUpdate {
							dlog.Printf("Received a service update from %v:%v. Status: %v", update.Ip,
								sbd.Services[i].Ports[update.ServiceUpIndex], boolToWord(update.ServiceUp))

							sbd.Services[i].ServicesUp[update.ServiceUpIndex] = update.ServiceUp
						} else {
							dlog.Printf("Received a ping update from %v. Status: %v", update.Ip,
								boolToWord(update.ServiceUp))

							sbd.Services[i].PingUp = update.ServiceUp
						}
					}
				}
			default: // There is not another update on the line, so we'll wait for one
				if wasLocked { // If we have a lock because we wrote an update, unlock so clients can view content.
					sbd.lock.Unlock()
					wasLocked = false
				}

				// Wait 10 seconds, then check again!
				time.Sleep(10 * time.Second)
			}
		}

	} (&scoreboard)

    // Register '/' with ScoreboardState
	http.Handle("/", &scoreboard)

	ilog.Println("Started Webserver")

    // Start the webserver and serve content
	http.ListenAndServe(":80", nil)
}
