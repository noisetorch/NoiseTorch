// This file is part of the program "NoiseTorch-ng".
// Please see the LICENSE file for copyright information.

package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/aarzilli/nucular"
	"github.com/aarzilli/nucular/label"
	"github.com/noisetorch/pulseaudio"
)

type ntcontext struct {
	inputList                []device
	outputList               []device
	noiseSupressorState      int
	paClient                 *pulseaudio.Client
	librnnoise               string
	sourceListColdWidthIndex int
	config                   *config
	licenseTextArea          nucular.TextEditor
	masterWindow             *nucular.MasterWindow
	update                   updateui
	reloadRequired           bool
	haveCapabilities         bool
	capsMismatch             bool
	views                    *ViewStack
	serverInfo               audioserverinfo
	virtualDeviceInUse       bool
}

//TODO pull some of these strucs out of UI, they don't belong here
type audioserverinfo struct {
	servertype       uint
	name             string
	major            int
	minor            int
	patch            int
	outdatedPipeWire bool
}

const (
	servertype_pulse = iota
	servertype_pipewire
)

var green = color.RGBA{34, 187, 69, 255}
var red = color.RGBA{255, 70, 70, 255}
var orange = color.RGBA{255, 140, 0, 255}
var lightBlue = color.RGBA{173, 216, 230, 255}

const notice = "NoiseTorch Next Gen (stylized NoiseTorch-ng) is a continuation of the NoiseTorch\nproject after it was abandoned by its original author. Please do not confuse\nboth programs. You may convey modified versions of this program under its name."

func updatefn(ctx *ntcontext, w *nucular.Window) {
	currView := ctx.views.Peek()
	currView(ctx, w)
}

