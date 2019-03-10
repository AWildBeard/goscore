package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/sparrc/go-ping"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"path"
	"regexp"
	"runtime"
	"strings"
	"sync"
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

// Struct to represent the scoreboard's state
type ScoreboardState struct {
	// The hosts this scoreboard scores
	Hosts  []Host

	// Scoreboard specific config to dictate
	// how to check services
	config ScoreboardConfig

	// The RW lock that will allow updating the scoreboard
	// quickly without locking out web clients
	lock   sync.RWMutex
}

// This struct represents the current configuration for the scoreboard.
// Namely, the timeouts for checking host's services and ICMP.
type ScoreboardConfig struct {
	// Config option that represents whether the scoreboard should
	// ICMP testing for Hosts
	pingHosts bool

	// Config option that signifies the duration to wait before
	// trying to ping all the hosts in the configuration file.
	TimeBetweenPingChecks time.Duration

	// The duration to wait on hosts to respond to this programs
	// Ping requests
	PingTimeout time.Duration

	// The duration to wait before trying to check the services
	// as they are defined in the configuration file.
	TimeBetweenServiceChecks time.Duration

	// The duration to wait for all services to respond to
	// this program.
	ServiceTimeout time.Duration
}

// Struct to hold an update to a service held by ScoreboardState
type ServiceUpdate struct {
	// The IP of the machine who's service update this is for.
	// This is used as a unique identifier to identify machines.
	Ip string

	// If true, this ServiceUpdate contains data on an update to a service,
	// otherwise, this is a ping update
	ServiceUpdate bool

	// Flag to represent whether the Service is up, or if ServiceUpdate is
	// false, this flag represents if the Ping is up for the remote machine
	IsUp bool

	// This variable contains the index of the port that holds a service that is now up
	ServicePort string
}

// Struct to represent a Host that contains Hosts
type Host struct {
	// The service provided on the host
	Services []Service

	// The IP Address of a Host
	Ip string

	// A flag used to represent whether a Host is responding to ICMP
	PingUp bool
}

// An individual Service that is contained by a Host
type Service struct {
	// The name of the Service this struct represents
	Name string

	// The Port that the Service is hosted on
	Port string

	// The String to write to the remote Service.
	// This is optional and can be an empty string
	SendString string

	// A Regular Expression that can match the expected
	// response fro the remote Service.
	RegexResponse string

	// The Layer 4 Protocol used to connect to the Service.
	// I.E. 'tcp' or 'udp'
	Protocol string

	// Boolean flag to represent whether the service is currently up
	IsUp bool
}

// Function to serve the index.html for the scoreboard.
func (sbd *ScoreboardState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Establish a read-only lock to the scoreboard to retrieve data,
	// then drop the lock after we have retrieved that data we need.
	sbd.lock.RLock()
	returnString, _ := json.MarshalIndent(sbd.Hosts,"", "  ")
	sbd.lock.RUnlock() // Drop the lock

	// Respond to the client
	fmt.Fprintf(w, string(returnString))
}

// Helper function to return a new scoreboard
func NewScoreboard() ScoreboardState {
	return ScoreboardState{
		make([]Host, 0),
		ScoreboardConfig {
			false,
			time.Duration(0),
			time.Duration(0),
			time.Duration(0),
			time.Duration(0),
		},
		sync.RWMutex{},
	}
}

// A struct to represent the parsed yaml config. This type is
// passed directly to yaml.v2 for parsing
type Config struct {
	Services []map[string]string
	Config   map[string]string
}

// An error that can arrise from parsing the config file and checking for
// specific required configuration fields.
type ConfigError string

