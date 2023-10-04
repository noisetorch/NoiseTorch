// This file is part of the program "NoiseTorch-ng".
// Please see the LICENSE file for copyright information.

package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type config struct {
	Threshold             int
	DisplayMonitorSources bool
	EnableUpdates         bool
	FilterInput           bool
	FilterOutput          bool
	LastUsedInput         string
	LastUsedOutput        string
}

const configFile = "config.toml"

func initializeConfigIfNot() {
	log.Println("Checking if config needs to be initialized")

	// if you're a package maintainer and you mess with this, we have a problem.
	// Unless you set -tags release on the build the updater is *not* compiled in anymore. DO NOT MESS WITH THIS!
	// This isn't and never was the proper location to disable the updater.
	conf := config{
		Threshold:             95,
		DisplayMonitorSources: false,
		EnableUpdates:         true,
		FilterInput:           true,
		FilterOutput:          false,
		LastUsedInput:         "",
		LastUsedOutput:        ""}

	configdir := configDir()
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
	f := filepath.Join(configDir(), configFile)
	config := config{}
	if _, err := toml.DecodeFile(f, &config); err != nil {
		log.Fatalf("Couldn't read config file: %v\n", err)
	}

	return &config
}

func writeConfig(conf *config) {
	f := filepath.Join(configDir(), configFile)
	var buffer bytes.Buffer
	if err := toml.NewEncoder(&buffer).Encode(&conf); err != nil {
		log.Fatalf("Couldn't write config file: %v\n", err)
	}
	os.WriteFile(f, buffer.Bytes(), 0644)
}

func configDir() string {
	return filepath.Join(xdgOrFallback("XDG_CONFIG_HOME", filepath.Join(os.Getenv("HOME"), ".config")), "noisetorch")
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

func xdgOrFallback(xdg string, fallback string) string {
	dir := os.Getenv(xdg)
	if dir != "" {
		if ok, err := exists(dir); ok && err == nil {
			log.Printf("Resolved $%s to '%s'\n", xdg, dir)
			return dir
		}

	}

	log.Printf("Couldn't resolve $%s falling back to '%s'\n", xdg, fallback)
	return fallback
}
