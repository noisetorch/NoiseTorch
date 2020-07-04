Mostly-immediate-mode GUI library for Go.
Source port to go of an early version of [nuklear](https://github.com/vurtun/nuklear).

:warning: Subject to backwards incompatible changes. :warning:

:warning: Feature requests unaccompanied by an implementation will not be serviced. :warning:

## Documentation

See [godoc](https://godoc.org/github.com/aarzilli/nucular), `_examples/simple/main.go` and `_examples/overview/main.go` for single window examples, `_examples/demo/demo.go` for a multi-window example, and [gdlv](http://github.com/aarzilli/gdlv) for a more complex application built using nucular.

## Screenshots

![Overview](https://raw.githubusercontent.com/aarzilli/nucular/master/_examples/screen.png)
![Gdlv](https://raw.githubusercontent.com/aarzilli/gdlv/master/doc/screen.png)

## Backend

Nucular uses build tags to select its backend:

```
go build -tags nucular_gio
```

Selects the [gio](https://gioui.org/) backend.

```
go build -tags nucular_shiny
```

Selects the [shiny](https://github.com/golang/exp/tree/master/shiny) backend.

```
go build -tags nucular_shiny,metal
```

Selects the shiny backend but uses metal to render on macOS.

By default shiny is used on all operating systems except macOS, where gio is used.