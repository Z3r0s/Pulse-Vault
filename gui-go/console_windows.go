//go:build windows && cgo

package main

import (
	"syscall"
	"unsafe"
)

// hideOwnConsole hides a console this process allocated (double-click of a
// console-subsystem build). An inherited terminal is left alone so running
// the exe from an already-open PowerShell still works.
//
// Production builds use -H windowsgui (build.ps1) so no console is created.
func init() {
	hideOwnConsole()
}

func hideOwnConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	user32 := syscall.NewLazyDLL("user32.dll")
	getConsoleProcessList := kernel32.NewProc("GetConsoleProcessList")
	getConsoleWindow := kernel32.NewProc("GetConsoleWindow")
	showWindow := user32.NewProc("ShowWindow")
	freeConsole := kernel32.NewProc("FreeConsole")

	var buf [4]uint32
	n, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&buf[0])), 4)
	if n != 1 {
		return
	}
	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	const swHide = 0
	_, _, _ = showWindow.Call(hwnd, swHide)
	_, _, _ = freeConsole.Call()
}
