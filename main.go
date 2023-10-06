// This file is part of the program "NoiseTorch-ng".
// Please see the LICENSE file for copyright information.

package main

import (
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/BurntSushi/xgbutil/icccm"
	"github.com/aarzilli/nucular/font"

	"github.com/noisetorch/pulseaudio"

	_ "embed"

	"github.com/aarzilli/nucular"
	"github.com/aarzilli/nucular/style"
)

//go:generate go run scripts/embedlicenses.go

//go:embed c/ladspa/rnnoise_ladspa.so
var libRNNoise []byte

type device struct {
	ID             string
	Name           string
	isMonitor      bool
	checked        bool
	dynamicLatency bool
	rate           uint32
}

var appName = "NoiseTorch-ng"

var nameSuffix = ""         // will be changed by build
var version = "unknown"     // ditto
var distribution = "custom" // ditto
var updateURL = ""          // ditto
var publicKeyString = ""    // ditto
var websiteURL = ""         // ditto

func main() {
	if nameSuffix != "" {
		appName += strings.Replace(nameSuffix, "_", " ", -1)
	}

	opt := parseCLIOpts()

	if opt.doLog {
		log.SetOutput(os.Stdout)
	} else {
		log.SetOutput(io.Discard)
	}
	log.Printf("Application starting. Version: %s (%s)\n", version, distribution)
	log.Printf("CAP_SYS_RESOURCE: %t\n", hasCapSysResource(getCurrentCaps()))

	initializeConfigIfNot()
	rnnoisefile := dumpLib()
	defer removeLib(rnnoisefile)

	ctx := ntcontext{}
	ctx.config = readConfig()
	ctx.librnnoise = rnnoisefile

	doCLI(opt, ctx.config, ctx.librnnoise)

	if ctx.config.EnableUpdates {
		go updateCheck(&ctx)
	}

	ctx.haveCapabilities = hasCapSysResource(getCurrentCaps())
	ctx.capsMismatch = hasCapSysResource(getCurrentCaps()) != hasCapSysResource(getSelfFileCaps())

	resetUI(&ctx)

	wnd := nucular.NewMasterWindowSize(0, appName, image.Point{600, 400}, func(w *nucular.Window) {
		updatefn(&ctx, w)
	})

	ctx.masterWindow = &wnd
	(*ctx.masterWindow).Changed()

	go paConnectionWatchdog(&ctx)

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
	f, err := os.CreateTemp("", "librnnoise-*.so")
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

func getSources(ctx *ntcontext, client *pulseaudio.Client) []device {
	sources, err := client.Sources()
	if err != nil {
		log.Printf("Couldn't fetch sources from pulseaudio\n")
	}

	outputs := make([]device, 0)
	for i := range sources {
		if strings.Contains(sources[i].Name, "nui_") || strings.Contains(sources[i].Name, "Filtered") {
			continue
		}

		var inp device

		inp.ID = sources[i].Name
		if ctx.serverInfo.servertype == servertype_pulse {
			inp.Name = sources[i].PropList["device.description"]
		} else {
			inp.Name = sources[i].Description
		}
		inp.isMonitor = (sources[i].MonitorSourceIndex != 0xffffffff)
		inp.rate = sources[i].SampleSpec.Rate

		//PA_SOURCE_DYNAMIC_LATENCY = 0x0040U
		inp.dynamicLatency = sources[i].Flags&uint32(0x0040) != 0

		outputs = append(outputs, inp)
	}

	return outputs
}

func getSinks(ctx *ntcontext, client *pulseaudio.Client) []device {
	sources, err := client.Sinks()
	if err != nil {
		log.Printf("Couldn't fetch sources from pulseaudio\n")
	}

	inputs := make([]device, 0)
	for i := range sources {
		if strings.Contains(sources[i].Name, "nui_") || strings.Contains(sources[i].Name, "Filtered") {
			continue
		}

		log.Printf("Output %s, %+v\n", sources[i].Name, sources[i])

		var inp device

		inp.ID = sources[i].Name
		if ctx.serverInfo.servertype == servertype_pulse {
			inp.Name = sources[i].PropList["device.description"]
		} else {
			inp.Name = sources[i].Description
		}
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

		ctx.views.Push(connectView)
		(*ctx.masterWindow).Changed()

		paClient, err := pulseaudio.NewClient()
		if err != nil {
			log.Printf("Couldn't create pulseaudio client: %v\n", err)
			fmt.Fprintf(os.Stderr, "Couldn't create pulseaudio client: %v\n", err)
		}

		info, err := serverInfo(paClient)
		if err != nil {
			log.Printf("Couldn't fetch audio server info: %s\n", err)
		}
		ctx.serverInfo = info

		log.Printf("Connected to audio server. Server name '%s'\n", info.name)

		ctx.paClient = paClient
		go updateNoiseSupressorLoaded(ctx)

		ctx.inputList = preselectDevice(ctx, getSources(ctx, paClient), ctx.config.LastUsedInput, getDefaultSourceID)
		ctx.outputList = preselectDevice(ctx, getSinks(ctx, paClient), ctx.config.LastUsedOutput, getDefaultSinkID)

		resetUI(ctx)
		(*ctx.masterWindow).Changed()

		time.Sleep(500 * time.Millisecond)
	}
}

func serverInfo(paClient *pulseaudio.Client) (audioserverinfo, error) {
	info, err := paClient.ServerInfo()
	if err != nil {
		log.Printf("Couldn't fetch pulse server info: %v\n", err)
		fmt.Fprintf(os.Stderr, "Couldn't fetch pulse server info: %v\n", err)
	}

	pkgname := info.PackageName
	log.Printf("Audioserver package name: %s\n", pkgname)
	log.Printf("Audioserver package version: %s\n", info.PackageVersion)
	isPipewire := strings.Contains(pkgname, "PipeWire")

	var servername string
	var servertype uint
	var major, minor, patch int
	var versionRegex *regexp.Regexp
	var versionString string

	var outdatedPipeWire bool

	if isPipewire {
		servername = "PipeWire"
		servertype = servertype_pipewire
		versionRegex = regexp.MustCompile(`.*?on PipeWire (\d+)\.(\d+)\.(\d+).*?`)
		versionString = pkgname
		log.Printf("Detected PipeWire\n")
	} else {
		servername = "PulseAudio"
		servertype = servertype_pulse
		versionRegex = regexp.MustCompile(`.*?(\d+)\.(\d+)\.?(\d+)?.*?`)
		versionString = info.PackageVersion
		log.Printf("Detected PulseAudio\n")
	}

	res := versionRegex.FindStringSubmatch(versionString)
	if len(res) != 4 {
		log.Printf("couldn't parse server version, regexp didn't match version: %s\n", versionString)
		return audioserverinfo{servertype: servertype}, nil
	}
	// the server version did not match the standard `major.minor.patch` pattern
	// setting the patch version to default 0
	if res[3] == "" {
		res[3] = "0"
	}
	major, err = strconv.Atoi(res[1])
	if err != nil {
		return audioserverinfo{servertype: servertype}, err
	}
	minor, err = strconv.Atoi(res[2])
	if err != nil {
		return audioserverinfo{servertype: servertype}, err
	}
	patch, err = strconv.Atoi(res[3])
	if err != nil {
		return audioserverinfo{servertype: servertype}, err
	}
	if isPipewire && major <= 0 && minor <= 3 && patch < 28 {
		log.Printf("pipewire version %d.%d.%d too old.\n", major, minor, patch)
		outdatedPipeWire = true
	}

	return audioserverinfo{
		servertype:       servertype,
		name:             servername,
		major:            major,
		minor:            minor,
		patch:            patch,
		outdatedPipeWire: outdatedPipeWire}, nil
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

// this is disgusting
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
