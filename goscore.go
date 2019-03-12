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
	"flag"
	"fmt"
	"github.com/AWildBeard/goscore/scoreboard"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/user"
	"path"
	"runtime"
	"strings"
	"time"
)

const defaultConfigFileName string = "config.yaml"

var (
	// Command line options
	defaultConfigFileLocation string
	debug                     bool
	buildCfg                  bool

	// Logging factories
	ilog *log.Logger
	dlog *log.Logger
)

func init() {
	// Determine the path to this executable
	execPath, _ := os.Executable()

	// Set the default for configFileLocation which has to be determined at runtime.
	defaultConfigFileLocation = fmt.Sprintf("%v/%v", path.Dir(execPath), defaultConfigFileName)

	cwd, _ := os.Getwd()

	flag.StringVar(&defaultConfigFileLocation, "c", defaultConfigFileLocation,
		"Specify a custom config file location")
	flag.BoolVar(&debug, "d", false, "Print debug messages")
	flag.BoolVar(&buildCfg, "buildcfg", false, "Output an example configuration file "+
		"to "+cwd+"/config.yaml")

	flag.Usage = usage
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

	if buildCfg { // buildcfg flag was set so write a config and exit
		buildConfig()
		os.Exit(0)
	}

	var (
		// Create a new scoreboard
		sbd = scoreboard.NewScoreboard()

		// Make a buffered channel to write service updates over. These updates will get read by a thread
		// that will write lock ScoreboardState
		updateChannel = make(chan scoreboard.ServiceUpdate, 10)
	)

	// Read and parse the config file
	if config, err := initConfig(); err == nil { // Initialize the config

		if err := config.validateConfig(); err != nil {
			ilog.Println(err)
			os.Exit(1)
		}

		// Parse the config to the scoreboard
		if err := parseConfigToScoreboard(&config, &sbd); err != nil { // Failed to parse config
			ilog.Println("Failed to parse config:", err)
			os.Exit(1)

		} else { // Successfully parsed, now debug print the details
			if sbd.Config.PingHosts {
				dlog.Println("Ping hosts:", boolToWord(sbd.Config.PingHosts))
				dlog.Println("Ping timeout:", sbd.Config.PingTimeout)
				dlog.Println("Time between ping checking hosts:", sbd.Config.TimeBetweenPingChecks)
			}

			dlog.Println("Service timeout:", sbd.Config.ServiceTimeout)
			dlog.Println("Time between service checking hosts:", sbd.Config.TimeBetweenServiceChecks)
		}

	} else {
		ilog.Println("Critical configuration file error encountered:", err)
		ilog.Println("This might be because the Config file wasn't found. " +
			"If this was the problem; run this program again with the " +
			"-buildcfg flag to generate a config or use the -c flag to " +
			"specify your a different config!")
		os.Exit(1)

	}

	// Test privileges for ICMP and opening port 80. Exit uncleanly if incorrect privileges are used.
	testPrivileges()

	if sbd.Config.PingHosts { // The ping option was set

		// Thread for pinging hosts. Results are shipped to the
		// ScoreboardStateUpdater (defined below) as ServiceUpdates.
		// We don't read-lock these threads with the Scoreboard State
		// Because they should only be reading copies of the data that
		// is stored in Scoreboard State.
		go func(channel chan scoreboard.ServiceUpdate, hosts []scoreboard.Host, scoreboardConfig scoreboard.Config) {

			ilog.Println("Started the Ping Check Provider")

			for {
				// Sleep for the configured amount of time before trying to ping hosts
				time.Sleep(scoreboardConfig.TimeBetweenPingChecks)

				for _, host := range hosts {
					// Asyncronously ping hosts so we don't wait full timeouts and can ping faster.
					go host.PingHost(channel, scoreboardConfig.PingTimeout)
				}
			}
		}(updateChannel, sbd.Hosts, sbd.Config)
	}

	// Thread for querying services. Results are shipped to the
	// ScoreboardStateUpdater as ServiceUpdates
	// We don't read-lock these threads because they should only be handling
	// copies of the data that is in the Scoreboard State, not the actual data.
	go func(channel chan scoreboard.ServiceUpdate, hosts []scoreboard.Host, config scoreboard.Config) {

		ilog.Println("Started the Host Check Provider")

		for {
			// Wait the configured amount of time before initiating threads to query services.
			time.Sleep(config.TimeBetweenServiceChecks)

			for _, host := range hosts { // Check each host
				for _, service := range host.Services { // Check each service
					// Asyncronously check services so we can check a lot of them
					// and don't have to wait on service timeout durations
					// which might be lengthy.
					go service.CheckService(channel, host.Ip, config.ServiceTimeout)
				}
			}
		}
	}(updateChannel, sbd.Hosts, sbd.Config)

	// Start the scoreboardStateUpdater to update the scoreboard with
	// ServiceUpdates
	go func(updates chan scoreboard.ServiceUpdate) {

		ilog.Println("Started the Service State Updater")
		output := make(chan string)
		go sbd.StateUpdater(updateChannel, output)

		for {
			select {
			case message := <-output:
				dlog.Println(message)
			default:
				time.Sleep(1 * time.Second)
			}
		}

	}(updateChannel)

	// Register '/' with ScoreboardState
	http.Handle("/", &sbd)

	ilog.Println("Started Webserver")

	// Start the webserver and serve content
	http.ListenAndServe(":80", nil)
}

// This function tests privileges and initiates an unclean exit if the
// incorrect privileges are used to run the program.
func testPrivileges() {
	if usr, err := user.Current(); err == nil {

		// Attempt to identify the Administrator group
		if runtime.GOOS == "windows" && !strings.HasSuffix(usr.Gid, "-544") {
			fmt.Println("Please run as Administrator. " +
				"This program needs Administrator to open port 80 and do ICMP.")

			os.Exit(1)
		} else if usr.Gid != "0" && usr.Uid != "0" { // ID root
			if runtime.GOOS == "linux" {
				fmt.Println("Please run as root. " +
					"This program needs root to open port 80 and do ICMP.")
			} else { // Dunno bud
				fmt.Println("Please run with elevated privileges. " +
					"This program needs elevated privileges to open port 80 and do ICMP")
			}

			os.Exit(1)
		}
	}
}

// Usage function to show program usage when the -h flag is given.
func usage() {
	fmt.Println(`USAGE:
	Starting goscore can be as simple as running it without any arguments.
	You can enable debug mode by passing the -d flag and can speicify your
	own config file by using the -c flag. You can build a simple config
	example by passing the -buildcfg flag. Use the -h flag to print this 
	message

LICENSE:
	You can view your rights with this software in the LICENSE here: 
	https://github.com/AWildBeard/goscore/blob/master/LICENSE and
	can download the source here: https://github.com/AWildBeard/goscore

	By using this piece of software you agree to the terms as they are
	detailed in the LICENSE

	This software is distributed as Free and Open Source Software.

AUTHOR:
	This program was created by Michael Mitchell for the
	University of West Florida Cyber Security Club`)
}

// Utility function to translate a boolean flag to
// the string representation of yes for true and
// no for false
func boolToWord(flag bool) string {
	if flag {
		return "yes"
	} else {
		return "no"
	}
}
