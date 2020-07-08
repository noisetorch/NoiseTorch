package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/lawl/pulseaudio"
)

const (
	loaded = iota
	unloaded
	inconsistent
)

// the ugly and (partially) repeated strings are unforunately difficult to avoid, as it's what pulse audio expects

func updateNoiseSupressorLoaded(c *pulseaudio.Client, b *int) {

	upd, err := c.Updates()
	if err != nil {
		fmt.Printf("Error listening for updates: %v\n", err)
	}

	for {
		*b = supressorState(c)
		<-upd
	}
}

func supressorState(c *pulseaudio.Client) int {
	//perform some checks to see if it looks like the noise supressor is loaded
	_, nullsink, err := findModule(c, "module-null-sink", "sink_name=nui_mic_denoised_out")
	if err != nil {
		log.Printf("Couldn't fetch module list to check for module-null-sink: %v\n", err)
	}
	_, ladspasink, err := findModule(c, "module-ladspa-sink", "sink_name=nui_mic_raw_in sink_master=nui_mic_denoised_out")
	if err != nil {
		log.Printf("Couldn't fetch module list to check for module-ladspa-sink: %v\n", err)
	}
	_, loopback, err := findModule(c, "module-loopback", "sink=nui_mic_raw_in")
	if err != nil {
		log.Printf("Couldn't fetch module list to check for module-loopback: %v\n", err)
	}
	_, remap, err := findModule(c, "module-remap-source", "master=nui_mic_denoised_out.monitor source_name=nui_mic_remap")
	if err != nil {
		log.Printf("Couldn't fetch module list to check for module-remap-source: %v\n", err)
	}

	if nullsink && ladspasink && loopback && remap {
		return loaded
	}
	if nullsink || ladspasink || loopback || remap {
		return inconsistent
	}
	return unloaded
}

func loadSupressor(c *pulseaudio.Client, inp input, ui *uistate) error {
	log.Printf("Loading supressor\n")
	idx, err := c.LoadModule("module-null-sink", "sink_name=nui_mic_denoised_out")
	if err != nil {
		return err
	}
	log.Printf("Loaded null sink as idx: %d\n", idx)

	time.Sleep(time.Millisecond * 500) // pulseaudio actually crashes if we send these too fast

	idx, err = c.LoadModule("module-ladspa-sink",
		fmt.Sprintf("sink_name=nui_mic_raw_in sink_master=nui_mic_denoised_out "+
			"label=noise_suppressor_mono plugin=%s control=%d", ui.librnnoise, ui.config.Threshold))
	if err != nil {
		return err
	}
	log.Printf("Loaded ladspa sink as idx: %d\n", idx)

	time.Sleep(time.Millisecond * 1000) // pulseaudio actually crashes if we send these too fast

	idx, err = c.LoadModule("module-loopback",
		fmt.Sprintf("source=%s sink=nui_mic_raw_in channels=1 latency_msec=1", inp.ID))
	if err != nil {
		return err
	}
	log.Printf("Loaded loopback as idx: %d\n", idx)

	time.Sleep(time.Millisecond * 500) // pulseaudio actually crashes if we send these too fast

	idx, err = c.LoadModule("module-remap-source", `master=nui_mic_denoised_out.monitor `+
		`source_name=nui_mic_remap source_properties="device.description='NoiseTorch Microphone'"`)
	if err != nil {
		return err
	}
	log.Printf("Loaded ladspa sink as idx: %d\n", idx)

	return nil
}

func unloadSupressor(c *pulseaudio.Client) error {
	log.Printf("Unloading pulseaudio modules\n")
	log.Printf("Searching for null-sink\n")
	m, found, err := findModule(c, "module-null-sink", "sink_name=nui_mic_denoised_out")
	if err != nil {
		return err
	}
	if found {
		log.Printf("Found null-sink at id [%d], sending unload command\n", m.Index)
		c.UnloadModule(m.Index)
	}
	log.Printf("Searching for ladspa-sink\n")
	m, found, err = findModule(c, "module-ladspa-sink", "sink_name=nui_mic_raw_in sink_master=nui_mic_denoised_out")
	if err != nil {
		return err
	}
	if found {
		log.Printf("Found ladspa-sink at id [%d], sending unload command\n", m.Index)
		c.UnloadModule(m.Index)
	}
	log.Printf("Searching for loopback\n")
	m, found, err = findModule(c, "module-loopback", "sink=nui_mic_raw_in")
	if err != nil {
		return err
	}
	if found {
		log.Printf("Found loopback at id [%d], sending unload command\n", m.Index)
		c.UnloadModule(m.Index)
	}

	log.Printf("Searching for remap-source\n")
	m, found, err = findModule(c, "module-remap-source", "master=nui_mic_denoised_out.monitor source_name=nui_mic_remap")
	if err != nil {
		return err
	}
	if found {
		log.Printf("Found remap source at id [%d], sending unload command\n", m.Index)
		c.UnloadModule(m.Index)
	}
	return nil
}

// Finds a module by exactly matching the module name, and checking if the second string is a substring of the argument
func findModule(c *pulseaudio.Client, name string, argMatch string) (module pulseaudio.Module, found bool, err error) {
	lst, err := c.ModuleList()
	if err != nil {
		return pulseaudio.Module{}, false, err
	}
	for _, m := range lst {
		if m.Name == name && strings.Contains(m.Argument, argMatch) {
			return m, true, nil
		}
	}

	return pulseaudio.Module{}, false, nil
}
