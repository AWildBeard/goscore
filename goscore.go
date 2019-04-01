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
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"path"
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

	// Flags
	flag.StringVar(&defaultConfigFileLocation, "c", defaultConfigFileLocation,
		"Specify a custom config file location")
	flag.BoolVar(&debug, "d", false, "Print debug messages")
	flag.BoolVar(&buildCfg, "buildcfg", false, "Output an example configuration file "+
		"to "+cwd+"/config.yaml")

	// Set a custom command line usage
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
	} else {
		// Create a new scoreboard
		sbd := NewScoreboard()

		// TODO: Rework config mockup?

		// Read and parse the config file
		if config, err := initConfig(); err == nil { // Initialize the config
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
			switch err.(type) {
			case *os.PathError:
				err := *err.(*os.PathError)

				ilog.Println("Failed to open config")
				if err.Op == "open" {
					ilog.Println("Run this program again with the -buildcfg flag to generate a " +
						"config to your current working directory, or use the -c flag to specify a " +
						"config somewhere else.")
				} else {
					ilog.Println("Unknown error encountered when trying to open config file:", err)
				}
			case *yaml.TypeError:
				ilog.Println("Failed to decode config file:", err)
			default:
				ilog.Println("Encountered unexpected error:", err)
			}

			os.Exit(1)
		}

		// Start the competition!
		sbd.Start()
	}
}

// Usage function to show program usage when the -h flag is given.
func usage() {
	fmt.Println(`SYNOPSIS:
	Goscore is designed to offer a simple scoreboard solution for
	cyber security competitions and comes ready to be deployed for a
	competition. It allows specifying services to test in a config 
	file, the interval by which to test them on, and the method by 
	which to test them; including host level commands that can be 
	run and evaluated to determine the services state or by 
	manually passing a connection string to the remote services port.
	This program also offers a built in HTML scoreboard with the
	option use your own HTML scoreboard.

	If you are looking for config file help, additional info about
	this program, or are looking for help on creating your own HTML
	scoreboard; see https://github.com/AWildBeard/goscore/wiki

OPTIONS:
	-buildcfg
		This flag will cause the program to write a working config file
		to your current working directory an exit. Use this to generate
		a config template that you can modify to suite your own needs.

	-c [config file]
		This flag allows a user to specify a custom config file location. 
		By default, this program checks for the config file in the 
		directory where this program is run (your current working 
		directory), or the directory where this program is stored.

	-d 
		This flag enables debug output to STDERR

	-h
		This flag will display this message and exit.

LICENSE:
	You can view your rights with this software in the LICENSE here: 
	https://github.com/AWildBeard/goscore/blob/master/LICENSE and
	can download the source code for this program here: 
	https://github.com/AWildBeard/goscore

	By using this piece of software you agree to the terms as they are
	detailed in the LICENSE

	This software is distributed as Free and Open Source Software.

AUTHOR:
	This program was created by Michael Mitchell for the
	University of West Florida Cyber Security Club and includes
	libraries and software written by Canonical, and Cameron Sparr`)
}
