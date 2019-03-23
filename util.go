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
	"os"
	"os/user"
	"runtime"
	"strings"
	"time"
)

// Utility function to translate a boolean flag to
// the string representation of yes for true and
// no for false
func boolToWord(flag bool) string {
	if flag {
		return "yes"
	}

	return "no"
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

// Simple function to format a time.Duration into a string
func fmtDuration(duration time.Duration) string {
	var (
		hours   time.Duration
		minutes time.Duration
		seconds time.Duration
		builder strings.Builder
	)

	duration = duration.Round(time.Second)

	if duration >= time.Hour {
		hours = duration / time.Hour
		duration -= hours * time.Hour
		builder.WriteString(fmt.Sprintf("%dh", hours))
	}

	if duration >= time.Minute {
		minutes = duration / time.Minute
		duration -= minutes * time.Minute
		builder.WriteString(fmt.Sprintf("%dm", minutes))
	}

	seconds = duration / time.Second
	builder.WriteString(fmt.Sprintf("%ds", seconds))

	return builder.String()
}
