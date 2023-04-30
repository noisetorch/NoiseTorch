// SPDX-License-Identifier: Unlicense OR MIT

package driver

import (
	"fmt"
	"unsafe"

	"gioui.org/internal/gl"
)

// See gpu/api.go for documentation for the API types

type API interface {
	implementsAPI()
}

type OpenGL struct {
	// Context contains the WebGL context for WebAssembly platforms. It is
	// empty for all other platforms; an OpenGL context is assumed current when
	// calling NewDevice.
	Context gl.Context
}

type Direct3D11 struct {
	// Device contains a *ID3D11Device.
	Device unsafe.Pointer
}

// API specific device constructors.
var (
	NewOpenGLDevice     func(api OpenGL) (Device, error)
	NewDirect3D11Device func(api Direct3D11) (Device, error)
)

// NewDevice creates a new Device given the api.
//
// Note that the device does not assume ownership of the resources contained in
// api; the caller must ensure the resources are valid until the device is
// released.
func NewDevice(api API) (Device, error) {
	switch api := api.(type) {
	case OpenGL:
		if NewOpenGLDevice != nil {
			return NewOpenGLDevice(api)
		}
	case Direct3D11:
		if NewDirect3D11Device != nil {
			return NewDirect3D11Device(api)
		}
	}
	return nil, fmt.Errorf("driver: no driver available for the API %T", api)
}

func (OpenGL) implementsAPI()     {}
func (Direct3D11) implementsAPI() {}
