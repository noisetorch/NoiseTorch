package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type config struct {
	Threshold             int
	DisplayMonitorSources bool
}

const configDir = ".config/noiseui/"
const configFile = "config.toml"

func initializeConfigIfNot() {
	log.Println("Checking if config needs to be initialized")
	conf := config{Threshold: 95, DisplayMonitorSources: false}
	configdir := filepath.Join(os.Getenv("HOME"), configDir)
	ok, err := exists(configdir)
	if err != nil {
		log.Fatalf("Couldn't check if config directory exists: %v\n", err)
	}
	if !ok {
		err = os.MkdirAll(configdir, 0700)
		if err != nil {
			log.Fatalf("Couldn't create config directory: %v\n", err)
		}
	}
	tomlfile := filepath.Join(configdir, configFile)
	ok, err = exists(tomlfile)
	if err != nil {
		log.Fatalf("Couldn't check if config file exists: %v\n", err)
	}
	if !ok {
		log.Println("Initializing config")
		writeConfig(&conf)
	}
}

func readConfig() *config {
	f := filepath.Join(os.Getenv("HOME"), configDir, configFile)
	config := config{}
	if _, err := toml.DecodeFile(f, &config); err != nil {
		log.Fatalf("Couldn't read config file: %v\n", err)
	}

	return &config
}

func writeConfig(conf *config) {
	f := filepath.Join(os.Getenv("HOME"), configDir, configFile)
	var buffer bytes.Buffer
	if err := toml.NewEncoder(&buffer).Encode(&conf); err != nil {
		log.Fatalf("Couldn't write config file: %v\n", err)
	}
	ioutil.WriteFile(f, []byte(buffer.String()), 0644)
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
