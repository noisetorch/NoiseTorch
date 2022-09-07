// This file is part of the program "NoiseTorch-ng".
// Please see the LICENSE file for copyright information.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/noisetorch/pulseaudio"
)

type CLIOpts struct {
	verbose     bool
	setcap      bool
	sinkName    string
	loadInput   bool
	loadOutput  bool
	unload      bool
	threshold   int
	list        bool
	checkUpdate bool
}

func parseCLIOpts() CLIOpts {
	var opt CLIOpts
	flag.BoolVar(&opt.verbose, "v", false, "Verbose output (print logs to stderr)")
	flag.BoolVar(&opt.setcap, "setcap", false, "for internal use only")
	flag.StringVar(&opt.sinkName, "s", "", "Use the specified source/sink device ID")
	flag.BoolVar(&opt.loadInput, "i", false, "Load supressor for input. If no source device ID is specified the default pulse audio source is used.")
	flag.BoolVar(&opt.loadOutput, "o", false, "Load supressor for output. If no source device ID is specified the default pulse audio source is used.")
	flag.BoolVar(&opt.unload, "u", false, "Unload supressor")
	flag.IntVar(&opt.threshold, "t", -1, "Voice activation threshold")
	flag.BoolVar(&opt.list, "l", false, "List available PulseAudio devices")
	flag.BoolVar(&opt.checkUpdate, "c", false, "Check if update is available (but do not update)")
	flag.Parse()

	return opt
}

func doCLI(opt CLIOpts, config *config, librnnoise string) {
	if opt.verbose {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(ioutil.Discard)
	}

	if opt.checkUpdate {
		latestRelease, err := getLatestRelease()
		if err == nil {
			latestVersion, _ := semver.Make(strings.TrimLeft(latestRelease, "v"))
			currentVersion, _ := semver.Make(strings.TrimLeft(version, "v"))
			if currentVersion.Compare(latestVersion) == -1 {
				fmt.Println("New version available: " + latestRelease)
			} else {
				fmt.Println("No update available")
			}
		} else {
			fmt.Println("Cannot look for updates right now.")
		}
		cleanupExit(librnnoise, 0)
	}

	if opt.setcap {
		exitCode := 0
		if err := makeBinarySetcapped(); err != nil {
			exitCode = 1
		}
		cleanupExit(librnnoise, exitCode)
	}

	paClient, err := pulseaudio.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't create pulseaudio client: %v\n", err)
		cleanupExit(librnnoise, 1)
	}
	defer paClient.Close()

	ctx := ntcontext{}

	info, err := serverInfo(paClient)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't fetch audio server info: %s\n", err)
	}
	ctx.serverInfo = info

	ctx.config = config
	ctx.librnnoise = librnnoise

	ctx.paClient = paClient

	if opt.list {
		fmt.Println("Sources:")
		sources := getSources(paClient)
		for i := range sources {
			fmt.Printf("\tDevice Name: %s\n\tDevice ID: %s\n\n", sources[i].Name, sources[i].ID)
		}

		fmt.Println("Sinks:")
		sinks := getSinks(paClient)
		for i := range sinks {
			fmt.Printf("\tDevice Name: %s\n\tDevice ID: %s\n\n", sinks[i].Name, sinks[i].ID)
		}

		cleanupExit(librnnoise, 0)
	}

	if opt.threshold > 0 {
		if opt.threshold > 95 {
			fmt.Fprintf(os.Stderr, "Threshold of '%d' too high, setting to maximum of 95.\n", opt.threshold)
			ctx.config.Threshold = 95
		} else {
			ctx.config.Threshold = opt.threshold
		}
	}

	if opt.unload {
		err := unloadSupressor(&ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error unloading PulseAudio Module: %+v\n", err)
			cleanupExit(librnnoise, 1)
		}
		cleanupExit(librnnoise, 0)
	}

	if opt.loadInput {
		sources := getSources(paClient)

		if opt.sinkName == "" {
			defaultSource, err := getDefaultSourceID(paClient)
			if err != nil {
				fmt.Fprintf(os.Stderr, "No source specified to load and failed to load default source: %+v\n", err)
				cleanupExit(librnnoise, 1)
			}
			opt.sinkName = defaultSource
		}
		for i := range sources {
			if sources[i].ID == opt.sinkName {
				sources[i].checked = true
				err := loadSupressor(&ctx, &sources[i], &device{})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error loading PulseAudio Module: %+v\n", err)
					cleanupExit(librnnoise, 1)
				}
				cleanupExit(librnnoise, 0)
			}
		}
		fmt.Fprintf(os.Stderr, "PulseAudio source not found: %s\n", opt.sinkName)
		cleanupExit(librnnoise, 1)

	}
	if opt.loadOutput {
		sinks := getSinks(paClient)

		if opt.sinkName == "" {
			defaultSink, err := getDefaultSinkID(paClient)
			if err != nil {
				fmt.Fprintf(os.Stderr, "No sink specified to load and failed to load default sink: %+v\n", err)
				cleanupExit(librnnoise, 1)
			}
			opt.sinkName = defaultSink
		}
		for i := range sinks {
			if sinks[i].ID == opt.sinkName {
				sinks[i].checked = true
				err := loadSupressor(&ctx, &device{}, &sinks[i])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error loading PulseAudio Module: %+v\n", err)
					cleanupExit(librnnoise, 1)
				}
				cleanupExit(librnnoise, 0)
			}
		}
		fmt.Fprintf(os.Stderr, "PulseAudio sink not found: %s\n", opt.sinkName)
		cleanupExit(librnnoise, 1)

	}

}

func cleanupExit(librnnoise string, exitCode int) {
	removeLib(librnnoise)
	os.Exit(exitCode)
}
