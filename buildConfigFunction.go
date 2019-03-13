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

// This function is kept in it's own file to cut down on having to
// look for things around it.

import (
	"io"
	"os"
	"strings"
)

func buildConfig() {
	config := `###################################
### Required fields for 'hosts:'
# The indentation of the fields denotes which parent field
# those fields belong to. Indentation in this config 
# is mandatory. Below you will find the descriptions
# and definitions for both required and optional fields
# to define hosts and services. Fields that are mandatory
# will say that they are mandatory in their description.
#
# Look below this definition section for examples
#
# host: 
#       - The name of the host you are testing. This will be
#         shown in the Web UI as an identifier. This is a
#         mandatory field so you must define at least one
#         of them in this configuration file
#
#   ip:
#       - This is a member variable to 'host:' that defines the
#         the IP address of the host. This is a mandatory field.
#
#   services:
#       - This defines the services hosted on the host. This is
#         a mandatory field.
#
#     service:
#       - This is the name of the service. You must define this
#         because this will show up in the scoreboard. You must
#         define at least one of these under 'services:'.
#
#     port:    
#       - The port that the service runs on. This is a
#         mandatory field if the 'protocol:' field
#         is set to 'tcp' or 'udp'.
#
#     protocol:
#       - The protocol for connecting to the service.
#         Either 'tcp', 'udp', or 'host-command'. For a 
#         definition of what 'host-command' is, see 
#         the 'command:' field below. This is a mandatory
#         field.
#
#     command:
#       - If the 'protocol:' field is defined as 'tcp' or 'udp'
#         this field denotes the literal string to send to the
#         remote socket once the connection has been established.
#         This is used if you would like to connect to a custom
#         service or if you don't wan't to run a host-command.
#
#         If the 'protocol:' field is defined as 'host-command'
#         then this field denotes the command to run on the host.
#
#         This is an optional field if the 'protocol:' field is
#         'tcp' or 'udp'. In these cases, omitting this field
#         will not send traffic to the remote service.
#
#         If 'protocol:' is 'host-command', then this field is
#         a mandatory field.
#
#     response:
#       - This fields denotes a string that is expected in the
#         response of the 'command:' field. In the case of 
#         the 'protocol:' field being 'tcp' or 'udp', the 
#         'response:' field is matched on what is returned
#         from the remote service.
#
#         In the case that 'protocol:' is 'host-command',
#         the stdout and stderr of the 'command:' is matched
#         to 'response:'
#
#         In both cases, if a match is found from 'response:',
#         the the service is marked as online
#
#         This is an optional field when 'protocol:' is
#         'tcp' or 'udp'. When 'protocol:' is 'tcp' or 'udp',
#         omitting this field will just cause the program to 
#         check if the remote port is open. 'udp' 
#         protocols in this case will most likely always be 
#         marked as online because of the nature of UDP.
#
#         In the case where 'protocol:' is 'host-command',
#         this is a mandatory field to eliminate the ambiguity
#         of determining if the service is online.
#
###
###################################

# You must define the hosts section
hosts:

  # A single 'host:' must be defined

  ## Open Port check example ##
  # This example shows the required fields
  # for a 'tcp' service
  - host: "Debian MySQL" # Host name is required
    ip: "172.20.240.20"  # IP address is required
    services:            # Required 
      - service: "MySQL" # Service name is required
        port: "3306"     # In 'tcp' mode, port is required
        protocol: "tcp"  # Required

  ## Multiple service example ##
  - host: "Fedora mail server" # Required
    ip: "172.20.241.40"        # Required 
    services:                  # Required 

      ## login service example ## 
      - service: "imap"        # Service name is required
        port: "143"            # in 'tcp' mode, 'port:' is required
        protocol: "tcp"        # Required
        # Send a string to the service
        command: "a0001 LOGIN \"sysadmin\" \"password\""
        # Test it's response
        response: "OK"

      # Optionally, you may define more than one service on a host.
      - service: "smtp"        # Service name is required
        port: "25"             # in 'tcp' mode, 'port:' is required
        protocol: "tcp"        # Required
        # Only test the response from the service (banner grab)
        response: "220"

  ## Manual HTTP example ##
  - host: "CentOS web server" # Required
    ip: "172.20.241.30"       # Required
    services:                 # Required
      - service: "http"       # Required
        port: "80"            # in 'tcp' mode, 'port:' is required
        protocol: "tcp"       # Required
        # Send a string to the service
        command: "GET / HTTP/1.0 \r\n\r\n"
        # Match it's response
        response: "200 OK"

  ## DNS example ##
  - host: "Ubuntu dns"            # Required
    ip: "172.20.242.10"           # Required
    services:                     # Required
      - service: "dns"            # Required
        protocol: "host-command"  # Required
        # Run the below command.
        # 'command:' is required when 
        # 'protocol:' is 'host-command'
        command: "dig www.google.com @172.20.242.10"
        # Match the response from the command
        # in either stderr or stdout
        response: "ANSWER: 1"     # Required in this mode

  ## HTTP example using a command  ##
  ## that outputs info over STDERR ##
  - host: "Splunk"                    # Required
    ip: "172.20.241.20"               # Required
    services:                         # Required
      - service: "http"               # Required
        protocol: "host-command"      # Required
        command: "wget 172.20.241.20" # Required in this mode
        response: "200 OK"            # Required in this mode

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
  pingHosts: "yes" # whether to ping hosts or not
  pingInterval: "60s" # time between pings
  pingTimeout: "5s" # time to wait for a response ping from host
  serviceInterval: "120s" # time between checking services
  serviceTimeout: "10s" # time to wait for a service to respond and finish its connection

`
	if wd, err := os.Getwd(); err == nil {
		if file, err := os.OpenFile(wd+"/"+defaultConfigFileName, os.O_CREATE|os.O_WRONLY, 0666); err == nil {
			io.Copy(file, strings.NewReader(config))
		}
	}
}
