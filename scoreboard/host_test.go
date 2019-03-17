package scoreboard

import (
	"os/user"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPingHost(t *testing.T) {
	t.Parallel()

	if usr, err := user.Current(); err == nil {

		// Attempt to identify the Administrator group
		if runtime.GOOS == "windows" && !strings.HasSuffix(usr.Gid, "-544") {
			t.Fatal("Please run as Administrator. " +
				"This test needs Administrator to do ICMP.")

			t.SkipNow()
		} else if usr.Gid != "0" && usr.Uid != "0" { // ID root
			if runtime.GOOS == "linux" {
				t.Fatal("Please run as root. " +
					"This test needs root to do ICMP.")
			} else { // Dunno bud
				t.Fatal("Please run with elevated privileges. " +
					"This test needs elevated privileges to do ICMP")
			}

			t.SkipNow()
		}
	}

	type PingHostTestTable struct {
		Scoreboards       []State
		ConfigShouldError []bool
	}

	testTable := PingHostTestTable{
		[]State{
			{
				[]Host{
					{
						"google",
						[]Service{
							{
								"http",
								"",
								"wget -o /dev/null www.google.com",
								"200 OK",
								"host-command",
								false,
							},
							{
								"dns",
								"",
								"dig www.google.com @8.8.8.8",
								"ANSWER: 1",
								"host-command",
								false,
							},
							{
								"drive",
								"80",
								"GET / HTTP/1.0\r\n\r\n",
								"200 OK",
								"tcp",
								false,
							},
						},
						"172.217.0.132",
						false,
					},
				},
				Config{
					true,
					time.Duration(60 * time.Second),
					time.Duration(7 * time.Second),
					time.Duration(120 * time.Second),
					time.Duration(10 * time.Second),
				},
				sync.RWMutex{},
			},
			{
				[]Host{
					{
						"Quad 9",
						[]Service{
							{
								"dns",
								"",
								"dig www.google.com @ 9.9.9.9",
								"ANSWER: 1",
								"host-command",
								false,
							},
						},
						"9.9.9.9",
						false,
					},
				},
				Config{
					true,
					time.Duration(60 * time.Second),
					time.Duration(7 * time.Second),
					time.Duration(120 * time.Second),
					time.Duration(10 * time.Second),
				},
				sync.RWMutex{},
			},
		},
		[]bool{
			false,
			false,
		},
	}

	// Test that our setup is mildly correct
	if len(testTable.Scoreboards) != len(testTable.ConfigShouldError) {
		t.Fatalf("Test Table not setup correctly: Length of "+
			"Configs and ConfigShouldError should match! "+
			"len(Configs): %d len(ConfigShouldError): %d",
			len(testTable.Scoreboards), len(testTable.ConfigShouldError))

		t.FailNow()
	}

	outputChannel := make(chan ServiceUpdate)
	durationToWait := testTable.Scoreboards[0].Config.PingTimeout * 2
	expectedNumUpdates := len(testTable.ConfigShouldError)
	termChan := make(chan bool)

	t.Logf("Expected number of ping updates: %d", expectedNumUpdates)

	t.Log("Starting update receiver")
	go func(inputChannel chan ServiceUpdate, termChannel chan bool) {
		count := 0

		time.AfterFunc(durationToWait, func() {
			if count != expectedNumUpdates {
				t.Fatalf("Did not get expected number of "+
					"ping updates! Expected %d got %d",
					expectedNumUpdates, count)
				t.FailNow()
			} else {
				t.Logf("Got correct amount of ping updates!")
			}

			termChannel <- true
		})

		for {
			select {
			case <-termChannel:
				return
			case update := <-inputChannel:
				if update.IsUp {
					count++
				}
			}
		}

	}(outputChannel, termChan)

	for i := range testTable.Scoreboards {
		sbd := testTable.Scoreboards[i]

		for _, host := range sbd.Hosts {
			host := host // Capture host
			t.Logf("Pinging %v", host.Ip)
			go host.PingHost(outputChannel, sbd.Config.PingTimeout)
		}
	}

	<-termChan
}
