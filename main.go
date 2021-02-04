package main

import (
	"flag"
	"fmt"
	"image"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/icccm"
	"github.com/aarzilli/nucular/font"

	"github.com/lawl/pulseaudio"

	"github.com/aarzilli/nucular"
	"github.com/aarzilli/nucular/style"
)

//go:generate go run scripts/embedbinary.go c/ladspa/rnnoise_ladspa.so librnnoise.go libRNNoise
//go:generate go run scripts/embedbinary.go assets/patreon.png patreon.go patreonPNG
//go:generate go run scripts/embedversion.go
//go:generate go run scripts/embedlicenses.go

type device struct {
	ID             string
	Name           string
	isMonitor      bool
	checked        bool
	dynamicLatency bool
	rate           uint32
}

const appName = "NoiseTorch"

func main() {

	var pulsepid int
	var setcap bool
	var sinkName string
	var unload bool
	var loadInput bool
	var loadOutput bool
	var threshold int
	var list bool
	var check bool

	flag.IntVar(&pulsepid, "removerlimit", -1, "for internal use only")
	flag.BoolVar(&setcap, "setcap", false, "for internal use only")
	flag.StringVar(&sinkName, "s", "", "Use the specified source/sink device ID")
	flag.BoolVar(&loadInput, "i", false, "Load supressor for input. If no source device ID is specified the default pulse audio source is used.")
	flag.BoolVar(&loadOutput, "o", false, "Load supressor for output. If no source device ID is specified the default pulse audio source is used.")
	flag.BoolVar(&unload, "u", false, "Unload supressor")
	flag.IntVar(&threshold, "t", -1, "Voice activation threshold")
	flag.BoolVar(&list, "l", false, "List available PulseAudio devices")
	flag.BoolVar(&check, "c", false, "Check supressor status")
	flag.Parse()

	if setcap {
		err := makeBinarySetcapped()
		if err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	//TODO:remove this after 0.10. Not required anymore after that.
	//We don't remove it right now, since someone could have an old instance running that calls the updated binary
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
	log.Printf("CAP_SYS_RESOURCE: %t\n", hasCapSysResource(getCurrentCaps()))

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

	ctx.paClient = paClient

	if check {
		if supressorState(&ctx) == loaded {
			os.Exit(0)
		}
		os.Exit(1)
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

	if ctx.config.EnableUpdates {
		go updateCheck(&ctx)
	}

	go paConnectionWatchdog(&ctx)

	ctx.haveCapabilities = hasCapSysResource(getCurrentCaps())
	ctx.capsMismatch = hasCapSysResource(getCurrentCaps()) != hasCapSysResource(getSelfFileCaps())

	wnd := nucular.NewMasterWindowSize(0, appName, image.Point{600, 400}, func(w *nucular.Window) {
		updatefn(&ctx, w)
	})

	ctx.masterWindow = &wnd
	style := style.FromTheme(style.DarkTheme, 2.0)
	style.Font = font.DefaultFont(16, 1)
	wnd.SetStyle(style)

	//this is a disgusting hack that searches for the noisetorch window
	//and then fixes up the WM_CLASS attribute so it displays
	//properly in the taskbar
	go fixWindowClass()
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

func getSources(client *pulseaudio.Client) []device {
	sources, err := client.Sources()
	if err != nil {
		log.Printf("Couldn't fetch sources from pulseaudio\n")
	}

	outputs := make([]device, 0)
	for i := range sources {
		if strings.Contains(sources[i].Name, "nui_") {
			continue
		}

		log.Printf("Input %s, %+v\n", sources[i].Name, sources[i])

		var inp device

		inp.ID = sources[i].Name
		inp.Name = sources[i].PropList["device.description"]
		inp.isMonitor = (sources[i].MonitorSourceIndex != 0xffffffff)
		inp.rate = sources[i].SampleSpec.Rate

		//PA_SOURCE_DYNAMIC_LATENCY = 0x0040U
		inp.dynamicLatency = sources[i].Flags&uint32(0x0040) != 0

		outputs = append(outputs, inp)
	}

	return outputs
}

func getSinks(client *pulseaudio.Client) []device {
	sources, err := client.Sinks()
	if err != nil {
		log.Printf("Couldn't fetch sources from pulseaudio\n")
	}

	inputs := make([]device, 0)
	for i := range sources {
		if strings.Contains(sources[i].Name, "nui_") {
			continue
		}

		log.Printf("Output %s, %+v\n", sources[i].Name, sources[i])

		var inp device

		inp.ID = sources[i].Name
		inp.Name = sources[i].PropList["device.description"]
		inp.rate = sources[i].SampleSpec.Rate

		// PA_SINK_DYNAMIC_LATENCY = 0x0080U
		inp.dynamicLatency = sources[i].Flags&uint32(0x0080) != 0

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
		go updateNoiseSupressorLoaded(ctx)

		ctx.inputList = preselectDevice(ctx, getSources(ctx.paClient), ctx.config.LastUsedInput, getDefaultSourceID)
		ctx.outputList = preselectDevice(ctx, getSinks(paClient), ctx.config.LastUsedOutput, getDefaultSinkID)

		resetUI(ctx)

		time.Sleep(500 * time.Millisecond)
	}
}

func preselectDevice(ctx *ntcontext, devices []device, preselectID string,
	fallbackFunc func(client *pulseaudio.Client) (string, error)) []device {

	deviceExists := false
	for _, input := range devices {
		deviceExists = deviceExists || input.ID == preselectID
	}

	if !deviceExists {
		defaultDevice, err := fallbackFunc(ctx.paClient)
		if err != nil {
			log.Printf("Failed to load default device: %+v\n", err)
		} else {
			preselectID = defaultDevice
		}
	}
	for i := range devices {
		if devices[i].ID == preselectID {
			devices[i].checked = true
		}
	}
	return devices
}

func getDefaultSourceID(client *pulseaudio.Client) (string, error) {
	server, err := client.ServerInfo()
	if err != nil {
		return "", err
	}
	return server.DefaultSource, nil
}

func getDefaultSinkID(client *pulseaudio.Client) (string, error) {
	server, err := client.ServerInfo()
	if err != nil {
		return "", err
	}
	return server.DefaultSink, nil
}

//this is disgusting
func fixWindowClass() {
	xu, err := xgbutil.NewConn()
	defer xu.Conn().Close()
	if err != nil {
		log.Printf("Couldn't create XU xdg conn: %+v\n", err)
		return
	}
	for i := 0; i < 100; i++ {
		wnds, _ := ewmh.ClientListGet(xu)
		for _, w := range wnds {
			n, _ := ewmh.WmNameGet(xu, w)
			if n == appName {
				_, err := icccm.WmClassGet(xu, w)
				//if we have *NO* WM_CLASS, then the above call errors. We *want* to make sure this errors
				if err == nil {
					continue
				}

				class := icccm.WmClass{}
				class.Class = appName
				class.Instance = appName
				icccm.WmClassSet(xu, w, &class)
				return
			}

		}
		time.Sleep(100 * time.Millisecond)
	}

}
