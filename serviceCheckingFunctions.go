package main

import (
	"bytes"
	"fmt"
	"github.com/sparrc/go-ping"
	"io"
	"net"
	"regexp"
	"strings"
	"time"
)

func checkService(updateChannel chan ServiceUpdate, service Service, serviceTimeout time.Duration) {
	for i := range service.Ports { // Services are defined by their port numbers.
		serviceUp := false

		byteBufferTemplate := make([]byte, 1024)
		if conn, err := net.DialTimeout(service.Protocols[i],
			fmt.Sprintf("%v:%v", service.Ip, service.Ports[i]), serviceTimeout); err == nil {

			stringToSend := fmt.Sprint(service.SendStrings[i])

			conn.SetDeadline(time.Now().Add(serviceTimeout))
			buffer := bytes.NewBuffer(byteBufferTemplate)

			if len(stringToSend) > 0 {
				io.Copy(conn, strings.NewReader(stringToSend)) // Write what we need to write.
			}

			io.Copy(buffer, conn)                                                // Read the response

			serviceUp, _ = regexp.Match(service.ResponseRegexs[i], buffer.Bytes())

			conn.Close()
		}

		// For now just set all services as offline.
		updateChannel <- ServiceUpdate{
			service.Ip, // Key to ID the machine
			true,       // This is a service update
			serviceUp,  // Wether the service is up
			i,          // The service to update
		}

		serviceUp = false
	}
}

func pingHost(updateChannel chan ServiceUpdate, hostToPing string, pingTimeout time.Duration) {
	pingSuccess := false

	if pinger, err := ping.NewPinger(hostToPing); err == nil {
		pinger.Timeout = pingTimeout
		pinger.SetPrivileged(true)
		pinger.Count = 3
		pinger.Run() // Run the pinger

		stats := pinger.Statistics() // Get the statistics for the ping from the pinger

		pingSuccess = stats.PacketsRecv != 0 // Test if packets were received
	}

	updateChannel <- ServiceUpdate {
		hostToPing, // Key to ID the machine
		false, // This is a ping update
		pingSuccess, // Wether the ping was successful
		-1, // Set this to a bad num so we don't run the risk of accidentily updating.
	}

}
