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
	"golang.org/x/term"
)

func cmdOpen(args []string) error {
	pw, args, err := flagPassword(args)
	if err != nil {
		return err
	}
	if len(args) < 1 {
		return fmt.Errorf("usage: open <vault>")
	}
	pw, err = passwordOrPrompt(pw, "password: ")
	if err != nil {
		return err
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
	version := v.Format
	if strings.HasPrefix(version, "PULSEVAULT") {
		version = strings.TrimPrefix(version, "PULSEVAULT")
		if idx := strings.IndexByte(version, '_'); idx >= 0 {
			version = version[:idx]
		}
	}
	bits = append(bits, version)
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
	fields, err := splitConsoleLine(line)
	if err != nil {
		return err
	}
	if len(fields) == 0 {
		return nil
	}
	cmd := strings.ToLower(fields[0])
	rest := fields[1:]
	switch cmd {
	case "help", "h", "?":
		fmt.Fprintln(out(), box("console", []string{
			cyan("list") + muted("                 inventory"),
			cyan("add") + muted(" <file-or-folder> seal files or a folder tree"),
			cyan("extract") + muted(" <name> [dir]  write a file out"),
			cyan("extract-all") + muted(" [dir]       restore every file"),
			cyan("delete") + muted(" <name> [...]  remove sealed files"),
			cyan("verify") + muted("               integrity check"),
			cyan("info") + muted("                 container card"),
			cyan("password") + muted(" [new]       rotate the key (hidden prompts)"),
			cyan("quit") + muted("                 lock and leave"),
		}, 56))
		return nil
	case "list", "ls":
		return printList(v)
	case "add":
		if len(rest) < 1 {
			return fmt.Errorf("usage: add <file-or-folder> [more paths ...]")
		}
		start := time.Now()
		if err := v.AddPaths(rest, false); err != nil {
			return err
		}
		printAdded(len(rest), time.Since(start))
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
	case "extract-all", "extractall":
		dir := "."
		if len(rest) > 0 {
			dir = rest[0]
		}
		paths, err := v.ExtractAll(dir, false)
		if err != nil {
			return err
		}
		for _, path := range paths {
			printExtracted(path)
		}
		return nil
	case "delete", "rm", "del":
		if len(rest) < 1 {
			return fmt.Errorf("usage: delete <name> [more names ...]")
		}
		if err := v.DeleteFiles(rest); err != nil {
			return err
		}
		if len(rest) == 1 {
			printDeleted(rest[0])
		} else {
			printDeletedBatch(len(rest))
		}
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
		newPassword := ""
		if len(rest) >= 1 {
			newPassword = rest[0]
		} else {
			newPassword, err = readHiddenPassword("  new password: ")
			if err != nil {
				return err
			}
		}
		warnPasswordPolicy(newPassword)
		old, err := readHiddenPassword("  current password: ")
		if err != nil {
			return err
		}
		if err := v.ChangePassword(old, newPassword); err != nil {
			return err
		}
		printPasswordChanged(v.Profile)
		return nil
	case "quit", "exit", "q", "lock":
		return errConsoleQuit
	default:
		return fmt.Errorf("unknown console command %q — try help", cmd)
	}
}

func readHiddenPassword(prompt string) (string, error) {
	if !stdinIsTTY() {
		return "", fmt.Errorf("%s requires an interactive terminal", strings.TrimSpace(prompt))
	}
	fmt.Fprint(out(), prompt)
	value, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(out())
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return strings.TrimRight(string(value), "\r\n"), nil
}

// splitConsoleLine accepts quoted paths so the interactive CLI can handle
// filenames containing spaces without requiring shell-specific escaping.
func splitConsoleLine(line string) ([]string, error) {
	var fields []string
	var current strings.Builder
	var quote rune
	started := false
	flush := func() {
		if started {
			fields = append(fields, current.String())
			current.Reset()
			started = false
		}
	}
	for _, r := range line {
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			started = true
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
			started = true
		case ' ', '\t', '\r', '\n':
			flush()
		default:
			current.WriteRune(r)
			started = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in console command")
	}
	flush()
	return fields, nil
}
