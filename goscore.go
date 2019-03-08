package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/sparrc/go-ping"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Config struct {
    Services []map[string]string
    Config   map[string]string
}

type Service struct {
	Names []string
	Ports []string
	ServicesUp []bool
	Ip string
	PingUp bool
}

type ScoreboardState struct {
	Services []Service
	lock sync.RWMutex
}

func (sbd *ScoreboardState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sbd.lock.RLock()
	returnString, _ := json.MarshalIndent(sbd.Services,"", "  ")
	sbd.lock.RUnlock()

	fmt.Fprintf(w, string(returnString))
}

func NewScoreboard() ScoreboardState {
	return ScoreboardState{
		make([]Service, 0),
		sync.RWMutex{},
	}
}

type ServiceUpdate struct {
	Ip string
	ServiceUpdate bool // If true, this ServiceUpdate contains data on an update to a service, otherwise, this is a ping update
	ServiceUp bool // bool to represent if the service at belows index is up.
	ServiceUpIndex int // This variable contains the index of the port that holds a service that is now up
}

var (
    // Variables
    configFileString string
    debug bool

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

	// Initialize debug output if relevant
    if debug {
        dlog = log.New(os.Stderr, "DBG: ", log.Ltime)
    } else {
        dlog = log.New(ioutil.Discard, "", 0)
    }

	var (
		pingHosts bool
		pingTimeout time.Duration
		timeBetweenPings time.Duration
		timeBetweenServices time.Duration
		scoreboard = NewScoreboard()

		// Make a buffered channel to write service updates over. These updates will get read by a thread
		// that will write lock ScoreboardState
		updateChannel = make(chan ServiceUpdate, 10)
	)

	// Read config file
    if config, err := initConfig() ; err == nil {
    	dlog.Println("Parsed config file")

		if config.Config["pingHosts"] != "yes" {
			pingHosts = false
		} else {
			dlog.Println("Ping hosts option enabled.")
			pingHosts = true
			if pingDuration, err := time.ParseDuration(config.Config["pingInterval"]) ; err == nil {
				timeBetweenPings = pingDuration
				dlog.Println("Time between ping hosts:", timeBetweenPings)
			} else {
				ilog.Println("Failed to parse pingInterval in config file:", err)
				os.Exit(1)
			}

			if ptimeout, err := time.ParseDuration(config.Config["pingTimeout"]) ; err == nil {
				pingTimeout = ptimeout
				dlog.Println("Ping timeout:", pingTimeout)
			} else {
				ilog.Println("Failed to parse pingTimeout in config file:", err)
				os.Exit(1)
			}
		}

		if serviceDuration, err := time.ParseDuration(config.Config["serviceInterval"]) ; err == nil {
			timeBetweenServices = serviceDuration
			dlog.Println("Time between service pings:", timeBetweenServices)
		} else {
			ilog.Println("Failed to parse serviceInterval from config file:", err)
			os.Exit(1)
		}

		for _, mp := range config.Services {
			hostToAdd := mp["ip"]
			shouldAdd := true

			for i := range scoreboard.Services {
				if hostToAdd == scoreboard.Services[i].Ip {
					shouldAdd = false
					scoreboard.Services[i].Names = append(scoreboard.Services[i].Names, mp["service"])
					scoreboard.Services[i].Ports = append(scoreboard.Services[i].Ports, mp["port"])
					scoreboard.Services[i].ServicesUp = append(scoreboard.Services[i].ServicesUp, true)
					break
				} else {
					shouldAdd = true
				}
			}

			if shouldAdd {
				scoreboard.Services = append(scoreboard.Services, Service{
					[]string{mp["service"]},
					[]string{mp["port"]},
					[]bool{true},
					mp["ip"],
					true,
				})
			}
		}
	}

	// Do the network stuff that this program was made to do.
    if pingHosts {
    	// Thread for pinging hosts
		go func (c chan ServiceUpdate, services []Service) {
			ilog.Println("Started ping provider")

			for {
				time.Sleep(timeBetweenPings)
				for _, service := range services {
					pingSuccess := false
					go func (channel chan ServiceUpdate, hostToPing string) {
						if pinger, err := ping.NewPinger(hostToPing); err == nil {
							pinger.Timeout = pingTimeout
							pinger.SetPrivileged(true)
							pinger.Count = 3
							pinger.Run()

							stats := pinger.Statistics()

							pingSuccess = stats.PacketsRecv != 0
						}

						channel <- ServiceUpdate {
							hostToPing,
							false,
							pingSuccess,
							-1,
						}

					} (c, service.Ip)
				}
			}
		} (updateChannel, scoreboard.Services)
	}

	// Thread for querying services
    go func (c chan ServiceUpdate, services []Service) {
    	ilog.Println("Started service provider")
    	for {
    		time.Sleep(timeBetweenServices)
			for _, service := range services {
				for i := range service.Ports {
					// For now just set all services as offline.
					updateChannel <- ServiceUpdate{
						service.Ip,
						true,
						false,
						i,
					}
				}
			}
		}
	} (updateChannel, scoreboard.Services)

	// Thread to read service updates and write the updates to ScoreboardState. We do this so
	// we don't have to give every thread the ability to RW lock. This lets us test services
	// without locking and only locks for updates to the UI, as it should be :D
	go func (c chan ServiceUpdate, sbd *ScoreboardState) {
		var (
			wasLocked = false
		)

		for {
			var update ServiceUpdate

			select {
			case update = <- updateChannel:
				if !wasLocked {
					sbd.lock.Lock()
					wasLocked = true
				}
				ilog.Println("Received service update for:", update.Ip)
				// Write the update
				for i := range sbd.Services {
					if update.Ip == sbd.Services[i].Ip {
						if update.ServiceUpdate {
							sbd.Services[i].ServicesUp[update.ServiceUpIndex] = update.ServiceUp
						} else {
							sbd.Services[i].PingUp = update.ServiceUp
						}
					}
				}
			default:
				if wasLocked {
					sbd.lock.Unlock()
					wasLocked = false
				}
				time.Sleep(10 * time.Second)
			}
		}

	} (updateChannel, &scoreboard)

	http.Handle("/", &scoreboard)
	http.ListenAndServe(":80", nil)
}

// This function simple Opens the config.yaml file and parses it into a go type, then returns that type.
func initConfig() (Config, error) {
	var (
		configFile *os.File
		config Config
	)

	if f, err := os.Open(configFileString); err == nil {
		configFile = f
	} else {
		return config, err
	}

	yamlDecoder := yaml.NewDecoder(configFile)
	if err := yamlDecoder.Decode(&config) ; err == nil {
		return config, nil
	} else {
		return config, err
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