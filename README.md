# NoiseTorch

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Last Release](https://img.shields.io/github/v/release/lawl/NoiseTorch?label=latest&style=flat-square)](https://github.com/lawl/NoiseTorch/releases)

NoiseTorch is an easy to use open source application for Linux with PulseAudio. It creates a virtual microphone that suppresses noise, in any application. Use whichever conferencing or VOIP application you like and simply select the NoiseTorch Virtual Microphone as input to torch the sound of your mechanical keyboard, computer fans, trains and the likes.

Don't forget to ~~like, comment and subscribe~~ leave a star ⭐ if this sounds useful to you! 

## Screenshot

![](https://i.imgur.com/uyLdB9P.png)

## Features
* Two click setup of your virtual denoising microphone
* A single, small, statically linked, self-contained binary

## Download

NoiseTorch offers the following installation methods:

#### Snap

Coming soon™.

#### Flatpak

Coming soon™.

#### Manual (no automatic updates)
[Download the latest release from GitHub](https://github.com/lawl/NoiseTorch/releases).

Unpack the `tgz` file, and double click the executable.

## Usage

Select the microphone you want to denoise, and click "Load NoiseTorch", NoiseTorch will create a virtual microphone called "NoiseTorch Microphone" that you can select in any application.

When you're done using it, simply click "Unload NoiseTorch" to remove it again, until you need it next time.

The slider "Filter strictness" under settings, allows you to choose how strict NoiseTorch should be. Generally you want this up as high as possible. With a decent microphone, you can turn this to the maximum of 95%. If you cut out during talking, slowly lower this strictness until you find a value that works for you.

Please keep in mind that you will need to reload NoiseTorch for these changes to apply.

## Troubleshooting

Unfortunately, sometimes TorchNoise may display a "Working..." screen forever when loading the virtual microphone. This is usually due to PulseAudio crashing. If that happens, close NoiseTorch and try again, that usually works. I am working to improve this situation.

If you have a different problem with NoiseTorch, you can find a log file in `/tmp/noisetorch.log`. Please make sure to attach this when reporting an issue.

## Building from source

Install the Go compiler from [golang.org](https://golang.org/).

```shell
 git clone https://github.com/lawl/NoiseTorch # Clone the repository
 cd NoiseTorch # cd into the cloned repository
 make # build it
 ```

 Make in this case is just a shorthand for

 ```shell
  go generate
 ```
  To generate the embed file for librnnoise_ladspa.so (You will need to build this separately, see the README file in the `librnnoise_ladspa` folder).
 
  Followed by

  ```shell
  go build
  ```
  
  `go generate` only needs to be ran once, whenever you want to embed a new build of `librnnoise_ladspa.so`.


## Special thanks to

* [xipgh.org](https://xiph.org)/[Mozilla's](https://mozilla.org) excellent [RNNoise](https://jmvalin.ca/demo/rnnoise/).
* [@werman](https://github.com/werman/)'s [LADSPA/VST wrapper](https://github.com/werman/noise-suppression-for-voice/) allowing us to load RNNoise into PulseAudio.
* [@aarzilli](https://github.com/aarzilli/)'s [nucular](https://github.com/aarzilli/nucular) GUI toolkit for Go.