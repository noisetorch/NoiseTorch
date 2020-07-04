package main

import (
	"image"
	"io/ioutil"
	"log"
	"os"

	"github.com/aarzilli/nucular/font"

	"github.com/lawl/pulseaudio"

	"github.com/aarzilli/nucular"
	"github.com/aarzilli/nucular/style"
)

//go:generate go run scripts/embedlibrnnoise.go

type input struct {
	ID        string
	Name      string
	isMonitor bool
	checked   bool
}

func main() {

	f, err := os.OpenFile("/tmp/noiseui.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("error opening file: %v\n", err)
	}
	defer f.Close()
	log.SetOutput(f)
	log.Println("Application starting.")

	initializeConfigIfNot()
	rnnoisefile := dumpLib()
	defer removeLib(rnnoisefile)

	ui := uistate{}
	ui.config = readConfig()
	ui.librnnoise = rnnoisefile

	paClient, err := pulseaudio.NewClient()
	defer paClient.Close()

	ui.paClient = paClient
	if err != nil {
		log.Fatalf("Couldn't create pulseaudio client\n")
	}

	go updateNoiseSupressorLoaded(paClient, &ui.noiseSupressorState)

	sources, err := paClient.Sources()
	if err != nil {
		log.Fatalf("Couldn't fetch sources from pulseaudio\n")
	}

	inputs := make([]input, 0)
	for i := range sources {
		if sources[i].Name == "nui_mic_remap" {
			continue
		}

		var inp input

		inp.ID = sources[i].Name
		inp.Name = sources[i].PropList["device.description"]
		inp.isMonitor = (sources[i].MonitorSourceIndex != 0xffffffff)

		inputs = append(inputs, inp)
	}

	ui.inputList = inputs

	wnd := nucular.NewMasterWindowSize(0, "NoiseUI", image.Point{550, 300}, func(w *nucular.Window) {
		updatefn(w, &ui)
	})
	ui.masterWindow = &wnd
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
