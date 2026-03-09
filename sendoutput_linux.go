//go:build linux

package main

import (
	"syscall"
	"unsafe"
)

func sendOutput() {
	if len(output) == 0 {
		return
	}
	b := []byte(output)
	dataPtr := uintptr(unsafe.Pointer(&b[0]))
	dataSize := uintptr(uint32(len(b)))

	_, _, errNo := syscall.RawSyscall(g_callback, 2, dataPtr, dataSize)
	if errNo != 0 {
		println("sendOutput error:", errNo.Error())
	}
}
