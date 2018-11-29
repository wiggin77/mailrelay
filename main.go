package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
)

type loggerLevels struct {
	Debug *log.Logger
	Error *log.Logger
}

type mailRelayConfig struct {
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Logger provides application logging.
var Logger loggerLevels

func main() {

	Logger.Debug = log.New(os.Stdout, "debug: ", log.Ldate|log.Ltime|log.Lshortfile)
	Logger.Error = log.New(os.Stderr, "error: ", log.Ldate|log.Ltime|log.Lshortfile)

	var configFile string
	flag.StringVar(&configFile, "config", "/etc/mailrelay.json", "specifies JSON config file")
	flag.Parse()

	appConfig, err := loadConfig(configFile)
	if err != nil {
		Logger.Error.Fatalf("loading config: %v", err)
	}

	err = Start(appConfig)
	if err != nil {
		Logger.Error.Fatalf("starting server: %v", err)
	}

	// Wait for SIGINT
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)

	// Block until a signal is received.
	s := <-c
	fmt.Println("Got signal:", s)
	os.Exit(0)
}

func loadConfig(path string) (*mailRelayConfig, error) {
	var cfg mailRelayConfig
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	parser := json.NewDecoder(file)
	if err := parser.Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
