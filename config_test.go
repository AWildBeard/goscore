package main

import (
	"github.com/AWildBeard/goscore/scoreboard"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	type ValidateConfigTestTable struct {
		Configs           []Config
		ConfigShouldError []bool
	}

	testTable := ValidateConfigTestTable{
		[]Config{
			{
				[]scoreboard.Host{
					{
						"google",
						[]scoreboard.Service{
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
				map[string]string{
					"pingHosts":       "yes",
					"pingInterval":    "60s",
					"pingTimeout":     "7s",
					"serviceInterval": "120s",
					"serviceTimeout":  "10s",
				},
			},
			{
				[]scoreboard.Host{
					{
						"cloudflare",
						[]scoreboard.Service{
							{
								"dns",
								"",
								"dig one.one.one.one @1.1.1.1",
								"ANSWER: 1",
								"",
								false,
							},
							{
								"",
								"",
								"dig one.one.one.one @1.1.1.1",
								"ANSWER: 1",
								"host-command",
								false,
							},
						},
						"1.1.1.1",
						false,
					},
					{
						"quad 9",
						[]scoreboard.Service{
							{
								"dns",
								"",
								"dig www.google.com @ 9.9.9.9",
								"ANSWER: 1",
								"",
								false,
							},
						},
						"9.9.9.9",
						false,
					},
				},
				map[string]string{
					"serviceInterval": "120s",
				},
			},
			{
				[]scoreboard.Host{
					{
						"Quad 9",
						[]scoreboard.Service{
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
				map[string]string{
					"pingHosts":       "no",
					"serviceInterval": "120s",
					"serviceTimeout":  "10s",
				},
			},
		},
		[]bool{
			false,
			true,
			false,
		},
	}

	// Test that our setup is mildly correct
	if len(testTable.Configs) != len(testTable.ConfigShouldError) {
		t.Fatalf("Test Table not setup correctly: Length of "+
			"Configs and ConfigShouldError should match! "+
			"len(Configs): %d len(ConfigShouldError): %d",
			len(testTable.Configs), len(testTable.ConfigShouldError))

		t.FailNow()
	}

	// Test the configs
	for i := range testTable.Configs {
		if err := testTable.Configs[i].ValidateConfig(); err == nil {
			if testTable.ConfigShouldError[i] {
				t.Errorf("Config %d did *not* error when it should have", i)
			} else {
				t.Logf("Config %d passed!", i)
			}
		} else {
			if !testTable.ConfigShouldError[i] {
				t.Errorf("Config %d errored when it should not have: %v", i, err)
			} else {
				t.Logf("Config %d passed!", i)
			}
		}
	}
}

func TestParseConfigToScoreboard(t *testing.T) {
	t.Parallel()

	type ParseConfigToScoreboardTestTable struct {
		Configs           []Config
		ConfigShouldError []bool
	}

	testTable := ParseConfigToScoreboardTestTable{
		[]Config{
			{
				[]scoreboard.Host{
					{
						"google",
						[]scoreboard.Service{
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
				map[string]string{
					"pingHosts":       "yes",
					"pingInterval":    "60s",
					"pingTimeout":     "7s",
					"serviceInterval": "120s",
					"serviceTimeout":  "10s",
				},
			},
			{
				[]scoreboard.Host{
					{
						"Quad 9",
						[]scoreboard.Service{
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
				map[string]string{
					"pingHosts":       "no",
					"serviceInterval": "120s",
					"serviceTimeout":  "10",
				},
			},
		},
		[]bool{
			false,
			true,
		},
	}

	// Test that our setup is mildly correct
	if len(testTable.Configs) != len(testTable.ConfigShouldError) {
		t.Fatalf("Test Table not setup correctly: Length of "+
			"Configs and ConfigShouldError should match! "+
			"len(Configs): %d len(ConfigShouldError): %d",
			len(testTable.Configs), len(testTable.ConfigShouldError))

		t.FailNow()
	}

	for i := range testTable.Configs {
		sbd := scoreboard.NewScoreboard()

		if err := ParseConfigToScoreboard(&testTable.Configs[i], &sbd) ; err == nil {
			if testTable.ConfigShouldError[i] {
				t.Fatalf("Config %d did *not* error when it " +
					"was expected to.", i)
			} else {
				t.Logf("Config %d passed!", i)
			}
		} else {
			if ! testTable.ConfigShouldError[i] {
				t.Fatalf("Config %d errored when it should " +
					"not have: %v", i, err)
			} else {
				t.Logf("Config %d passed!", i)
			}
		}
	}
}