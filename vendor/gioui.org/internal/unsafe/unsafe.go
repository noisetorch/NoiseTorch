// SPDX-License-Identifier: Unlicense OR MIT

package unsafe

import (
	"reflect"
	"unsafe"
)

// BytesView returns a byte slice view of a slice.
func BytesView(s interface{}) []byte {
	v := reflect.ValueOf(s)
	first := v.Index(0)
	sz := int(first.Type().Size())
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		Data: uintptr(unsafe.Pointer((*reflect.SliceHeader)(unsafe.Pointer(first.UnsafeAddr())))),
		Len:  v.Len() * sz,
		Cap:  v.Cap() * sz,
	}))
}

// SliceOf returns a slice from a (native) pointer.
func SliceOf(s uintptr) []byte {
	if s == 0 {
		return nil
	}
	sh := reflect.SliceHeader{
		Data: s,
		Len:  1 << 30,
		Cap:  1 << 30,
	}
	return *(*[]byte)(unsafe.Pointer(&sh))
}

// GoString convert a NUL-terminated C string
// to a Go string.
func GoString(s []byte) string {
	i := 0
	for {
		if s[i] == 0 {
			break
		}
		i++
	}
	return string(s[:i])
}