// Converts ConfigError to a String
func (err ConfigError) Error() string {
	return string(err)
}

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
		ilog.Println("Critical configuration file error encountered:", err)
		ilog.Println("This might be because the Config file wasn't found. " +
			"If this was the problem; run this program again with the " +
			"-buildcfg flag or use the -c flag to specify your a different config!")
		os.Exit(1)

	}

	// Test privileges for ICMP and opening port 80. Exit uncleanly if incorrect privileges are used.
	testPrivileges()

    if scoreboard.config.pingHosts { // The ping option was set

    	// Thread for pinging hosts. Results are shipped to the
    	// ScoreboardStateUpdater as ServiceUpdate's.
    	// We don't read-lock these threads with the Scoreboard State
    	// Because they should only be reading copies of the data that is stored in
    	// Scoreboard State.
		go func (channel chan ServiceUpdate, services []Host, scoreboardConfig ScoreboardConfig) {

			ilog.Println("Started the Ping Check Provider")

			for {
				// Sleep for the configured amount of time before trying to ping hosts
				time.Sleep(scoreboardConfig.TimeBetweenPingChecks)

				for _, service := range services {
					// Asyncronously ping services so we don't wait full timeouts and can ping faster.
					go pingHost(channel, service.Ip, scoreboardConfig.PingTimeout)
				}
			}
		} (updateChannel, scoreboard.Hosts, scoreboard.config)
	}

	// Thread for querying services. Results are shipped to the
	// ScoreboardStateUpdater as ServiceUpdate's
	// We don't read-lock these threads because they should only be handling
	// copies of the data that is in the Scoreboard State, not the actual data.
    go func (channel chan ServiceUpdate, hosts []Host, config ScoreboardConfig) {

    	ilog.Println("Started the Host Check Provider")

    	for {
    		// Wait the configured amount of time before initiating threads to query services.
    		time.Sleep(config.TimeBetweenServiceChecks)

			for _, host := range hosts { // Check each host
				for i := range host.Services { // Check each service
					// Asyncronously check services so we can check a lot of them
					// and don't have to wait on service timeout durations
					// which might be lengthy.
					go checkService(channel, host.Ip, host.Services[i], config.ServiceTimeout)
				}
			}
		}
	} (updateChannel, scoreboard.Hosts, scoreboard.config)

    // Start the scoreboardStateUpdater to update the scoreboard with
    // ServiceUpdates
    go scoreboardStateUpdater(updateChannel, &scoreboard)

    // Register '/' with ScoreboardState
	http.Handle("/", &scoreboard)

	ilog.Println("Started Webserver")

    // Start the webserver and serve content
	http.ListenAndServe(":80", nil)
}

// This function checks a single service in the predefined
// manner contained in the Service struct.
func checkService(updateChannel chan ServiceUpdate, ip string, service Service, serviceTimeout time.Duration) {
	serviceUp := false

	byteBufferTemplate := make([]byte, 1024)
	if conn, err := net.DialTimeout(service.Protocol,
		fmt.Sprintf("%v:%v", ip, service.Port), serviceTimeout); err == nil {

		stringToSend := fmt.Sprint(service.SendString)
		regexToMatch := fmt.Sprint(service.RegexResponse)

		conn.SetDeadline(time.Now().Add(serviceTimeout))

		if len(stringToSend) > 0 {
			io.Copy(conn, strings.NewReader(stringToSend)) // Write what we need to write.
		}

		// No sense of even bothering to read the response if we aren't
		// going to do anything with it.
		if len(regexToMatch) > 0 {
			buffer := bytes.NewBuffer(byteBufferTemplate)
			io.Copy(buffer, conn) // Read the response
			serviceUp, _ = regexp.Match(regexToMatch, buffer.Bytes())
		} else {
			serviceUp = true
		}

		conn.Close()
	}

	// Write the service update
	updateChannel <- ServiceUpdate{
		ip,
		true,
		serviceUp,
		service.Port,
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
		hostToPing,
		false, // This is an ICMP update
		pingSuccess, // Whether the ping was successful
		"", // Set this to an empty string.
	}
}

