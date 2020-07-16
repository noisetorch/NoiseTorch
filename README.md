# NoiseTorch

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Last Release](https://img.shields.io/github/v/release/lawl/NoiseTorch?label=latest&style=flat-square)](https://github.com/lawl/NoiseTorch/releases)

NoiseTorch is an easy to use open source application for Linux with PulseAudio. It creates a virtual microphone that suppresses noise, in any application. Use whichever conferencing or VOIP application you like and simply select the NoiseTorch Virtual Microphone as input to torch the sound of your mechanical keyboard, computer fans, trains and the likes.

Don't forget to ~~like, comment and subscribe~~ leave a star ⭐ if this sounds useful to you! 

## Screenshot

![](https://i.imgur.com/T2wH0bl.png)

## Features
* Two click setup of your virtual denoising microphone
* A single, small, statically linked, self-contained binary

## Download & Install

[Download the latest release from GitHub](https://github.com/lawl/NoiseTorch/releases).

Unpack the `tgz` file, into your home directory.

    tar -C $HOME -xzf NoiseTorch_x64.tgz

This will unpack the application, icon and desktop entry to the correct place.  
Depending on your desktop environment you may need to tell it to rescan for applications, or tell it to do a refresh now.

With gnome this can be done with:

    gtk-update-icon-cache

You now have a `noisetorch` binary and desktop entry on your system.

#### Uninstall

    rm .local/bin/noisetorch
    rm .local/share/applications/noisetorch.desktop
    rm .local/share/icons/hicolor/256x256/apps/noisetorch.png 

## Usage

Select the microphone you want to denoise, and click "Load NoiseTorch", NoiseTorch will create a virtual microphone called "NoiseTorch Microphone" that you can select in any application.

When you're done using it, simply click "Unload NoiseTorch" to remove it again, until you need it next time.

The slider "Voice Activation Threshold" under settings, allows you to choose how strict NoiseTorch should be in only allowing your microphone to send sounds when it detects voice.. Generally you want this up as high as possible. With a decent microphone, you can turn this to the maximum of 95%. If you cut out during talking, slowly lower this strictness until you find a value that works for you.

If you set this to 0%, NoiseTorch will still dampen noise, but not deactivate your microphone if it doesn't detect voice.

Please keep in mind that you will need to reload NoiseTorch for these changes to apply.

## Troubleshooting

Unfortunately, sometimes NoiseTorch may display a "Working..." screen forever when loading the virtual microphone. This is usually due to PulseAudio crashing. If that happens, close NoiseTorch and try again, that usually works. I am working to improve this situation.

If you have a different problem with NoiseTorch, you can find a log file in `/tmp/noisetorch.log`. Please make sure to attach this when reporting an issue.

## Latency

NoiseTorch may introduce a small amount of latency. The amount of inherent latency introduced by noise supression is 10ms, this is very low and should not be a problem. Additionally PulseAudio currently introduces a variable amount of latency that depends on your system. Lowering this latency [requires a change in PulseAudio](https://gitlab.freedesktop.org/pulseaudio/pulseaudio/-/issues/120).

## Building from source

Install the Go compiler from [golang.org](https://golang.org/). And make sure you have a working C++ compiler.

```shell
 git clone https://github.com/lawl/NoiseTorch # Clone the repository
 cd NoiseTorch # cd into the cloned repository
 git submodule init # Tell git to look at submodules
 git submodule update # Update submodules
 make # build it
 ```

If you build from source, it's recommended that you disable automatic update checks.  
In `~/.config/noisetorch/config.toml` set `EnableUpdates = false`.

## Special thanks to

* [xiph.org](https://xiph.org)/[Mozilla's](https://mozilla.org) excellent [RNNoise](https://jmvalin.ca/demo/rnnoise/).
* [@werman](https://github.com/werman/)'s [LADSPA/VST wrapper](https://github.com/werman/noise-suppression-for-voice/) allowing us to load RNNoise into PulseAudio.
* [@aarzilli](https://github.com/aarzilli/)'s [nucular](https://github.com/aarzilli/nucular) GUI toolkit for Go.