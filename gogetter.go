package main

import (
	"flag"
	"fmt"
	"github.com/sparrc/go-ping"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"runtime"
	"strings"
	"time"
)

type Config struct {
    Services []map[string]string
    Config   map[string]string
}

var (
    // Variables
    configFileString string
    debug bool

	config Config
	pingHosts bool
    pingTimeout time.Duration
	timeBetweenPings time.Duration
	timeBetweenServices time.Duration

    // Logger
    ilog *log.Logger
    dlog *log.Logger
)

func init() {
	// Test privileges for ICMP and opening port 80, then restart if
	// privileges are not sufficient on Linux. Uses pkexec to do so.
	// If not on linux, say that we need more privileges and stop.
	// On linux, this function blocks until the child process with priv's
	// exits. Then this program exits with the return code of the child
	testPrivileges()

	var (
		execPath, _ = os.Executable()
		configDefault = fmt.Sprintf("%v/config.yaml", path.Dir(execPath))
	)

	flag.StringVar(&configFileString, "c", configDefault,
	    "Specify a custom config file location. Default: `config.yaml`")
	flag.BoolVar(&debug, "d", false, "Print debug messages")
}

func main() {
	// Read command line flags
    flag.Parse()

    // Initialize logging devices
	ilog = log.New(os.Stdout, "", 0)

    if debug {
        dlog = log.New(os.Stderr, "DBG: ", log.Ltime)
    } else {
        dlog = log.New(ioutil.Discard, "", 0)
    }

	// Catch interrupt signals to handle shutting down threads and restarting for privileges
	// TODO: Finish ^
	var interruptSignals = make(chan os.Signal, 1)
	signal.Notify(interruptSignals, os.Interrupt, os.Kill)

	// Read config file
    initConfig()

	// Do the network stuff that this program was made to do.
    if pingHosts {
    	// Thread for pinging hosts
		go func () {
			ilog.Println("Started ping provider")

			// Identify hosts to ping
			hosts := make([]string, 0) // 0 - TeHe!

			// For loop to search the hosts for duplicates. If a duplicate is found, don't add the new host.
			// Else add the new host :D
			for _, mp := range config.Services {
				hostToAdd := mp["ip"]
				shouldAdd := true
				for _, host := range hosts {
					if hostToAdd == host {
						shouldAdd = false
						break
					} else {
						shouldAdd = true
					}
				}

				if shouldAdd {
					hosts = append(hosts, hostToAdd)
				}
			}

			for {
				time.Sleep(timeBetweenPings)
				for _, host := range hosts {
					ilog.Printf("Pinging %v\n", host)
					go func (hostToPing string) {
						if pinger, err := ping.NewPinger(hostToPing); err == nil {
							pinger.Timeout = pingTimeout
							pinger.SetPrivileged(true)
							pinger.Count = 3
							pinger.Run()
							stats := pinger.Statistics()
							if stats.PacketsRecv != 0 {
								// Set a flag that says this service is up
								ilog.Println("Successfully pinged", hostToPing)
								return
							}
						}
						// Set a flag that says this service is down
						ilog.Println("Failed to ping host", hostToPing)
					} (host)
				}
			}
		} ()
	}

	// Thread for querying services
    go func () {
    	ilog.Println("Started service provider")
    	for {
    		time.Sleep(timeBetweenServices)
			for _, mp := range config.Services {
				ilog.Printf("Checking service %v at %v:%v\n", mp["service"], mp["ip"], mp["port"])
			}
		}
	} ()

    // Pop a sigint off when it arrives. Otherwise we wait :D
	<- interruptSignals
	ilog.Println()
    dlog.Print("Exiting!")
}

func initConfig() {
	var (
		configFile *os.File
	)

	if f, err := os.Open(configFileString); err == nil {
		dlog.Println("Opened config file:", configFileString)
		configFile = f
	} else {
		ilog.Println("Failed to open config file:", configFileString)
		os.Exit(1)
	}

	yamlDecoder := yaml.NewDecoder(configFile)
	if err := yamlDecoder.Decode(&config) ; err == nil {
		dlog.Println("Decoded config file")
		if config.Config["pingHosts"] != "yes" {
			pingHosts = false
		} else {
			dlog.Println("Ping hosts option enabled.")
			pingHosts = true
			if pingDuration, err := time.ParseDuration(config.Config["pingInterval"]) ; err == nil {
				timeBetweenPings = pingDuration
				dlog.Println("Time between ping hosts:", timeBetweenPings)
			} else {
				dlog.Println("Failed to parse pingInterval in config file:", err)
				ilog.Println("Failed to parse config file.")
				os.Exit(1)
			}

			if ptimeout, err := time.ParseDuration(config.Config["pingTimeout"]) ; err == nil {
				pingTimeout = ptimeout
				dlog.Println("Ping timeout:", pingTimeout)
			} else {
				dlog.Println("Failed to parse pingTimeout in config file:", err)
				ilog.Println("Failed to parse config file.")
				os.Exit(1)
			}
		}

		if serviceDuration, err := time.ParseDuration(config.Config["serviceInterval"]) ; err == nil {
			timeBetweenServices = serviceDuration
			dlog.Println("Time between service pings:", timeBetweenServices)
		} else {
			dlog.Println("Failed to parse serviceInterval from config file:", err)
			ilog.Println("Failed to parse config file.")
			os.Exit(1)
		}
	} else {
		dlog.Println("Failed to decode config file:", err)
		os.Exit(1)
	}

}

// This function tests privileges and implements a way to restart the program with privileges on Linux. Windows
// and others do not have a way to escalate privileges at this time.
func testPrivileges() {
	if usr, err := user.Current(); err == nil {
		if runtime.GOOS == "windows" && ! strings.HasSuffix(usr.Gid, "-544") {
			fmt.Println("Please run me as Administrator. I need Administrator to open port 80 and do ICMP.")
			os.Exit(1)
		} else if usr.Gid != "0" && usr.Uid != "0" {
			if runtime.GOOS == "linux" {
				fmt.Println("I need root privileges. I'm going to try to restart with policy kit.")

				offsetForAdditionalArgs := 3
				totalLen := (len(os.Args) - 1) + offsetForAdditionalArgs
				fullArgs := make([]string, totalLen)
				fullArgs[0] = "--user"
				fullArgs[1] = "root"
				fullArgs[2], _ = os.Executable()

				for i := range os.Args[1:] {
					fullArgs[offsetForAdditionalArgs+i] = os.Args[i+1]
				}

				cmd := exec.Command("pkexec", fullArgs...)
				cmd.Stdout = os.Stdout
				cmd.Stdin = os.Stdin
				cmd.Stderr = os.Stderr

				// Start child
				if err := cmd.Start(); err != nil {
					fmt.Println("Failed to restart for privilege escalation:", err)
					os.Exit(1)
				}

				// Wait for the child to exit, then follow suit
				if pstate, err := cmd.Process.Wait() ; err == nil {
					os.Exit(pstate.ExitCode())
				}

				fmt.Println("Exiting!")
				os.Exit(1)
			} else {
				fmt.Println("Please run with elevated privileges")
				os.Exit(1)
			}
		}
	}
}