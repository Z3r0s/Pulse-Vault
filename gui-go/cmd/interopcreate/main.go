package main

import (
	"fmt"
	"os"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/vault"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: interopcreate <vault> <password> <file>")
		os.Exit(2)
	}
	v := vault.New(os.Args[1])
	if err := v.Create(os.Args[2], "fast"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := v.AddFile(os.Args[3], false); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	names, err := v.ListFiles()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("go_created", names)
}
