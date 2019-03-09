package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"os"
	"os/user"
	"runtime"
	"strings"
)

type Config struct {
	Services []map[string]string
	Config   map[string]string
}

// This function simple Opens the config.yaml file and parses it into a go type, then returns that type.
func initConfig() (Config, error) {
	var (
		configFile *os.File
		config Config
	)

	// Test each config file option.
	if f, err := os.Open(configFileString); err == nil {
		configFile = f
	} else if f, err := os.Open(defaultConfigFile) ; err == nil{
		configFile = f
	} else {
		return config, err
	}

	defer configFile.Close()

	dlog.Println("Opened config:", configFile.Name())

	// Attempt to decode the config into a go type
	yamlDecoder := yaml.NewDecoder(configFile)
	if err := yamlDecoder.Decode(&config) ; err == nil {
		return config, nil
	} else {
		return config, err
	}

}

// This function tests privileges and initiates an unclean exit if the
// incorrect privileges are used to run the program.
func testPrivileges() {
	if usr, err := user.Current(); err == nil {

		// Attempt to identify the Administrator group
		if runtime.GOOS == "windows" && ! strings.HasSuffix(usr.Gid, "-544") {
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

func boolToWord(flag bool) string {
	if flag { return "yes" } else { return "no" }
}

func buildConfig() {
	config := `###################################
### Required fields for 'services:'
# service: 
#       - The name of the service you are testing. This will be shown in the Web UI as an identifier
#
# port:    
#       - The port that the service runs on.
#
# ip:      
#       - The IP of the machine where this service is running
#
# connection_protocol: 
#       - The protocol for connecting to the service. Either 'tcp' or 'udp'.
#
# send_string:
#       - The string to send to the remote service, prior to testing it's response.
#       - This can be used to test services, do logins, etc before testing a response.
#
# response_regex:
#       - A regular expression that matches the reponse we are expecting from the server.
#       - An empty string will match everything. '200 OK' would match the OK return code 
#       - from an HTTP server. 
###
###################################

services:
    # This service actually demonstrates a login scenario
  - service: "mail_serv_imap"
    port: "143"
    ip: "172.20.241.40"
    connection_protocol: "tcp"
    send_string: "a0001 LOGIN \"sysadmin\" \"password\""
    response_regex: "OK"

    # In this service example, this program will just read the header from the service
  - service: "mail_serv_smtp"
    port: "25"
    ip: "172.20.241.40"
    connection_protocol: "tcp"
    send_string: ""
    response_regex: "250"

    # Simple http service example.
  - service: "webserver_http"
    port: "80"
    ip: "172.20.241.30"
    connection_protocol: "tcp"
    send_string: "GET / HTTP/1.0 \r\n\r\n"
    response_regex: "200 OK"

#################################
### Required fields for 'config:'
# pingHosts:
#       - Either 'yes' or 'no'. If set to 'yes', every service defined in the 'service:' section
#       - will have it's host pinged for better metric gathering.
#
# pingInterval:
#       - The interval between pinging hosts. The argument for this option can be in the form of 
#       - any numerical value that has a suffix such as 's', 'm', 'h', and more. 's' stands for seconds
#       - 'm' stands for minutes, and 'h' stands four hours. If the argument was '60s', every host would be
#       - pinged every 60 seconds to determine if it is still online. '3m' would mean that every host will
#       - be pinged every 3 minutes.
#
# pingTimeout:
#       - The duration to wait for the remote host to respond to one of our pings
#
# serviceInterval:
#       - The same as pingInterval above but for services.
#
# serviceTimeout:
#       - The same as pingTimeout above but for services.
###
#################################

config:
  pingHosts: "yes" # wheter to ping hosts or not
  pingInterval: "60s" # time between pings
  pingTimeout: "5s" # time to wait for a response ping from host
  serviceInterval: "120s" # time between checking services
  serviceTimeout: "10s" # time to wait for a service to respond and finish its connection

`
	if wd, err := os.Getwd() ; err == nil {
		if file, err := os.OpenFile(wd + "/" + defaultConfigFile, os.O_CREATE | os.O_WRONLY, 0666) ; err == nil {
			io.Copy(file, strings.NewReader(config))
		}
	}
}
