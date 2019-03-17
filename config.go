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
	"fmt"
	"github.com/AWildBeard/goscore/scoreboard"
	"gopkg.in/yaml.v2"
	"os"
	"time"
)

// A struct to represent the parsed yaml config. This type is
// passed directly to yaml.v2 for parsing the physical
// config file into active memory used to create ServiceState.Config
type Config struct {
	Hosts  []scoreboard.Host `yaml:"hosts"`
	Config map[string]string
}

// An error that can arrise from parsing the config file and checking for
// specific required configuration fields.
type ConfigError string

// Converts ConfigError to a String.
// Implements Error for ConfigError
func (err ConfigError) Error() string {
	return string(err)
}

// This function simple Opens the config.yaml file and parses it
// into the Config type, then returns that type.
func initConfig() (Config, error) {
	var (
		configFile *os.File
		config     Config
	)

	// Test each config file option.
	if f, err := os.Open(defaultConfigFileLocation); err == nil {
		configFile = f
	} else if f, err := os.Open(defaultConfigFileName); err == nil {
		configFile = f
	} else {
		return config, err
	}

	defer configFile.Close()

	dlog.Println("Opened config:", configFile.Name())

	// Attempt to decode the config into a go type
	yamlDecoder := yaml.NewDecoder(configFile)
	if err := yamlDecoder.Decode(&config); err == nil {
		return config, nil
	} else {
		return config, err
	}
}

func (config *Config) ValidateConfig() error {
	// Test for pingHosts
	if len(config.Config["pingHosts"]) == 0 {
		return ConfigError("You must include the 'pingHosts:' field under 'config:'")
	} else if config.Config["pingHosts"] == "yes" { // It's there so test pingHost related fields
		if len(config.Config["pingInterval"]) == 0 {
			return ConfigError("You must define the 'pingInterval:' field under 'config:'")
		}

		if len(config.Config["pingTimeout"]) == 0 {
			return ConfigError("You must define the 'pingTimeout:' field under 'config:'")
		}
	}

	// Test Service fields
	if len(config.Config["serviceInterval"]) == 0 {
		return ConfigError("You must define the 'serviceInterval:' field under 'config:'")
	}

	if len(config.Config["serviceTimeout"]) == 0 {
		return ConfigError("You must define the 'serviceTimeout:' field under 'config:'")
	}

	// Check that at least one service is defined in the config file
	if len(config.Hosts) < 1 {
		return ConfigError("There must be at least one service defined in the config file!")
	}

	// Test for the required fields for Hosts and Services
	for _, host := range config.Hosts {
		if len(host.Name) == 0 {
			return ConfigError("You must define the name of the host in the host: field under hosts:")
		}

		if len(host.Ip) == 0 {
			return ConfigError(fmt.Sprintf("You must define the IP field for %v "+
				"in the ip: field.", host.Name))
		}

		if len(host.Services) == 0 {
			return ConfigError(fmt.Sprintf("You must define at least one "+
				"Service for %v under the services: field", host.Name))
		}

		for _, service := range host.Services {
			if len(service.Name) == 0 {
				return ConfigError(fmt.Sprintf("You must define the name of the "+
					"service for %v under the service: field", host.Name))
			}

			if len(service.Protocol) == 0 {
				return ConfigError(fmt.Sprintf("You must define the protocol "+
					"to use to test %v on %v", service.Name, host.Name))
			}

			if service.Protocol != "host-command" && len(service.Port) == 0 {
				return ConfigError(fmt.Sprintf("You must define the port to "+
					"connet to to test %v on %v", service.Name, host.Name))
			}

			if service.Protocol == "host-command" && (len(service.Command) == 0 || len(service.Response) == 0) {
				return ConfigError(fmt.Sprintf("You must speicify a command and a response to "+
					"run to test %v on %v in host-command mode", service.Name, host.Name))
			}
		}
	}

	return nil
}

// This function converts the raw Config type to ScoreboardState.Config
func ParseConfigToScoreboard(config *Config, scoreboard *scoreboard.State) error {
	// Determine if the user has set the ping option in the config file.
	if config.Config["pingHosts"] != "yes" {
		scoreboard.Config.PingHosts = false // Deactivates all the ping functionality of the program

	} else {
		scoreboard.Config.PingHosts = true // Activates the ping functionality of the program

		// Determine the required pingInterval option from the config file
		if pingDuration, err := time.ParseDuration(config.Config["pingInterval"]); err == nil {
			scoreboard.Config.TimeBetweenPingChecks = pingDuration

		} else { // The option was not found
			return ConfigError(fmt.Sprint("Failed to parse pingInterval from config file:", err))
		}

		// Determine the required pingTimeout option from the config file
		if ptimeout, err := time.ParseDuration(config.Config["pingTimeout"]); err == nil {
			scoreboard.Config.PingTimeout = ptimeout

		} else { // The option was not found
			return ConfigError(fmt.Sprint("Failed to parse pingTimeout in config file:", err))
		}
	}

	// Determine the required serviceInterval option from the config file
	if serviceDuration, err := time.ParseDuration(config.Config["serviceInterval"]); err == nil {
		scoreboard.Config.TimeBetweenServiceChecks = serviceDuration

	} else { // The option was not found
		return ConfigError(fmt.Sprint("Failed to parse serviceInterval from config file:", err))
	}

	// Check for ServiceTimeout
	if stimeout, err := time.ParseDuration(config.Config["serviceTimeout"]); err == nil {
		scoreboard.Config.ServiceTimeout = stimeout

	} else {
		return ConfigError(fmt.Sprint("Failed to parse serviceTimeout from config file:", err))
	}

	if configDefaultServiceState := config.Config["defaultState"] ; configDefaultServiceState != "" {
		if configDefaultServiceState == "up" {
			scoreboard.Config.DefaultServiceState = true
		} else {
			scoreboard.Config.DefaultServiceState = false
		}
	} else {
		return ConfigError(fmt.Sprint("Failed to parse defaultState from 'config:' section!"))
	}

	if configCompetitionName := config.Config["competitionName"] ; configCompetitionName != "" {
		scoreboard.Name = configCompetitionName
	} else {
		return ConfigError(fmt.Sprint("Failed to parse competitionName from 'config:' section!"))
	}

	scoreboard.Hosts = config.Hosts

	return nil
}
