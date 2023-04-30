// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import "gioui.org/gpu/internal/driver"

// An API carries the necessary GPU API specific resources to create a Device.
// There is an API type for each supported GPU API such as OpenGL and Direct3D.
type API = driver.API

// OpenGL denotes the OpenGL or OpenGL ES API.
type OpenGL = driver.OpenGL

// Direct3D11 denotes the Direct3D API.
type Direct3D11 = driver.Direct3D11
