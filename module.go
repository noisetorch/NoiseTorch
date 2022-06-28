package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/noisetorch/pulseaudio"
)

const (
	loaded = iota
	unloaded
	inconsistent
)

// the ugly and (partially) repeated strings are unforunately difficult to avoid, as it's what pulse audio expects

func updateNoiseSupressorLoaded(ctx *ntcontext) {
	c := ctx.paClient
	upd, err := c.Updates()
	if err != nil {
		fmt.Printf("Error listening for updates: %v\n", err)
	}

	for {
		ctx.noiseSupressorState, ctx.virtualDeviceInUse = supressorState(ctx)
		if !c.Connected() {
			break
		}

		<-upd
	}
}

func supressorState(ctx *ntcontext) (int, bool) {
	//perform some checks to see if it looks like the noise supressor is loaded
	c := ctx.paClient
	var inpLoaded, outLoaded, inputInc, outputInc bool
	var virtualDeviceInUse bool = false
	if ctx.config.FilterInput {
		if ctx.serverInfo.servertype == servertype_pipewire {
			module, ladspasource, err := findModule(c, "module-ladspa-source", "source_name='Filtered Microphone")
			if err != nil {
				log.Printf("Couldn't fetch module list to check for module-ladspa-source: %v\n", err)
			}
			virtualDeviceInUse = virtualDeviceInUse || (module.NUsed != 0)
			inpLoaded = ladspasource
			inputInc = false
		} else {
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
			module, remap, err := findModule(c, "module-remap-source", "master=nui_mic_denoised_out.monitor source_name=nui_mic_remap")
			if err != nil {
				log.Printf("Couldn't fetch module list to check for module-remap-source: %v\n", err)
			}

			virtualDeviceInUse = virtualDeviceInUse || (module.NUsed != 0)

			if nullsink && ladspasink && loopback && remap {
				inpLoaded = true
			} else if nullsink || ladspasink || loopback || remap {
				inputInc = true
			}
		}
	} else {
		inpLoaded = true
	}

	if ctx.config.FilterOutput {
		if ctx.serverInfo.servertype == servertype_pipewire {
			module, ladspasink, err := findModule(c, "module-ladspa-sink", "sink_name='Filtered Headphones'")
			if err != nil {
				log.Printf("Couldn't fetch module list to check for module-ladspa-sink: %v\n", err)
			}
			virtualDeviceInUse = virtualDeviceInUse || (module.NUsed != 0)
			outLoaded = ladspasink
			outputInc = false
		} else {
			_, out, err := findModule(c, "module-null-sink", "sink_name=nui_out_out_sink")
			if err != nil {
				log.Printf("Couldn't fetch module list to check for output module-ladspa-sink: %v\n", err)
			}
			_, lad, err := findModule(c, "module-ladspa-sink", "sink_name=nui_out_ladspa")
			if err != nil {
				log.Printf("Couldn't fetch module list to check for output module-ladspa-sink: %v\n", err)
			}
			_, loop, err := findModule(c, "module-loopback", "source=nui_out_out_sink.monitor")
			if err != nil {
				log.Printf("Couldn't fetch module list to check for output module-ladspa-sink: %v\n", err)
			}
			module, outin, err := findModule(c, "module-null-sink", "sink_name=nui_out_in_sink")
			if err != nil {
				log.Printf("Couldn't fetch module list to check for output module-ladspa-sink: %v\n", err)
			}
			virtualDeviceInUse = virtualDeviceInUse || (module.NUsed != 0)
			_, loop2, err := findModule(c, "module-loopback", "source=nui_out_in_sink.monitor")
			if err != nil {
				log.Printf("Couldn't fetch module list to check for output module-ladspa-sink: %v\n", err)
			}

			outLoaded = out && lad && loop && outin && loop2
			outputInc = out || lad || loop || outin || loop2
		}
	} else {
		outLoaded = true
	}

	if (inpLoaded || !ctx.config.FilterInput) && (outLoaded || !ctx.config.FilterOutput) && !inputInc {
		return loaded, virtualDeviceInUse
	}

	if (inpLoaded && ctx.config.FilterInput) || (outLoaded && ctx.config.FilterOutput) || inputInc || outputInc {
		return inconsistent, virtualDeviceInUse
	}

	return unloaded, virtualDeviceInUse
}

