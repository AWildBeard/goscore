package main

import (
	"flag"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
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
	timeBetweenPings time.Duration
	timeBetweenServices time.Duration

    // Logger
    ilog *log.Logger
    dlog *log.Logger
)

func init() {
	flag.StringVar(&configFileString, "c", "config.yaml",
	    "Specify a custom config file location. Default: `config.yaml`")
	flag.BoolVar(&debug, "d", true, "Print debug messages")
	ilog = log.New(os.Stdout, "", 0)
}

func main() {
    flag.Parse()

    var (
    	interruptSignals = make(chan os.Signal, 1)
	)

    if debug {
        dlog = log.New(os.Stderr, "DBG: ", log.Ltime)
    } else {
        dlog = log.New(ioutil.Discard, "", 0)
    }

    initConfig()

    signal.Notify(interruptSignals, os.Interrupt)

    if pingHosts {
    	// Thread for pinging hosts
		go func () {
			dlog.Println("Starting ping provider")
			for {
				time.Sleep(timeBetweenPings)
				for _, mp := range config.Services {
					ilog.Printf("Pinging %v at %v\n", mp["service"], mp["ip"])
				}
			}
		} ()
	}

	// Thread for querying services
    go func () {
    	dlog.Println("Starting service provider")
    	for {
    		time.Sleep(timeBetweenServices)
			for _, mp := range config.Services {
				ilog.Printf("Checking service %v at %v:%v\n", mp["service"], mp["ip"], mp["port"])
			}
		}
	} ()

    // Pop a sigint off when it arrives. Otherwise we wait :D
	<- interruptSignals
    dlog.Println("Exiting!")
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
		if config.Config["pingHosts"] == "false" {
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