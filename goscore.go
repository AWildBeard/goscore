package main

import (
	"flag"
	"fmt"
	"github.com/sparrc/go-ping"
	"gopkg.in/yaml.v2"
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

type Config struct {
    Services []map[string]string
    Config   map[string]string
}

var (
    // Variables
    defaultConfigFile = "config.yaml"
    configFileString string
    debug bool

    // Logger
    ilog *log.Logger
    dlog *log.Logger
)

func init() {
	// Test privileges for ICMP and opening port 80. Exit uncleanly if incorrect privileges are used.
	testPrivileges()

	execPath, _ := os.Executable()
	configFileString = fmt.Sprintf("%v/%v", path.Dir(execPath), defaultConfigFile)

	flag.StringVar(&configFileString, "c", configFileString,
	    "Specify a custom config file location. Default: `config.yaml`")
	flag.BoolVar(&debug, "d", false, "Print debug messages")
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

	var (
		// Configuration options for how to run the threads.
		pingHosts bool
		pingTimeout time.Duration
		timeBetweenPings time.Duration
		timeBetweenServices time.Duration

		// Create a new scoreboard
		scoreboard = NewScoreboard()

		// Make a buffered channel to write service updates over. These updates will get read by a thread
		// that will write lock ScoreboardState
		updateChannel = make(chan ServiceUpdate, 10)
	)

	// Read config file
    if config, err := initConfig() ; err == nil {
    	dlog.Println("Parsed config file")

    	// Determine if the user has set the ping option in the config file.
		if config.Config["pingHosts"] != "yes" {
			pingHosts = false // Deactivates all the ping functionalility of the program
		} else {
			dlog.Println("Ping hosts option enabled.")
			pingHosts = true // Activates the ping functionalility of the program

			// Determine the required pingInterval option from the config file
			if pingDuration, err := time.ParseDuration(config.Config["pingInterval"]) ; err == nil {
				timeBetweenPings = pingDuration
				dlog.Println("Time between ping hosts:", timeBetweenPings)
			} else { // The option was not found
				ilog.Println("Failed to parse pingInterval in config file:", err)
				os.Exit(1)
			}

			// Determine the required pingTimeout option from the config file
			if ptimeout, err := time.ParseDuration(config.Config["pingTimeout"]) ; err == nil {
				pingTimeout = ptimeout
				dlog.Println("Ping timeout:", pingTimeout)
			} else { // The option was not found
				ilog.Println("Failed to parse pingTimeout in config file:", err)
				os.Exit(1)
			}
		}

    	// Determine the required serviceInterval option from the config file
		if serviceDuration, err := time.ParseDuration(config.Config["serviceInterval"]) ; err == nil {
			timeBetweenServices = serviceDuration
			dlog.Println("Time between service pings:", timeBetweenServices)
		} else { // The option was not found
			ilog.Println("Failed to parse serviceInterval from config file:", err)
			os.Exit(1)
		}

		// Create the services for the ScoreboardState
		for _, mp := range config.Services {
			hostToAdd := mp["ip"] // Use IP as a unique identifier for machines
			shouldAdd := true

			for i := range scoreboard.Services {
				if hostToAdd == scoreboard.Services[i].Ip { // There are more services to test from this machine
					shouldAdd = false
					scoreboard.Services[i].Names = append(scoreboard.Services[i].Names, mp["service"])
					scoreboard.Services[i].Ports = append(scoreboard.Services[i].Ports, mp["port"])
					scoreboard.Services[i].ServicesUp = append(scoreboard.Services[i].ServicesUp, true)
					break
				} else { // This is the first occurence of this IP, so we should set this flag
					shouldAdd = true
				}
			}

			// If the above loop got this far, then we are looking at a service we haven't seen yet, so add it here
			if shouldAdd {
				scoreboard.Services = append(scoreboard.Services, Service{
					[]string{mp["service"]}, // Array of len 1 that holds the service name we haven't added yet
					[]string{mp["port"]}, // Array of 1 that holds the services port
					[]bool{true}, // Default to up
					mp["ip"], // Key to ID the machine that may have many services
					true, // Default to up
				})
			}
		}
	}

    if pingHosts { // The ping option was set
    	// Thread for pinging hosts. Results are shipped to the ScoreboardStateUpdater as ServiceUpdate's
		go func (c chan ServiceUpdate, services []Service) {
			ilog.Println("Started the Ping Check Provider")

			for {
				// Sleep for the configured amount of time before trying to ping hosts
				time.Sleep(timeBetweenPings)

				for _, service := range services {
					// Asyncronously ping services so we don't wait full timeouts and can ping faster.
					go func (channel chan ServiceUpdate, hostToPing string) {
						pingSuccess := false

						if pinger, err := ping.NewPinger(hostToPing); err == nil {
							pinger.Timeout = pingTimeout
							pinger.SetPrivileged(true)
							pinger.Count = 3
							pinger.Run() // Run the pinger

							stats := pinger.Statistics() // Get the statistics for the ping from the pinger

							pingSuccess = stats.PacketsRecv != 0 // Test if packets were received
						}

						channel <- ServiceUpdate {
							hostToPing, // Key to ID the machine
							false, // This is a ping update
							pingSuccess, // Wether the ping was successful
							-1, // Set this to a bad num so we don't run the risk of accidentily updating.
						}

					} (c, service.Ip)
				}
			}
		} (updateChannel, scoreboard.Services)
	}

	// Thread for querying services. Results are shipped to the ScoreboardStateUpdater as ServiceUpdate's
    go func (c chan ServiceUpdate, services []Service) {
    	ilog.Println("Started the Service Check Provider")
    	for {

    		// Wait the configured amount of time before initiating threads to query services.
    		time.Sleep(timeBetweenServices)

			for _, service := range services {
				for i := range service.Ports { // Services are defined by their port numbers.

					// For now just set all services as offline.
					updateChannel <- ServiceUpdate{
						service.Ip, // Key to ID the machine
						true, // This is a service update
						false, // Wether the service is up
						i, // The service to update
					}
				}
			}
		}
	} (updateChannel, scoreboard.Services)

	// Thread to read service updates and write the updates to ScoreboardState. We do this so
	// we don't have to give every thread the ability to RW lock the ScoreboardState. This lets us test services
	// without locking and only locks for updates to the ScoreboardState, as it should be :D. The RW lock is maintained
	// for as long as there are service updates to be written to ScoreboardState.
	go func (c chan ServiceUpdate, sbd *ScoreboardState) {
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
								sbd.Services[i].Ports[update.ServiceUpIndex], update.ServiceUp)

							sbd.Services[i].ServicesUp[update.ServiceUpIndex] = update.ServiceUp
						} else {
							dlog.Printf("Received a ping update from %v. Status: %v", update.Ip,
								update.ServiceUp)
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

	} (updateChannel, &scoreboard)

    // Register '/' with ScoreboardState
	http.Handle("/", &scoreboard)

	ilog.Println("Started Webserver")
    // Start the webserver and serve content
	http.ListenAndServe(":80", nil)
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