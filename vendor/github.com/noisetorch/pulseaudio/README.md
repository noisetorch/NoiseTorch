# pulseaudio [![GoDoc](https://godoc.org/github.com/noisetorch/pulseaudio?status.svg)](https://godoc.org/github.com/noisetorch/pulseaudio)
Package pulseaudio is a pure-Go (no libpulse) implementation of the PulseAudio native protocol.

Download:
```shell
go get github.com/noisetorch/pulseaudio
```

* * *
Package pulseaudio is a pure-Go (no libpulse) implementation of the PulseAudio native protocol.

This library is a fork of https://github.com/mafik/pulseaudio
The original library deliberately tries to hide pulseaudio internals and doesn't expose them.

For the use case of NoiseTorch the opposite is true: access to the pulseaudio internals is needed.
This will be maintained as is required for [NoiseTorch](https://github.com/noisetorch/NoiseTorch) to work.
Pull Requests are, however, welcome.
