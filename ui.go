package main

import (
	"fmt"
	"image/color"
	"log"
	"time"

	"github.com/aarzilli/nucular"
	"github.com/lawl/pulseaudio"
)

type uistate struct {
	inputList                []input
	noiseSupressorState      int
	paClient                 *pulseaudio.Client
	librnnoise               string
	sourceListColdWidthIndex int
	useBuiltinRNNoise        bool
	config                   *config
	loadingScreen            bool
	masterWindow             *nucular.MasterWindow
}

func updatefn(w *nucular.Window, ui *uistate) {

	if ui.loadingScreen {
		loadingScreen(w, ui)
		return
	}

	w.Row(15).Dynamic(2)
	w.Label("NoiseUI", "LC")

	if ui.noiseSupressorState == loaded {
		w.LabelColored("Denoised virtual microphone active", "RC", color.RGBA{0, 255, 0, 255} /*green*/)
	} else if ui.noiseSupressorState == unloaded {
		w.LabelColored("Denoised virtual microphone inactive", "RC", color.RGBA{255, 0, 0, 255} /*red*/)
	} else if ui.noiseSupressorState == inconsistent {
		w.LabelColored("Inconsistent state, please unload first.", "RC", color.RGBA{255, 140, 0, 255} /*orange*/)
	}

	if w.TreePush(nucular.TreeTab, "Settings", true) {
		w.Row(15).Dynamic(2)
		if w.CheckboxText("Display Monitor Sources", &ui.config.DisplayMonitorSources) {
			ui.sourceListColdWidthIndex++ //recompute the with because of new elements
			go writeConfig(ui.config)
		}

		w.Spacing(1)

		w.Row(25).Ratio(0.5, 0.45, 0.05)
		w.Label("Filter strictness", "LC")
		if w.Input().Mouse.HoveringRect(w.LastWidgetBounds) {
			w.Tooltip("If you have a decent microphone, you can usually turn this all the way up.")
		}
		if w.SliderInt(0, &ui.config.Threshold, 95, 1) {
			go writeConfig(ui.config)
		}
		w.Label(fmt.Sprintf("%d%%", ui.config.Threshold), "RC")

		w.TreePop()
	}
	if w.TreePush(nucular.TreeTab, "Select Device", true) {
		w.Row(15).Dynamic(1)
		w.Label("Select an input device below:", "LC")

		for i := range ui.inputList {
			el := &ui.inputList[i]

			if el.isMonitor && !ui.config.DisplayMonitorSources {
				continue
			}
			w.Row(15).Static()
			w.LayoutFitWidth(0, 0)
			if w.CheckboxText("", &el.checked) {
				ensureOnlyOneInputSelected(&ui.inputList, el)
			}

			w.LayoutFitWidth(ui.sourceListColdWidthIndex, 0)
			w.Label(el.Name, "LC")
		}

		w.Row(30).Dynamic(1)
		w.Spacing(1)

		w.Row(25).Dynamic(2)
		if ui.noiseSupressorState != unloaded {
			if w.ButtonText("Unload Denoised Virtual Microphone") {
				ui.loadingScreen = true
				go func() { // don't block the UI thread, just display a working screen so user can't run multiple loads/unloads
					if err := unloadSupressor(ui.paClient); err != nil {
						log.Println(err)
					}
					//wait until PA reports it has actually loaded it, timeout at 10s
					for i := 0; i < 20; i++ {
						if supressorState(ui.paClient) != unloaded {
							time.Sleep(time.Millisecond * 500)
						}
					}
					ui.loadingScreen = false
					(*ui.masterWindow).Changed()
				}()
			}
		} else {
			w.Spacing(1)
		}
		txt := "Load Denoised Virtual Microphone"
		if ui.noiseSupressorState == loaded {
			txt = "Reload Denoised Virtual Microphone"
		}
		if inp, ok := inputSelection(ui); ok && ui.noiseSupressorState != inconsistent {
			if w.ButtonText(txt) {
				ui.loadingScreen = true
				go func() { // don't block the UI thread, just display a working screen so user can't run multiple loads/unloads
					if ui.noiseSupressorState == loaded {
						if err := unloadSupressor(ui.paClient); err != nil {
							log.Println(err)
						}
					}
					if err := loadSupressor(ui.paClient, inp, ui); err != nil {
						log.Println(err)
					}

					//wait until PA reports it has actually loaded it, timeout at 10s
					for i := 0; i < 20; i++ {
						if supressorState(ui.paClient) != loaded {
							time.Sleep(time.Millisecond * 500)
						}
					}
					ui.loadingScreen = false
					(*ui.masterWindow).Changed()
				}()
			}
		} else {
			w.Spacing(1)
		}
		w.TreePop()
	}
}

func ensureOnlyOneInputSelected(inps *[]input, current *input) {
	if current.checked != true {
		return
	}
	for i := range *inps {
		el := &(*inps)[i]
		el.checked = false
	}
	current.checked = true
}

func inputSelection(ui *uistate) (input, bool) {
	for _, in := range ui.inputList {
		if in.checked {
			return in, true
		}
	}
	return input{}, false
}

func loadingScreen(w *nucular.Window, ui *uistate) {
	w.Row(50).Dynamic(1)
	w.Label("Working...", "CB")
	w.Row(50).Dynamic(1)
	w.Label("(this may take a few seconds)", "CB")
}
