# pulseaudio [![GoDoc](https://godoc.org/github.com/lawl/pulseaudio?status.svg)](https://godoc.org/github.com/lawl/pulseaudio)
Package pulseaudio is a pure-Go (no libpulse) implementation of the PulseAudio native protocol.

Download:
```shell
go get github.com/lawl/pulseaudio
```

* * *
Package pulseaudio is a pure-Go (no libpulse) implementation of the PulseAudio native protocol.

This library is a fork of https://github.com/mafik/pulseaudio
The original library deliberately tries to hide pulseaudio internals and doesn't expose them.

For my usecase I needed the exact opposite, access to pulseaudio internals.
I will most likely only maintain this as far as is required for [noisetorch](https://github.com/lawl/NoiseTorch) to work.
Pull Requests are however welcome.
