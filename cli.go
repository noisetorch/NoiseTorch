package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/lawl/pulseaudio"
)

func doCLI(config *config, librnnoise string) {
	var setcap bool
	var sinkName string
	var unload bool
	var loadInput bool
	var loadOutput bool
	var threshold int
	var list bool

	flag.BoolVar(&setcap, "setcap", false, "for internal use only")
	flag.StringVar(&sinkName, "s", "", "Use the specified source/sink device ID")
	flag.BoolVar(&loadInput, "i", false, "Load supressor for input. If no source device ID is specified the default pulse audio source is used.")
	flag.BoolVar(&loadOutput, "o", false, "Load supressor for output. If no source device ID is specified the default pulse audio source is used.")
	flag.BoolVar(&unload, "u", false, "Unload supressor")
	flag.IntVar(&threshold, "t", -1, "Voice activation threshold")
	flag.BoolVar(&list, "l", false, "List available PulseAudio devices")
	flag.Parse()

	paClient, err := pulseaudio.NewClient()

	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't create pulseaudio client: %v\n", err)
		os.Exit(1)
	}
	defer paClient.Close()

	ctx := ntcontext{}
	ctx.config = config
	ctx.librnnoise = librnnoise

	ctx.paClient = paClient

	if setcap {
		err := makeBinarySetcapped()
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	if list {
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

		os.Exit(0)
	}

	if threshold > 0 {
		if threshold > 95 {
			fmt.Fprintf(os.Stderr, "Threshold of '%d' too high, setting to maximum of 95.\n", threshold)
			ctx.config.Threshold = 95
		} else {
			ctx.config.Threshold = threshold
		}
	}

	if unload {
		err := unloadSupressor(&ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error unloading PulseAudio Module: %+v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if loadInput {
		sources := getSources(paClient)

		if sinkName == "" {
			defaultSource, err := getDefaultSourceID(paClient)
			if err != nil {
				fmt.Fprintf(os.Stderr, "No source specified to load and failed to load default source: %+v\n", err)
				os.Exit(1)
			}
			sinkName = defaultSource
		}
		for i := range sources {
			if sources[i].ID == sinkName {
				sources[i].checked = true
				err := loadSupressor(&ctx, &sources[i], &device{})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error loading PulseAudio Module: %+v\n", err)
					os.Exit(1)
				}
				os.Exit(0)
			}
		}
		fmt.Fprintf(os.Stderr, "PulseAudio source not found: %s\n", sinkName)
		os.Exit(1)

	}
	if loadOutput {
		sinks := getSinks(paClient)

		if sinkName == "" {
			defaultSink, err := getDefaultSinkID(paClient)
			if err != nil {
				fmt.Fprintf(os.Stderr, "No sink specified to load and failed to load default sink: %+v\n", err)
				os.Exit(1)
			}
			sinkName = defaultSink
		}
		for i := range sinks {
			if sinks[i].ID == sinkName {
				sinks[i].checked = true
				err := loadSupressor(&ctx, &device{}, &sinks[i])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error loading PulseAudio Module: %+v\n", err)
					os.Exit(1)
				}
				os.Exit(0)
			}
		}
		fmt.Fprintf(os.Stderr, "PulseAudio sink not found: %s\n", sinkName)
		os.Exit(1)

	}
}