// Thread to read service updates and write the updates to ScoreboardState. We do this so
// we don't have to give every status checking thread the ability to
// RW lock the ScoreboardState. This lets us test services without locking.
// This function read locks for determining if an update should be applied to the
// Scoreboard State. If an update needs to be applied, the function drops its read lock
// and establishes a write lock to update the data. The write lock is maintained for as long as there
// are service updates that need to be analyzed. If no write lock is established, the function maintains
// it's read lock as long as there are service updates that need to be analyzed.
//
// The end goal of this complex locking is to minimize the time spent holding a
// write lock. however, once this function has establish a write lock,
// don't drop it because it might need to be re-established nano-seconds later.
// This function read locks for safety reasons.
func scoreboardStateUpdater(updateChannel chan ServiceUpdate, sbd *ScoreboardState) {

	ilog.Println("Started the Scoreboard State Updater")

	// These two flags are mutually exclusive. One being set does not rely on the other
	// which is why we have two of them, instead of expressing their logic with a single flag.
	// This function will drop it's read lock when it's in a sleeping state,
	// and only establishes a read lock when needing to find data that might be
	// changed, and only then establishing a write lock **if** that data needs to be
	// changed. A write lock or a read lock is kept until there is no more
	// data to be parsed through.
	var (
		isWriteLocked = false // Flag to hold whether we already have a lock or not.
		isReadLocked = false // Flag to hold whether we have a read lock.
	)

	for {
		// A service update that we are waiting for
		var update ServiceUpdate

		// Test for there being another service update on the line
		select {
		case update = <- updateChannel: // There is another update on the line

			// Read-Lock to be safe.
			if !isWriteLocked && !isReadLocked {
				sbd.lock.RLock()
				isReadLocked = true
			}

			// Interate down to the Service or Host that needs to be updated
			for indexOfHosts, host := range sbd.Hosts {
				if update.Ip == sbd.Hosts[indexOfHosts].Ip {
					if update.ServiceUpdate { // Is the update a service update, or an ICMP update?
						// It's a service update so iterate down to the service that needs to be updated.
						for indexOfServices, service := range host.Services {
							if service.Port == update.ServicePort {
								// Decide if the update contradicts the current Scoreboard State.
								// If it does, we need to establish a Write lock before changing
								// the service state.
								if sbd.Hosts[indexOfHosts].Services[indexOfServices].IsUp != update.IsUp {
									if !isWriteLocked { // If we already have a RW lock, don't que another
										sbd.lock.RUnlock() // Unlock our Read lock before Write Locking
										isReadLocked = false
										sbd.lock.Lock() // WRITE LOCK
										isWriteLocked = true
									}

									// Debug print that we received a service update
									dlog.Printf("Received a service update from %v:%v. The status " +
										"is different, so updating the Scoreboard State. Status: %v", update.Ip,
										sbd.Hosts[indexOfHosts].Services[indexOfServices].Port,
										boolToWord(update.IsUp))

									// Update that services state
									sbd.Hosts[indexOfHosts].Services[indexOfServices].IsUp = update.IsUp
	 							} else {
									// Debug print that we received a service update
									dlog.Printf("Received a service update from %v:%v. The status " +
										"is not different, so not updating the Scoreboard State. Status: %v", update.Ip,
										sbd.Hosts[indexOfHosts].Services[indexOfServices].Port,
										boolToWord(update.IsUp))
								}
							}
						}
					} else {
						// We are dealing with an ICMP update. We need to determine if the
						// Scoreboard State needs to be updated.
						if sbd.Hosts[indexOfHosts].PingUp != update.IsUp { // We need to establish a write lock
							if !isWriteLocked { // If we already have a RW lock, don't que another
								sbd.lock.RUnlock()
								isReadLocked = false
								sbd.lock.Lock() // WRITE LOCK
								isWriteLocked = true
							}

							dlog.Printf("Received a ping update from %v. The status is different, " +
								"so updating the Scoreboard State. Status: %v", update.Ip,
								boolToWord(update.IsUp))

							sbd.Hosts[indexOfHosts].PingUp = update.IsUp
						} else {
							dlog.Printf("Received a ping update from %v. The status is not different," +
								"so not updating the Scoreboard State. Status: %v", update.Ip,
								boolToWord(update.IsUp))
						}
					}
				}
			}
		default: // There is not another update on the line, so we'll wait for one
			// If we have a write lock because we wrote an update,
			// unlock so clients can view content. otherwise, we had a read
			// lock that needs to be released because we don't need it any longer.
			if isWriteLocked {
				sbd.lock.Unlock()
				isWriteLocked = false
			} else if isReadLocked { // This isn't a else case because this default case might be ran quickly in succession
				sbd.lock.RUnlock()
				isReadLocked = false
			}

			// Wait 1 second, then check again!
			time.Sleep(1 * time.Second)
		}
	}

}

