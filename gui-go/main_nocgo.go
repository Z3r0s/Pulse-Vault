//go:build !cgo

package main

import (
	"fmt"
	"os"
)

// GUI binary needs CGO + gcc. Without that we still compile so
// `go test ./...` works. Use build.ps1 when you want the real window.
func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-version", "-v", "version":
			printVersion()
			os.Exit(0)
		}
	}
	fmt.Fprintln(os.Stderr, "pulse-vault-gui needs CGO and gcc (OpenGL).")
	fmt.Fprintln(os.Stderr, "Windows: run .\\build.ps1 from gui-go (it finds MSYS2 mingw).")
	os.Exit(2)
}
