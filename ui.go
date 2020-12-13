package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os/exec"
	"time"

	"github.com/aarzilli/nucular"
	"github.com/aarzilli/nucular/label"
	"github.com/lawl/pulseaudio"
)

type ntcontext struct {
	inputList                []device
	outputList               []device
	noiseSupressorState      int
	paClient                 *pulseaudio.Client
	librnnoise               string
	sourceListColdWidthIndex int
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

var patreonImg *image.RGBA

func updatefn(ctx *ntcontext, w *nucular.Window) {

	if !ctx.paClient.Connected() {
		connectScreen(ctx, w)
		return
	}

	if ctx.loadingScreen {
		loadingScreen(ctx, w)
		return
	}

	if ctx.licenseScreen {
		licenseScreen(ctx, w)
		return
	}

	if ctx.versionScreen {
		versionScreen(ctx, w)
		return
	}

	w.MenubarBegin()

	w.Row(10).Dynamic(1)
	if w := w.Menu(label.TA("About", "LC"), 120, nil); w != nil {
		w.Row(10).Dynamic(1)
		if w.MenuItem(label.T("Licenses")) {
			ctx.licenseScreen = true
		}
		w.Row(10).Dynamic(1)
		if w.MenuItem(label.T("Source code")) {
			exec.Command("xdg-open", "https://github.com/lawl/NoiseTorch").Run()
		}
		if w.MenuItem(label.T("Version")) {
			ctx.versionScreen = true
		}
	}

	w.MenubarEnd()

	w.Row(25).Dynamic(2)
	if patreonImg == nil {
		patreonImg = loadPatreonImg()
	}

	if imageButton(w, patreonImg) {
		exec.Command("xdg-open", "https://patreon.com/lawl").Run()
	}

	if ctx.noiseSupressorState == loaded {
		w.LabelColored("NoiseTorch active", "RC", green)
	} else if ctx.noiseSupressorState == unloaded {
		w.LabelColored("NoiseTorch inactive", "RC", red)
	} else if ctx.noiseSupressorState == inconsistent {
		w.LabelColored("Inconsistent state, please unload first.", "RC", orange)
	}

	if ctx.update.available && !ctx.update.triggered {
		w.Row(20).Ratio(0.9, 0.1)
		w.LabelColored("Update available! Click to install version: "+ctx.update.serverVersion, "LC", green)
		if w.ButtonText("Update") {
			ctx.update.triggered = true
			go update(ctx)
			(*ctx.masterWindow).Changed()
		}
	}

	if ctx.update.triggered {
		w.Row(20).Dynamic(1)
		w.Label(ctx.update.updatingText, "CC")
	}

	if w.TreePush(nucular.TreeTab, "Settings", true) {
		w.Row(15).Dynamic(2)
		if w.CheckboxText("Display Monitor Sources", &ctx.config.DisplayMonitorSources) {
			ctx.sourceListColdWidthIndex++ //recompute the with because of new elements
			go writeConfig(ctx.config)
		}

		w.Spacing(1)

		w.Row(25).Ratio(0.5, 0.45, 0.05)
		w.Label("Voice Activation Threshold", "LC")
		if w.Input().Mouse.HoveringRect(w.LastWidgetBounds) {
			w.Tooltip("If you have a decent microphone, you can usually turn this all the way up.")
		}
		if w.SliderInt(0, &ctx.config.Threshold, 95, 1) {
			go writeConfig(ctx.config)
			ctx.reloadRequired = true
		}
		w.Label(fmt.Sprintf("%d%%", ctx.config.Threshold), "RC")

		if ctx.reloadRequired {
			w.Row(20).Dynamic(1)
			w.LabelColored("Reloading NoiseTorch is required to apply these changes.", "LC", orange)
		}

		w.Row(15).Dynamic(2)
		if w.CheckboxText("Filter Microphone", &ctx.config.FilterInput) {
			ctx.sourceListColdWidthIndex++ //recompute the with because of new elements
			go writeConfig(ctx.config)
			go (func() { ctx.noiseSupressorState = supressorState(ctx) })()
		}

		if w.CheckboxText("Filter Headphones", &ctx.config.FilterOutput) {
			ctx.sourceListColdWidthIndex++ //recompute the with because of new elements
			go writeConfig(ctx.config)
			go (func() { ctx.noiseSupressorState = supressorState(ctx) })()
		}

		w.TreePop()
	}
	if ctx.config.FilterInput && w.TreePush(nucular.TreeTab, "Select Microphone", true) {
		w.Row(15).Dynamic(1)
		w.Label("Select an input device below:", "LC")

		for i := range ctx.inputList {
			el := &ctx.inputList[i]

			if el.isMonitor && !ctx.config.DisplayMonitorSources {
				continue
			}
			w.Row(15).Static()
			w.LayoutFitWidth(0, 0)
			if w.CheckboxText("", &el.checked) {
				ensureOnlyOneInputSelected(&ctx.inputList, el)
			}

			w.LayoutFitWidth(ctx.sourceListColdWidthIndex, 0)
			if el.dynamicLatency {
				w.Label(el.Name, "LC")
			} else {
				w.LabelColored("(incompatible?) "+el.Name, "LC", orange)
			}
		}

		w.TreePop()
	}

	if ctx.config.FilterOutput && w.TreePush(nucular.TreeTab, "Select Headphones", true) {
		if ctx.config.GuiltTripped {
			w.Row(15).Dynamic(1)
			w.Label("Select an output device below:", "LC")

			for i := range ctx.outputList {
				el := &ctx.outputList[i]

				if el.isMonitor && !ctx.config.DisplayMonitorSources {
					continue
				}
				w.Row(15).Static()
				w.LayoutFitWidth(0, 0)
				if w.CheckboxText("", &el.checked) {
					ensureOnlyOneInputSelected(&ctx.outputList, el)
				}

				w.LayoutFitWidth(ctx.sourceListColdWidthIndex, 0)
				if el.dynamicLatency {
					w.Label(el.Name, "LC")
				} else {
					w.LabelColored("(incompatible?) "+el.Name, "LC", orange)
				}

			}
		} else {
			w.Row(15).Dynamic(1)
			w.Label("This feature is only for patrons.", "LC")
			w.Row(15).Dynamic(1)
			w.Label("You can still use it eitherway, but you are legally required to feel bad.", "LC")
			w.Row(25).Dynamic(2)
			if w.ButtonText("Become a patron") {
				exec.Command("xdg-open", "https://patreon.com/lawl").Run()
				ctx.config.GuiltTripped = true
				go writeConfig(ctx.config)
			}

			if w.ButtonText("Feel bad") {
				ctx.config.GuiltTripped = true
				go writeConfig(ctx.config)
			}
		}

		w.TreePop()
	}

	w.Row(15).Dynamic(1)
	w.Spacing(1)

	w.Row(25).Dynamic(2)
	if ctx.noiseSupressorState != unloaded {
		if w.ButtonText("Unload NoiseTorch") {
			ctx.loadingScreen = true
			ctx.reloadRequired = false
			go func() { // don't block the UI thread, just display a working screen so user can't run multiple loads/unloads
				if err := unloadSupressor(ctx); err != nil {
					log.Println(err)
				}
				//wait until PA reports it has actually loaded it, timeout at 10s
				for i := 0; i < 20; i++ {
					if supressorState(ctx) != unloaded {
						time.Sleep(time.Millisecond * 500)
					}
				}
				ctx.loadingScreen = false
				(*ctx.masterWindow).Changed()
			}()
		}
	} else {
		w.Spacing(1)
	}
	txt := "Load NoiseTorch"
	if ctx.noiseSupressorState == loaded {
		txt = "Reload NoiseTorch"
	}

	inp, inpOk := inputSelection(ctx)
	out, outOk := outputSelection(ctx)
	if (!ctx.config.FilterInput || (ctx.config.FilterInput && inpOk)) &&
		(!ctx.config.FilterOutput || (ctx.config.FilterOutput && outOk)) &&
		(ctx.config.FilterInput || ctx.config.FilterOutput) &&
		((ctx.config.FilterOutput && ctx.config.GuiltTripped) || !ctx.config.FilterOutput) &&
		ctx.noiseSupressorState != inconsistent {
		if w.ButtonText(txt) {
			ctx.loadingScreen = true
			ctx.reloadRequired = false
			go func() { // don't block the UI thread, just display a working screen so user can't run multiple loads/unloads
				if ctx.noiseSupressorState == loaded {
					if err := unloadSupressor(ctx); err != nil {
						log.Println(err)
					}
				}
				if err := loadSupressor(ctx, &inp, &out); err != nil {
					log.Println(err)
				}

				//wait until PA reports it has actually loaded it, timeout at 10s
				for i := 0; i < 20; i++ {
					if supressorState(ctx) != loaded {
						time.Sleep(time.Millisecond * 500)
					}
				}
				ctx.config.LastUsedInput = inp.ID
				ctx.config.LastUsedOutput = out.ID
				go writeConfig(ctx.config)
				ctx.loadingScreen = false
				(*ctx.masterWindow).Changed()
			}()
		}
	} else {
		w.Spacing(1)
	}

}

func ensureOnlyOneInputSelected(inps *[]device, current *device) {
	if current.checked != true {
		return
	}
	for i := range *inps {
		el := &(*inps)[i]
		el.checked = false
	}
	current.checked = true
}

func inputSelection(ctx *ntcontext) (device, bool) {
	if !ctx.config.FilterInput {
		return device{}, false
	}

	for _, in := range ctx.inputList {
		if in.checked {
			return in, true
		}
	}
	return device{}, false
}

func outputSelection(ctx *ntcontext) (device, bool) {
	if !ctx.config.FilterOutput {
		return device{}, false
	}

	for _, out := range ctx.outputList {
		if out.checked {
			return out, true
		}
	}
	return device{}, false
}

func loadingScreen(ctx *ntcontext, w *nucular.Window) {
	w.Row(50).Dynamic(1)
	w.Label("Working...", "CB")
	w.Row(50).Dynamic(1)
	w.Label("(this may take a few seconds)", "CB")
}

func licenseScreen(ctx *ntcontext, w *nucular.Window) {
	w.Row(255).Dynamic(1)
	field := &ctx.licenseTextArea
	field.Flags |= nucular.EditMultiline
	if len(field.Buffer) < 1 {
		field.Buffer = []rune(licenseString)
	}
	field.Edit(w)

	w.Row(20).Dynamic(2)
	w.Spacing(1)
	if w.ButtonText("OK") {
		ctx.licenseScreen = false
	}
}

func versionScreen(ctx *ntcontext, w *nucular.Window) {
	w.Row(50).Dynamic(1)
	w.Label("Version", "CB")
	w.Row(50).Dynamic(1)
	w.Label(version, "CB")
	w.Row(50).Dynamic(1)
	w.Spacing(1)
	w.Row(20).Dynamic(2)
	w.Spacing(1)
	if w.ButtonText("OK") {
		ctx.versionScreen = false
	}
}

func connectScreen(ctx *ntcontext, w *nucular.Window) {
	w.Row(50).Dynamic(1)
	w.Label("Connecting to pulseaudio...", "CB")
}

func resetUI(ctx *ntcontext) {
	ctx.loadingScreen = false

	if ctx.masterWindow != nil {
		(*ctx.masterWindow).Changed()
	}
}

func loadPatreonImg() *image.RGBA {
	var pat *image.RGBA
	img, _ := png.Decode(bytes.NewReader(patreonPNG))
	pat = image.NewRGBA(img.Bounds())
	draw.Draw(pat, img.Bounds(), img, image.Point{}, draw.Src)
	return pat
}

func imageButton(w *nucular.Window, img *image.RGBA) bool {
	style := w.Master().Style()
	origButtonStyle := style.Button
	style.Button.Border = 0
	style.Button.Normal.Data.Color = style.NormalWindow.Background
	style.Button.Hover.Data.Color = style.NormalWindow.Background
	style.Button.Active.Data.Color = style.NormalWindow.Background

	defer (func() { style.Button = origButtonStyle })()

	return w.Button(label.I(patreonImg), false)

}