func parseConfigToScoreboard(config *Config, scoreboard *ScoreboardState) error {
	// Determine if the user has set the ping option in the config file.
	if config.Config["pingHosts"] != "yes" {
		scoreboard.config.pingHosts = false // Deactivates all the ping functionality of the program

	} else {
		scoreboard.config.pingHosts = true // Activates the ping functionality of the program

		// Determine the required pingInterval option from the config file
		if pingDuration, err := time.ParseDuration(config.Config["pingInterval"]) ; err == nil {
			scoreboard.config.TimeBetweenPingChecks = pingDuration

		} else { // The option was not found
			return ConfigError(fmt.Sprint("Failed to parse pingInterval from config file:", err))
		}

		// Determine the required pingTimeout option from the config file
		if ptimeout, err := time.ParseDuration(config.Config["pingTimeout"]) ; err == nil {
			scoreboard.config.PingTimeout = ptimeout

		} else { // The option was not found
			return ConfigError(fmt.Sprint("Failed to parse pingTimeout in config file:", err))
		}
	}

	// Determine the required serviceInterval option from the config file
	if serviceDuration, err := time.ParseDuration(config.Config["serviceInterval"]) ; err == nil {
		scoreboard.config.TimeBetweenServiceChecks = serviceDuration

	} else { // The option was not found
		return ConfigError(fmt.Sprint("Failed to parse serviceInterval from config file:", err))
	}

	if stimeout, err := time.ParseDuration(config.Config["serviceTimeout"]) ; err == nil {
		scoreboard.config.ServiceTimeout = stimeout

	} else {
		return ConfigError(fmt.Sprint("Failed to parse serviceTimeout from config file:", err))
	}

	// Check that at least one service is defined in the config file
	if len(config.Services) < 1 {
		return ConfigError("There must be at least one service defined in the config file!")
	}

	// Create the services for the ScoreboardState
	for _, mp := range config.Services {
		newHostsIp := mp["ip"] // Use IP as a unique identifier for machines
		createNewHost := true

		// This is the service that we need to add to the scoreboard,
		// the only question left is if the host that this service belongs to
		// is already created or if we need to create it.
		newService := Service{
			mp["service"],
			mp["port"],
			mp["send_string"],
			mp["response_regex"],
			mp["connection_protocol"],
			true,
		}

		// Search for a host that contains the current services IP, if it's not found
		// We'll create a new Host with the newService already further added below.
		for i := range scoreboard.Hosts {

			// A host containing this IP already exists, so we just need add our
			// newService to it :D
			if newHostsIp == scoreboard.Hosts[i].Ip { // There are more services to test from this machine
				createNewHost = false
				scoreboard.Hosts[i].Services = append(scoreboard.Hosts[i].Services, newService)
				break
			}
		}

		// Test the regex for services and error if the regex parser can't handle it.
		if _, err := regexp.Match(mp["response_regex"], []byte("")) ; err != nil {
			return ConfigError(fmt.Sprintf("invalid regular expression: %v \nFor more " +
				"information see https://github.com/google/re2/wiki/Syntax", err))
		}

		// If the above loop got this far, then we are looking at a Host we haven't seen yet,
		// so add it here and it's Service
		if createNewHost {
			scoreboard.Hosts = append(scoreboard.Hosts, Host{
				[]Service{newService},
				newHostsIp,
				true,
			})
		}
	}

	return nil
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

// Utility function to translate a boolean flag to
// the string representation of yes for true and
// no for false
func boolToWord(flag bool) string {
	if flag { return "yes" } else { return "no" }
}