func loadSupressor(ctx *ntcontext, inp *device, out *device) error {
	if ctx.serverInfo.servertype == servertype_pulse {
		log.Printf("Querying pulse rlimit\n")
		pid, err := getPulsePid()
		if err != nil {
			return err
		}

		lim, err := getRlimit(pid)
		if err != nil {
			return err
		}
		log.Printf("Rlimit: %+v. Trying to remove.\n", lim)

		removeRlimit(pid)

		defer setRlimit(pid, &lim) // lowering RLIMIT doesn't require root

		newLim, err := getRlimit(pid)
		if err != nil {
			return err
		}
		log.Printf("Rlimit: %+v\n", newLim)
	}

	if inp.checked {
		var err error
		if ctx.serverInfo.servertype == servertype_pipewire {
			err = loadPipeWireInput(ctx, inp)
		} else {
			err = loadPulseInput(ctx, inp)
		}
		if err != nil {
			log.Printf("Error loading input: %v\n", err)
			return err
		}
	}

	if out.checked {
		var err error
		if ctx.serverInfo.servertype == servertype_pipewire {
			err = loadPipeWireOutput(ctx, out)
		} else {
			err = loadPulseOutput(ctx, out)
		}
		if err != nil {
			log.Printf("Error loading output: %v\n", err)
			return err
		}
	}

	return nil
}

func loadModule(ctx *ntcontext, module, args string) (uint32, error) {
	idx, err := ctx.paClient.LoadModule(module, args)

	//14 = module initialisation failed
	if paErr, ok := err.(*pulseaudio.Error); ok && paErr.Code == 14 {
		resetUI(ctx)
		ctx.views.Push(makeErrorView(ctx, fmt.Sprintf("Could not load module '%s'. This is likely a problem with your system or distribution.", module)))
	}
	return idx, err
}

func loadPipeWireInput(ctx *ntcontext, inp *device) error {
	log.Printf("Loading supressor for pipewire\n")
	idx, err := loadModule(ctx, "module-ladspa-source",
		fmt.Sprintf("source_name='Filtered Microphone for %s' master=%s "+
			"rate=48000 channels=1 "+
			"label=changemelater plugin=%s control=%d", inp.Name, inp.ID, ctx.librnnoise, ctx.config.Threshold))

	if err != nil {
		return err
	}
	log.Printf("Loaded ladspa source as idx: %d\n", idx)
	return nil
}

func loadPipeWireOutput(ctx *ntcontext, out *device) error {
	log.Printf("Loading supressor for pipewire\n")
	idx, err := loadModule(ctx, "module-ladspa-sink",
		fmt.Sprintf("sink_name='Filtered Headphones' master=%s "+
			"rate=48000 channels=1 "+
			"label=changemelater plugin=%s control=%d", out.ID, ctx.librnnoise, ctx.config.Threshold))

	if err != nil {
		return err
	}
	log.Printf("Loaded ladspa source as idx: %d\n", idx)
	return nil
}

func loadPulseInput(ctx *ntcontext, inp *device) error {
	log.Printf("Loading supressor for pulse\n")
	idx, err := loadModule(ctx, "module-null-sink", "sink_name=nui_mic_denoised_out rate=48000")
	if err != nil {
		return err
	}
	log.Printf("Loaded null sink as idx: %d\n", idx)

	idx, err = loadModule(ctx, "module-ladspa-sink",
		fmt.Sprintf("sink_name=nui_mic_raw_in sink_master=nui_mic_denoised_out "+
			"label=changemelater plugin=%s control=%d", ctx.librnnoise, ctx.config.Threshold))
	if err != nil {
		return err
	}
	log.Printf("Loaded ladspa sink as idx: %d\n", idx)

	if inp.dynamicLatency {
		idx, err = loadModule(ctx, "module-loopback",
			fmt.Sprintf("source=%s sink=nui_mic_raw_in channels=1 latency_msec=1 source_dont_move=true sink_dont_move=true", inp.ID))
		if err != nil {
			return err
		}
		log.Printf("Loaded loopback as idx: %d\n", idx)
	} else {
		idx, err = loadModule(ctx, "module-loopback",
			fmt.Sprintf("source=%s sink=nui_mic_raw_in channels=1 latency_msec=50 source_dont_move=true sink_dont_move=true adjust_time=1", inp.ID))
		if err != nil {
			return err
		}
		log.Printf("Loaded fixed latency loopback as idx: %d\n", idx)
	}

	idx, err = loadModule(ctx, "module-remap-source", fmt.Sprintf(`master=nui_mic_denoised_out.monitor `+
		`source_name=nui_mic_remap source_properties="device.description='Filtered Microphone for %s'"`, inp.Name))
	if err != nil {
		return err
	}
	log.Printf("Loaded remap source as idx: %d\n", idx)
	return nil
}

