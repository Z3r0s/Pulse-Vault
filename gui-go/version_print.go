package main

import (
	"fmt"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/version"
)

func printVersion() {
	fmt.Println("Pulse-Vault")
	fmt.Printf("version: %s\n", version.Version)
	fmt.Println("Go native desktop GUI — V6 encrypted vault (V5 readable)")
	fmt.Println("Production Windows build · rewritten for speed")
}