func mainView(ctx *ntcontext, w *nucular.Window) {

	w.MenubarBegin()

	w.Row(10).Dynamic(1)
	if w := w.Menu(label.TA("About", "LC"), 120, nil); w != nil {
		w.Row(10).Dynamic(1)
		if w.MenuItem(label.T("Licenses")) {
			ctx.views.Push(licenseView)
		}
		w.Row(10).Dynamic(1)
		if w.MenuItem(label.T("Website")) {
			exec.Command("xdg-open", websiteURL).Run()
		}
		if w.MenuItem(label.T("Version")) {
			ctx.views.Push(versionView)
		}
	}

	w.MenubarEnd()

	w.Row(15).Dynamic(1)

	if ctx.noiseSupressorState == loaded {
		if ctx.virtualDeviceInUse {
			w.LabelColored("Filtering active", "RC", green)
		} else {
			w.LabelColored("Filtering unconfigured", "RC", lightBlue)
		}
	} else if ctx.noiseSupressorState == unloaded {
		_, inpOk := inputSelection(ctx)
		_, outOk := outputSelection(ctx)
		if validConfiguration(ctx, inpOk, outOk) {
			w.LabelColored("Filtering inactive", "RC", red)
		} else {
			w.LabelColored("Filtering unconfigured", "RC", lightBlue)
		}
	} else if ctx.noiseSupressorState == inconsistent {
		w.LabelColored("Inconsistent state, please unload first.", "RC", orange)
	}

	if ctx.serverInfo.servertype == servertype_pipewire {
		w.Row(20).Dynamic(1)
		w.Label("Running in PipeWire mode. PipeWire support is currently alpha quality. Please report bugs.", "LC")
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
			w.LabelColored("Reloading the filter(s) is required to apply these changes.", "LC", orange)
		}

		w.Row(15).Dynamic(2)
		if w.CheckboxText("Filter Microphone", &ctx.config.FilterInput) {
			ctx.sourceListColdWidthIndex++ //recompute the with because of new elements
			go writeConfig(ctx.config)
			go (func() { ctx.noiseSupressorState, _ = supressorState(ctx) })()
		}

		if w.CheckboxText("Filter Headphones", &ctx.config.FilterOutput) {
			ctx.sourceListColdWidthIndex++ //recompute the with because of new elements
			go writeConfig(ctx.config)
			go (func() { ctx.noiseSupressorState, _ = supressorState(ctx) })()
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
			if w.OptionText("", el.checked) {
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
		w.Row(15).Dynamic(1)
		w.Label("Select an output device below:", "LC")

		for i := range ctx.outputList {
			el := &ctx.outputList[i]

			if el.isMonitor && !ctx.config.DisplayMonitorSources {
				continue
			}
			w.Row(15).Static()
			w.LayoutFitWidth(0, 0)
			if w.OptionText("", el.checked) {
				ensureOnlyOneInputSelected(&ctx.outputList, el)
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

	w.Row(15).Dynamic(1)
	w.Spacing(1)

	w.Row(25).Dynamic(2)
	if ctx.noiseSupressorState != unloaded {
		if w.ButtonText("Unload Filter(s)") {
			ctx.reloadRequired = false
			if ctx.virtualDeviceInUse {
				confirm := makeConfirmView(ctx,
					"Virtual Device in Use",
					"Some applications may behave weirdly when you remove a device they're currently using",
					"Unload",
					"Go back",
					func() { uiUnloadFilters(ctx) },
					func() {})
				ctx.views.Push(confirm)
			} else {
				go uiUnloadFilters(ctx)
			}
		}
	} else {
		w.Spacing(1)
	}
	txt := "Load Filter(s)"
	if ctx.noiseSupressorState == loaded {
		txt = "Reload Filter(s)"
	}

	inp, inpOk := inputSelection(ctx)
	out, outOk := outputSelection(ctx)
	if validConfiguration(ctx, inpOk, outOk) {
		if w.ButtonText(txt) {
			ctx.reloadRequired = false

			if ctx.virtualDeviceInUse {
				confirm := makeConfirmView(ctx,
					"Virtual Device in Use",
					"Some applications may behave weirdly when you reload a device they're currently using",
					"Reload",
					"Go back",
					func() { uiReloadFilters(ctx, inp, out) },
					func() {})
				ctx.views.Push(confirm)
			} else {
				go uiReloadFilters(ctx, inp, out)
			}
		}
	} else {
		w.Spacing(1)
	}

}

func uiUnloadFilters(ctx *ntcontext) {
	ctx.views.Push(loadingView)
	if err := unloadSupressor(ctx); err != nil {
		log.Println(err)
	}
	//wait until PA reports it has actually loaded it, timeout at 10s
	for i := 0; i < 20; i++ {
		if state, _ := supressorState(ctx); state != unloaded {
			time.Sleep(time.Millisecond * 500)
		}
	}
	ctx.views.Pop()
	(*ctx.masterWindow).Changed()
}

func uiReloadFilters(ctx *ntcontext, inp, out device) {
	ctx.views.Push(loadingView)
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
		if state, _ := supressorState(ctx); state != loaded {
			time.Sleep(time.Millisecond * 500)
		}
	}
	ctx.config.LastUsedInput = inp.ID
	ctx.config.LastUsedOutput = out.ID
	go writeConfig(ctx.config)
	ctx.views.Pop()
	(*ctx.masterWindow).Changed()
}

func ensureOnlyOneInputSelected(inps *[]device, current *device) {
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
		if in.checked && (!in.isMonitor || ctx.config.DisplayMonitorSources) {
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

func validConfiguration(ctx *ntcontext, inpOk bool, outOk bool) bool {
	return (!ctx.config.FilterInput || (ctx.config.FilterInput && inpOk)) &&
		(!ctx.config.FilterOutput || (ctx.config.FilterOutput && outOk)) &&
		(ctx.config.FilterInput || ctx.config.FilterOutput) &&
		ctx.noiseSupressorState != inconsistent
}

func loadingView(ctx *ntcontext, w *nucular.Window) {
	w.Row(50).Dynamic(1)
	w.Label("Working...", "CB")
	w.Row(50).Dynamic(1)
	w.Label("(this may take a few seconds)", "CB")
}

func licenseView(ctx *ntcontext, w *nucular.Window) {
	w.Row(40).Dynamic(1) // space above notice
	w.Label(notice, "CB")
	w.Row(40).Dynamic(1)  // space below notice
	w.Row(255).Dynamic(1) // space for license text area
	field := &ctx.licenseTextArea
	field.Flags |= nucular.EditMultiline
	if len(field.Buffer) < 1 {
		field.Buffer = []rune(licenseString) // nolint
	}
	field.Edit(w)

	w.Row(20).Dynamic(2)
	w.Spacing(1)
	if w.ButtonText("OK") {
		ctx.views.Pop()
	}
}

func versionView(ctx *ntcontext, w *nucular.Window) {
	w.Row(50).Dynamic(1)
	w.Label("Version", "CB")
	w.Row(50).Dynamic(1)
	w.Label(notice, "CB")
	w.Row(50).Dynamic(1)
	w.Label(fmt.Sprintf("%s (%s)", version, distribution), "CB")
	w.Row(50).Dynamic(1)
	w.Spacing(1)
	w.Row(20).Dynamic(2)
	w.Spacing(1)
	if w.ButtonText("OK") {
		ctx.views.Pop()
	}
}

func connectView(ctx *ntcontext, w *nucular.Window) {
	w.Row(50).Dynamic(1)
	w.Label("Connecting to pulseaudio...", "CB")
}

func capabilitiesView(ctx *ntcontext, w *nucular.Window) {
	w.Row(15).Dynamic(1)
	w.Label("This program does not have the capabilities to function properly.", "CB")
	w.Row(15).Dynamic(1)
	w.Label("We require CAP_SYS_RESOURCE. If that doesn't mean anything to you, don't worry. I'll fix it for you.", "CB")
	if ctx.capsMismatch {
		w.Row(15).Dynamic(1)
		w.LabelColored("Warning: File has CAP_SYS_RESOURCE but our process doesn't.", "CB", orange)
		w.Row(15).Dynamic(1)
		w.LabelColored("Check if your filesystem has nosuid set or check the troubleshooting page.", "CB", orange)
	}
	w.Row(40).Dynamic(1)
	w.Row(25).Dynamic(1)
	if w.ButtonText("Grant capability (requires root)") {
		err := pkexecSetcapSelf()
		if err != nil {
			ctx.views.Push(makeErrorView(ctx, err.Error()))
			return
		}
		self, err := os.Executable()
		if err != nil {
			ctx.views.Push(makeErrorView(ctx, err.Error()))
			return
		}
		err = syscall.Exec(self, []string{""}, os.Environ())
		if err != nil {
			ctx.views.Push(makeErrorView(ctx, err.Error()))
			return
		}
	}
}

func makeErrorView(ctx *ntcontext, errorMsg string) ViewFunc {
	return func(ctx *ntcontext, w *nucular.Window) {
		w.Row(15).Dynamic(1)
		w.Label("Error", "CB")
		w.Row(15).Dynamic(1)
		w.Label(errorMsg, "CB")
		w.Row(40).Dynamic(1)
		w.Row(25).Dynamic(1)
		if w.ButtonText("OK") {
			ctx.views.Pop()
			return
		}
	}
}

func makeFatalErrorView(ctx *ntcontext, errorMsg string) ViewFunc {
	return func(ctx *ntcontext, w *nucular.Window) {
		w.Row(15).Dynamic(1)
		w.Label("Fatal Error", "CB")
		w.Row(15).Dynamic(1)
		w.Label(errorMsg, "CB")
		w.Row(40).Dynamic(1)
		w.Row(25).Dynamic(1)
		if w.ButtonText("Quit") {
			os.Exit(1)
			return
		}
	}
}

func makeConfirmView(ctx *ntcontext, title, text, confirmText, denyText string, confirmfunc, denyfunc func()) ViewFunc {
	return func(ctx *ntcontext, w *nucular.Window) {
		w.Row(15).Dynamic(1)
		w.Label(title, "CB")
		w.Row(15).Dynamic(1)
		w.Label(text, "CB")
		w.Row(40).Dynamic(1)
		w.Row(25).Dynamic(2)
		if w.ButtonText(denyText) {
			ctx.views.Pop()
			go denyfunc()
			return
		}
		if w.ButtonText(confirmText) {
			ctx.views.Pop()
			go confirmfunc()
			return
		}
	}
}

func resetUI(ctx *ntcontext) {
	ctx.views = NewViewStack()
	ctx.views.Push(mainView)

	if !ctx.haveCapabilities {
		ctx.views.Push(capabilitiesView)
	}

	if ctx.serverInfo.outdatedPipeWire {
		ctx.views.Push(makeFatalErrorView(ctx,
			fmt.Sprintf("Your PipeWire version is too old. Detected %d.%d.%d. Require at least 0.3.28.",
				ctx.serverInfo.major, ctx.serverInfo.minor, ctx.serverInfo.patch)))
	}
}