func loadPulseOutput(ctx *ntcontext, out *device) error {
	_, err := loadModule(ctx, "module-null-sink", `sink_name=nui_out_out_sink`)
	if err != nil {
		return err
	}

	_, err = loadModule(ctx, "module-null-sink", `sink_name=nui_out_in_sink sink_properties="device.description='Filtered Headphones'"`)
	if err != nil {
		return err
	}

	_, err = loadModule(ctx, "module-ladspa-sink", fmt.Sprintf(`sink_name=nui_out_ladspa sink_master=nui_out_out_sink `+
		`label=changemelater channels=1 plugin=%s control=%d rate=%d`,
		ctx.librnnoise, ctx.config.Threshold, 48000))
	if err != nil {
		return err
	}

	_, err = loadModule(ctx, "module-loopback",
		fmt.Sprintf("source=nui_out_out_sink.monitor sink=%s channels=2 latency_msec=50 source_dont_move=true sink_dont_move=true", out.ID))
	if err != nil {
		return err
	}

	_, err = loadModule(ctx, "module-loopback",
		fmt.Sprintf("source=nui_out_in_sink.monitor sink=nui_out_ladspa channels=1 latency_msec=50 source_dont_move=true sink_dont_move=true"))
	if err != nil {
		return err
	}
	return nil
}

func unloadSupressor(ctx *ntcontext) error {
	if ctx.serverInfo.servertype == servertype_pipewire {
		return unloadSupressorPipeWire(ctx)
	} else {
		return unloadSupressorPulse(ctx)
	}
}

func unloadSupressorPipeWire(ctx *ntcontext) error {
	log.Printf("Unloading modules for pipewire\n")

	log.Printf("Searching for module-ladspa-source\n")
	c := ctx.paClient
	m, found, err := findModule(c, "module-ladspa-source", "source_name='Filtered Microphone")
	if err != nil {
		return err
	}
	if found {
		log.Printf("Found module-ladspa-source at id [%d], sending unload command\n", m.Index)
		c.UnloadModule(m.Index)
	}

	log.Printf("Searching for module-ladspa-sink\n")
	m, found, err = findModule(c, "module-ladspa-sink", "sink_name='Filtered Headphones'")
	if err != nil {
		return err
	}
	if found {
		log.Printf("Found module-ladspa-sink at id [%d], sending unload command\n", m.Index)
		c.UnloadModule(m.Index)
	}
	return nil
}

func unloadSupressorPulse(ctx *ntcontext) error {
	log.Printf("Unloading modules for pulseaudio\n")

	if pid, err := getPulsePid(); err == nil {
		if lim, err := getRlimit(pid); err == nil {
			log.Printf("Trying to remove rlimit. Limit is: %+v\n", lim)
			removeRlimit(pid)
			newLim, _ := getRlimit(pid)
			log.Printf("Rlimit: %+v\n", newLim)
			defer setRlimit(pid, &lim)
		}

	}

	log.Printf("Searching for null-sink\n")
	c := ctx.paClient
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

	log.Printf("Searching for output module-null-sink\n")
	m, found, err = findModule(c, "module-null-sink", "sink_name=nui_out_out_sink")
	if err != nil {
		return err
	}
	if found {
		log.Printf("Found output null sink at id [%d], sending unload command\n", m.Index)
		c.UnloadModule(m.Index)
	}

	log.Printf("Searching for output module-null-sink\n")
	m, found, err = findModule(c, "module-null-sink", "sink_name=nui_out_in_sink")
	if err != nil {
		return err
	}
	if found {
		log.Printf("Found output null sink at id [%d], sending unload command\n", m.Index)
		c.UnloadModule(m.Index)
	}

	log.Printf("Searching for output module-ladspa-sink\n")
	m, found, err = findModule(c, "module-ladspa-sink", "sink_name=nui_out_ladspa")
	if err != nil {
		return err
	}
	if found {
		log.Printf("Found output ladspa sink at id [%d], sending unload command\n", m.Index)
		c.UnloadModule(m.Index)
	}

	log.Printf("Searching for output module-loopback\n")
	m, found, err = findModule(c, "module-loopback", "source=nui_out_out_sink.monitor")
	if err != nil {
		return err
	}
	if found {
		log.Printf("Found output loopback at id [%d], sending unload command\n", m.Index)
		c.UnloadModule(m.Index)
	}

	log.Printf("Searching for output module-loopback\n")
	m, found, err = findModule(c, "module-loopback", "source=nui_out_in_sink.monitor")
	if err != nil {
		return err
	}
	if found {
		log.Printf("Found output loopback at id [%d], sending unload command\n", m.Index)
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
