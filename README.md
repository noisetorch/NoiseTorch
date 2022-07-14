<h1 align="center"> NoiseTorch-ng</h1>
<h3 align="center"> Noise Supression Application for PulseAudio or Pipewire</h3>
<p align="center"><img src="https://raw.githubusercontent.com/noisetorch/NoiseTorch/master/assets/icon/noisetorch.png" width="100" height="100"></p> 


<div align="center">
    
  <a href="">[![Licence][licence]][licence-url]</a>
  <a href="">[![Latest][version]][version-url]</a>
    
</div>

[licence]: https://img.shields.io/badge/License-GPLv3-blue.svg
[licence-url]: https://www.gnu.org/licenses/gpl-3.0
[version]: https://img.shields.io/github/v/release/noisetorch/NoiseTorch?label=Latest&style=flat
[version-url]: https://github.com/noisetorch/NoiseTorch/releases
[stars-shield]: https://img.shields.io/github/stars/noisetorch/NoiseTorch?maxAge=2592000
[stars-url]: https://github.com/noisetorch/NoiseTorch/stargazers/

NoiseTorch-ng is an easy to use open source application for Linux with PulseAudio or PipeWire. It creates a virtual microphone that suppresses noise, in any application. Use whichever conferencing or VOIP application you like and simply select the filtered Virtual Microphone as input to torch the sound of your mechanical keyboard, computer fans, trains and the likes.

Don't forget to leave a star ⭐ if this sounds useful to you! 

## Regarding the recent security incident

Due to a suspected security breach of the update server and code repository, there's
 been a concerted effort by the NoiseTorch community to ensure the source code and
 binaries are free from malicious code.
 
 > No malicious code has been found.
 
 You can read more about the audit that was done [here](https://github.com/noisetorch/NoiseTorch/discussions/275)
 and [here](https://github.com/noisetorch/NoiseTorch/discussions/264).
 Updates will now be retrieved from the project's releases page to avoid any risk
 of this reoccurring. We thank everyone for their trust and the love that they've
 shown towards the project in this unpleasant time. 

## Screenshot

![](https://i.imgur.com/T2wH0bl.png)

Then simply select "Filtered" as your microphone in any application. OBS, Mumble, Discord, anywhere.

![](https://i.imgur.com/nimi7Ne.png)

## Demo

Linux For Everyone has a good demo video [here](https://www.youtube.com/watch?v=DzN9rYNeeIU).

## Features
* Two click setup of your virtual denoising microphone
* A single, small, statically linked, self-contained binary

## Download & Install

[Download the latest release from GitHub](https://github.com/noisetorch/NoiseTorch/releases).

Unpack the `tgz` file, into your home directory.

    tar -C $HOME -h -xzf NoiseTorch_x64_v0.12.2.tgz

This will unpack the application, icon and desktop entry to the correct place.  
Depending on your desktop environment you may need to wait for it to rescan for applications, or tell it to do a refresh now.

With gnome this can be done with:

    gtk-update-icon-cache

You now have a `noisetorch` binary and desktop entry on your system.

Give it the required permissions with `setcap`:

    sudo setcap 'CAP_SYS_RESOURCE=+ep' ~/.local/bin/noisetorch

If NoiseTorch-ng doesn't start after installation, you may also have to make sure that `~/.local/bin` is in your PATH. On most distributions e.g. Ubuntu, this should be the case by default. If it's not, make sure to append

```
if [ -d "$HOME/.local/bin" ] ; then
    PATH="$HOME/.local/bin:$PATH"
fi
```

to your `~/.profile`. If you do already have that, you may have to log in and out for it to actually apply if this is the first time you're using `~/.local/bin`.

#### Uninstall

    rm ~/.local/bin/noisetorch
    rm ~/.local/share/applications/noisetorch.desktop
    rm ~/.local/share/icons/hicolor/256x256/apps/noisetorch.png 

## Troubleshooting

Please see the [Troubleshooting](https://github.com/noisetorch/NoiseTorch/wiki/Troubleshooting) section in the wiki.

## Usage

Select the microphone you want to denoise, and click "Load", NoiseTorch-ng will create a virtual microphone called "Filtered Microphone" that you can select in any application. Output filtering works the same way, simply output the applications you want to filter to "Filtered Headphones".

When you're done using it, simply click "Unload" to remove it again, until you need it next time.

The slider "Voice Activation Threshold" under settings, allows you to choose how strict NoiseTorch-ng should be in only allowing your microphone to send sounds when it detects voice.. Generally you want this up as high as possible. With a decent microphone, you can turn this to the maximum of 95%. If you cut out during talking, slowly lower this strictness until you find a value that works for you.

If you set this to 0%, NoiseTorch-ng will still dampen noise, but not deactivate your microphone if it doesn't detect voice.

Please keep in mind that you will need to reload NoiseTorch-ng for these changes to apply.

Once NoiseTorch-ng has been loaded, feel free to close the window, the virtual microphone will continue working until you explicitly unload it. The NoiseTorch-ng process is not required anymore once it has been loaded.

## Latency

NoiseTorch-ng may introduce a small amount of latency for microphone filtering. The amount of inherent latency introduced by noise supression is 10ms, this is very low and should not be a problem. Additionally PulseAudio currently introduces a variable amount of latency that depends on your system. Lowering this latency [requires a change in PulseAudio](https://gitlab.freedesktop.org/pulseaudio/pulseaudio/-/issues/120).

Output filtering currently introduces something on the order of ~100ms with pulseaudio. This should still be fine for regular conferences, VOIPing and gaming. Maybe not for competitive gaming teams.

## Building from source

Install the Go compiler from [golang.org](https://golang.org/). And make sure you have a working C++ compiler.

```shell
 git clone https://github.com/noisetorch/NoiseTorch # Clone the repository
 cd NoiseTorch # cd into the cloned repository
 make # build it
```

To install it:

```shell
cp ./bin/noisetorch ~/.local/bin
cp ./assets/noisetorch.desktop ~/.local/share/applications
cp ./assets/icon/noisetorch.png ~/.local/share/icons/hicolor/256x256/apps
```

## Special thanks to

* [@lawl](https://github.com/lawl) Creator of NoiseTorch
* [xiph.org](https://xiph.org)/[Mozilla's](https://mozilla.org) excellent [RNNoise](https://jmvalin.ca/demo/rnnoise/).
* [@werman](https://github.com/werman/)'s [noise-suppression-for-voice](https://github.com/werman/noise-suppression-for-voice/) for the inspiration
* [@aarzilli](https://github.com/aarzilli/)'s [nucular](https://github.com/aarzilli/nucular) GUI toolkit for Go.
* [Sallee Design](https://www.salleedesign.com) (info@salleedesign.com)'s Microphone Icon under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)

