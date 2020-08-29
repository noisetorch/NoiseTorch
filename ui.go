package main

import (
	"fmt"
	"image/color"
	"log"
	"os/exec"
	"time"

	"github.com/aarzilli/nucular"
	"github.com/aarzilli/nucular/label"
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
	licenseScreen            bool
	versionScreen            bool
	licenseTextArea          nucular.TextEditor
	masterWindow             *nucular.MasterWindow
	update                   updateui
	reloadRequired           bool
}

var green = color.RGBA{34, 187, 69, 255}
var red = color.RGBA{255, 70, 70, 255}
var orange = color.RGBA{255, 140, 0, 255}

func updatefn(w *nucular.Window, ui *uistate) {

	if !ui.paClient.Connected() {
		connectScreen(w, ui)
		return
	}

	if ui.loadingScreen {
		loadingScreen(w, ui)
		return
	}

	if ui.licenseScreen {
		licenseScreen(w, ui)
		return
	}

	if ui.versionScreen {
		versionScreen(w, ui)
		return
	}

	w.MenubarBegin()

	w.Row(10).Dynamic(2)
	if w := w.Menu(label.TA("About", "LC"), 120, nil); w != nil {
		w.Row(10).Dynamic(1)
		if w.MenuItem(label.T("Licenses")) {
			ui.licenseScreen = true
		}
		w.Row(10).Dynamic(1)
		if w.MenuItem(label.T("Source code")) {
			exec.Command("xdg-open", "https://github.com/lawl/NoiseTorch").Run()
		}
		if w.MenuItem(label.T("Version")) {
			ui.versionScreen = true
		}
	}
	w.MenubarEnd()

	w.Row(15).Dynamic(1)

	if ui.noiseSupressorState == loaded {
		w.LabelColored("NoiseTorch active", "RC", green)
	} else if ui.noiseSupressorState == unloaded {
		w.LabelColored("NoiseTorch inactive", "RC", red)
	} else if ui.noiseSupressorState == inconsistent {
		w.LabelColored("Inconsistent state, please unload first.", "RC", orange)
	}

	if ui.update.available && !ui.update.triggered {
		w.Row(20).Ratio(0.9, 0.1)
		w.LabelColored("Update available! Click to install version: "+ui.update.serverVersion, "LC", green)
		if w.ButtonText("Update") {
			ui.update.triggered = true
			go update(ui)
			(*ui.masterWindow).Changed()
		}
	}

	if ui.update.triggered {
		w.Row(20).Dynamic(1)
		w.Label(ui.update.updatingText, "CC")
	}

	if w.TreePush(nucular.TreeTab, "Settings", true) {
		w.Row(15).Dynamic(2)
		if w.CheckboxText("Display Monitor Sources", &ui.config.DisplayMonitorSources) {
			ui.sourceListColdWidthIndex++ //recompute the with because of new elements
			go writeConfig(ui.config)
		}

		w.Spacing(1)

		w.Row(25).Ratio(0.5, 0.45, 0.05)
		w.Label("Voice Activation Threshold", "LC")
		if w.Input().Mouse.HoveringRect(w.LastWidgetBounds) {
			w.Tooltip("If you have a decent microphone, you can usually turn this all the way up.")
		}
		if w.SliderInt(0, &ui.config.Threshold, 95, 1) {
			go writeConfig(ui.config)
			ui.reloadRequired = true
		}
		w.Label(fmt.Sprintf("%d%%", ui.config.Threshold), "RC")

		if ui.reloadRequired {
			w.Row(20).Dynamic(1)
			w.LabelColored("Reloading NoiseTorch is required to apply these changes.", "LC", orange)
		}

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
			if el.dynamicLatency {
				w.Label(el.Name, "LC")
			} else {
				w.LabelColored("(incompatible?) "+el.Name, "LC", orange)
			}
		}

		w.Row(30).Dynamic(1)
		w.Spacing(1)

		w.Row(25).Dynamic(2)
		if ui.noiseSupressorState != unloaded {
			if w.ButtonText("Unload NoiseTorch") {
				ui.loadingScreen = true
				ui.reloadRequired = false
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
		txt := "Load NoiseTorch"
		if ui.noiseSupressorState == loaded {
			txt = "Reload NoiseTorch"
		}
		if inp, ok := inputSelection(ui); ok && ui.noiseSupressorState != inconsistent {
			if w.ButtonText(txt) {
				ui.loadingScreen = true
				ui.reloadRequired = false
				go func() { // don't block the UI thread, just display a working screen so user can't run multiple loads/unloads
					if ui.noiseSupressorState == loaded {
						if err := unloadSupressor(ui.paClient); err != nil {
							log.Println(err)
						}
					}
					if err := loadSupressor(ui.paClient, inp, ui, false); err != nil {
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

func licenseScreen(w *nucular.Window, ui *uistate) {
	w.Row(255).Dynamic(1)
	field := &ui.licenseTextArea
	field.Flags |= nucular.EditMultiline
	if len(field.Buffer) < 1 {
		field.Buffer = []rune(licenseString)
	}
	field.Edit(w)

	w.Row(20).Dynamic(2)
	w.Spacing(1)
	if w.ButtonText("OK") {
		ui.licenseScreen = false
	}
}

func versionScreen(w *nucular.Window, ui *uistate) {
	w.Row(50).Dynamic(1)
	w.Label("Version", "CB")
	w.Row(50).Dynamic(1)
	w.Label(version, "CB")
	w.Row(50).Dynamic(1)
	w.Spacing(1)
	w.Row(20).Dynamic(2)
	w.Spacing(1)
	if w.ButtonText("OK") {
		ui.versionScreen = false
	}
}

func connectScreen(w *nucular.Window, ui *uistate) {
	w.Row(50).Dynamic(1)
	w.Label("Connecting to pulseaudio...", "CB")
}

func resetUI(ui *uistate) {
	ui.loadingScreen = false

	if ui.masterWindow != nil {
		(*ui.masterWindow).Changed()
	}
}
