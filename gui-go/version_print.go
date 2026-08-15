package main

import (
	"fmt"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/version"
)

func printVersion() {
	fmt.Println("Pulse-Vault")
	fmt.Printf("version: %s\n", version.Version)
	fmt.Println("Go native desktop GUI — V5 encrypted vault")
	fmt.Println("Production Windows build · rewritten for speed")
}
