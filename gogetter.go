package main

import (
    "gopkg.in/yaml.v2"
    "fmt"
    "os"
)

type Config struct {
    Services []map[string]string `services:`
    Config   map[string]string `config:`
}

func main() {
    var configFile *os.File
    var config Config
    if f, err := os.Open("config.yaml"); err == nil {
        fmt.Println("Opend file!")
        configFile = f
    } else {
        fmt.Println("Failed to open file!")
        os.Exit(0)
    }

    yamlDecoder := yaml.NewDecoder(configFile)
    if err := yamlDecoder.Decode(&config) ; err == nil {
        fmt.Println("Sucessfully decoded yaml!")
        fmt.Println(config)
        fmt.Println("-----------------")
        for _, mp := range config.Services {
            for key, val := range mp {
                fmt.Printf("%v: %v\n", key, val)
            }
            fmt.Println("-----------------")
        }
    } else {
        fmt.Println("Failed to decode yaml :(")
        fmt.Println(err)
    }

}