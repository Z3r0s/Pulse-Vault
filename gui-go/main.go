//go:build cgo

// Pulse-Vault desktop GUI (Go + Fyne). Needs gcc (CGO).
// Windows: .\build.ps1  (adds MSYS2 mingw if it's installed)
package main

import (
	"os"

	"fyne.io/fyne/v2/app"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/ui"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-version", "-v", "version":
			printVersion()
			os.Exit(0)
		}
	}

	a := app.NewWithID("io.github.z3r0s.PulseVault")
	a.Settings().SetTheme(&ui.PulseTheme{})
	ui.Run(a)
}
