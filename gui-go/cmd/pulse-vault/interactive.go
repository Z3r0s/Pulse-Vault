package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/vault"
)

func cmdOpen(args []string) error {
	pw, args, err := flagPassword(args)
	if err != nil {
		return err
	}
	if len(args) < 1 || pw == "" {
		return fmt.Errorf("usage: open <vault> --password <pw>")
	}
	path := args[0]
	v := vault.New(path)
	if err := v.Unlock(pw); err != nil {
		return err
	}
	defer v.Lock()

	printBanner()
	printOpenStatus(v)

	// Interactive prompt only on a real terminal. Captured stdout (tests,
	// pipes, CI) and inherited-but-empty stdin get a one-shot inventory.
	if !stdinIsTTY() || !stdoutIsTTY() {
		fmt.Fprintln(out(), muted("  (non-interactive — showing inventory, then exit)"))
		return printList(v)
	}
	return runConsole(v)
}

func printOpenStatus(v *vault.Vault) {
	n := v.FileCount()
	item := "files"
	if n == 1 {
		item = "file"
	}
	state := markOpen() + " " + bold("UNLOCKED")
	bits := []string{
		state,
		fmt.Sprintf("%d %s", n, item),
		filepath.Base(v.Path),
	}
	if v.HasCarrier() {
		bits = append(bits, "hidden inside picture")
	}
	bits = append(bits, "V5")
	fmt.Fprintf(out(), "  %s\n\n", strings.Join(bits, muted("  ·  ")))
	fmt.Fprintln(out(), muted("  /list  /add  /extract  /delete  /verify  /info  /password  /help  /quit"))
	fmt.Fprintln(out())
}

func runConsole(v *vault.Vault) error {
	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprint(out(), "  "+glyph()+"  "+cyan("pulse")+" "+muted("›")+" ")
		line, err := in.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Fprintln(out())
				return nil
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if err := handleConsole(v, line, in); err != nil {
			if err == errConsoleQuit {
				fmt.Fprintf(out(), "  %s  vault locked. see you.\n", glyph())
				return nil
			}
			printError(err)
		}
	}
}

var errConsoleQuit = fmt.Errorf("quit")

func handleConsole(v *vault.Vault, line string, in *bufio.Reader) error {
	line = strings.TrimPrefix(line, "/")
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	cmd := strings.ToLower(fields[0])
	rest := fields[1:]
	switch cmd {
	case "help", "h", "?":
		fmt.Fprintln(out(), box("console", []string{
			cyan("list") + muted("                 inventory"),
			cyan("add") + muted(" <file>           seal a file"),
			cyan("extract") + muted(" <name> [dir]  write a file out"),
			cyan("delete") + muted(" <name>        remove a sealed file"),
			cyan("verify") + muted("               integrity check"),
			cyan("info") + muted("                 container card"),
			cyan("password") + muted(" <new>       rotate the key"),
			cyan("quit") + muted("                 lock and leave"),
		}, 56))
		return nil
	case "list", "ls":
		return printList(v)
	case "add":
		if len(rest) < 1 {
			return fmt.Errorf("usage: add <file>")
		}
		start := time.Now()
		if err := v.AddFile(rest[0], false); err != nil {
			return err
		}
		st, _ := os.Stat(rest[0])
		var sz int64
		if st != nil {
			sz = st.Size()
		}
		printAdded(filepath.Base(rest[0]), sz, time.Since(start))
		return nil
	case "extract", "x":
		if len(rest) < 1 {
			return fmt.Errorf("usage: extract <name> [outdir]")
		}
		dir := "."
		if len(rest) >= 2 {
			dir = rest[1]
		}
		outPath, err := v.ExtractFile(rest[0], dir, false)
		if err != nil {
			return err
		}
		printExtracted(outPath)
		return nil
	case "delete", "rm", "del":
		if len(rest) < 1 {
			return fmt.Errorf("usage: delete <name>")
		}
		if err := v.DeleteFile(rest[0]); err != nil {
			return err
		}
		printDeleted(rest[0])
		return nil
	case "verify":
		fc, bc, hc, err := v.VerifyAll()
		if err != nil {
			return err
		}
		printVerified(fc, bc, hc)
		return nil
	case "info":
		st, err := os.Stat(v.Path)
		if err != nil {
			return err
		}
		rec, recErr := vault.PeekKDFProfile(v.Path)
		prefix, pErr := vault.PeekCarrierPrefix(v.Path)
		printInfo(v.Path, st.Size(), rec, recErr == nil, recErr, prefix, pErr, v)
		return nil
	case "password", "passwd":
		if len(rest) < 1 {
			return fmt.Errorf("usage: password <new-password>")
		}
		warnPasswordPolicy(rest[0])
		// Need the current password — prompt if TTY, else require it inline is already new only.
		fmt.Fprint(out(), "  current password: ")
		old, err := in.ReadString('\n')
		if err != nil {
			return err
		}
		old = strings.TrimRight(old, "\r\n")
		if err := v.ChangePassword(old, rest[0]); err != nil {
			return err
		}
		printPasswordChanged()
		return nil
	case "quit", "exit", "q", "lock":
		return errConsoleQuit
	default:
		return fmt.Errorf("unknown console command %q — try help", cmd)
	}
}
