//go:build windows

package main

import "unsafe"

// unsafePointer is the single spot where the console helpers hand a Go slice to
// a Win32 call.
func unsafePointer[T any](p *T) unsafe.Pointer {
	return unsafe.Pointer(p)
}
