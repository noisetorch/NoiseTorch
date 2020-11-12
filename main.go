package main

import (
	"flag"
	"fmt"
	"image"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/aarzilli/nucular/font"

	"github.com/lawl/pulseaudio"

	"github.com/aarzilli/nucular"
	"github.com/aarzilli/nucular/style"
)

//go:generate go run scripts/embedlibrnnoise.go
//go:generate go run scripts/embedversion.go
//go:generate go run scripts/embedlicenses.go

type input struct {
	ID             string
	Name           string
	isMonitor      bool
	checked        bool
	dynamicLatency bool
}

func main() {

	var pulsepid int
	var sourceName string
	var unload bool
	var load bool
	var threshold int
	var list bool

	flag.IntVar(&pulsepid, "removerlimit", -1, "for internal use only")
	flag.StringVar(&sourceName, "s", "", "Use the specified source device ID")
	flag.BoolVar(&load, "i", false, "Load supressor for input")
	flag.BoolVar(&unload, "u", false, "Unload supressor")
	flag.IntVar(&threshold, "t", -1, "Voice activation threshold")
	flag.BoolVar(&list, "l", false, "List available PulseAudio sources")
	flag.Parse()

	if pulsepid > 0 {
		const MaxUint = ^uint64(0)
		new := syscall.Rlimit{Cur: MaxUint, Max: MaxUint}
		err := setRlimit(pulsepid, &new)
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	date := time.Now().Format("2006_01_02_03_04_05")
	tmpdir := os.TempDir()
	f, err := os.OpenFile(filepath.Join(tmpdir, fmt.Sprintf("noisetorch-%s.log", date)), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("error opening file: %v\n", err)
	}
	defer f.Close()

	log.SetOutput(f)
	log.Printf("Application starting. Version: %s\n", version)

	initializeConfigIfNot()
	rnnoisefile := dumpLib()
	defer removeLib(rnnoisefile)

	ctx := ntcontext{}
	ctx.config = readConfig()
	ctx.librnnoise = rnnoisefile

	paClient, err := pulseaudio.NewClient()
	if err != nil {
		log.Printf("Couldn't create pulseaudio client: %v\n", err)
		os.Exit(1)
	}

	if list {
		sources := getSources(paClient)
		for i := range sources {
			fmt.Printf("Device Name: %s\nDevice ID: %s\n\n", sources[i].Name, sources[i].ID)
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
		unloadSupressor(paClient)
		os.Exit(0)
	}

	if load {
		if sourceName == "" {
			fmt.Fprintf(os.Stderr, "No source specified to load.\n")
			os.Exit(1)
		}

		if supressorState(paClient) != unloaded {
			fmt.Fprintf(os.Stderr, "Supressor is already loaded.\n")
			os.Exit(1)
		}

		sources := getSources(paClient)
		for i := range sources {
			if sources[i].ID == sourceName {
				loadSupressor(&ctx, sources[i])
				os.Exit(0)
			}
		}

		fmt.Fprintf(os.Stderr, "PulseAudio source not found: %s\n", sourceName)
		os.Exit(1)

	}

	if ctx.config.EnableUpdates {
		go updateCheck(&ctx)
	}

	go paConnectionWatchdog(&ctx)

	wnd := nucular.NewMasterWindowSize(0, "NoiseTorch", image.Point{550, 300}, func(w *nucular.Window) {
		updatefn(&ctx, w)
	})
	ctx.masterWindow = &wnd
	style := style.FromTheme(style.DarkTheme, 2.0)
	style.Font = font.DefaultFont(16, 1)
	wnd.SetStyle(style)
	wnd.Main()

}

func dumpLib() string {
	f, err := ioutil.TempFile("", "librnnoise-*.so")
	if err != nil {
		log.Fatalf("Couldn't open temp file for librnnoise\n")
	}
	f.Write(libRNNoise)
	log.Printf("Wrote temp librnnoise to: %s\n", f.Name())
	return f.Name()
}

func removeLib(file string) {
	err := os.Remove(file)
	if err != nil {
		log.Printf("Couldn't delete temp librnnoise: %v\n", err)
	}
	log.Printf("Deleted temp librnnoise: %s\n", file)
}

func getSources(client *pulseaudio.Client) []input {
	sources, err := client.Sources()
	if err != nil {
		log.Printf("Couldn't fetch sources from pulseaudio\n")
	}

	inputs := make([]input, 0)
	for i := range sources {
		if sources[i].Name == "nui_mic_remap" {
			continue
		}

		log.Printf("Input %s, %+v\n", sources[i].Name, sources[i])

		var inp input

		inp.ID = sources[i].Name
		inp.Name = sources[i].PropList["device.description"]
		inp.isMonitor = (sources[i].MonitorSourceIndex != 0xffffffff)

		//PA_SOURCE_DYNAMIC_LATENCY = 0x0040U
		inp.dynamicLatency = sources[i].Flags&uint32(0x0040) != 0

		inputs = append(inputs, inp)
	}

	return inputs
}

func paConnectionWatchdog(ctx *ntcontext) {
	for {
		if ctx.paClient.Connected() {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		paClient, err := pulseaudio.NewClient()
		if err != nil {
			log.Printf("Couldn't create pulseaudio client: %v\n", err)
			fmt.Fprintf(os.Stderr, "Couldn't create pulseaudio client: %v\n", err)
		}

		ctx.paClient = paClient
		go updateNoiseSupressorLoaded(paClient, &ctx.noiseSupressorState)

		ctx.inputList = getSources(paClient)

		resetUI(ctx)

		time.Sleep(500 * time.Millisecond)
	}
}
