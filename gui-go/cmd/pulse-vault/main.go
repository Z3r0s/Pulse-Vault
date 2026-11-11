// Pulse-Vault CLI — pure Go, no CGO. Cross-compiles for windows/linux/darwin.
//
//	go build -o pulse-vault ./cmd/pulse-vault
//	GOOS=linux GOARCH=amd64 go build -o pulse-vault-linux-amd64 ./cmd/pulse-vault
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

const productName = "Pulse-Vault"

func main() {
	args := consumeGlobals(os.Args[1:])
	initTheme()

	if len(args) < 1 {
		printHelp()
		os.Exit(0)
	}
	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "version", "--version", "-version", "-v":
		printVersion()
		return
	case "help", "--help", "-h":
		printHelp()
		return
	case "create":
		exit(cmdCreate(rest))
	case "open":
		exit(cmdOpen(rest))
	case "list", "ls":
		exit(cmdList(rest))
	case "add":
		exit(cmdAdd(rest))
	case "extract":
		exit(cmdExtract(rest))
	case "extract-all", "extractall":
		exit(cmdExtractAll(rest))
	case "delete", "rm":
		exit(cmdDelete(rest))
	case "verify":
		exit(cmdVerify(rest))
	case "change-password":
		exit(cmdChangePassword(rest))
	case "migrate":
		exit(cmdMigrate(rest))
	case "info":
		exit(cmdInfo(rest))
	default:
		printUnknown(cmd)
		printHelp()
		os.Exit(2)
	}
}

func exit(err error) {
	if err != nil {
		printError(err)
		os.Exit(1)
	}
}

func flagPassword(args []string) (pw string, rest []string, err error) {
	rest = make([]string, 0, len(args))
	var source string
	set := func(value, kind string) error {
		if source != "" {
			return fmt.Errorf("password specified more than once (%s and %s)", source, kind)
		}
		source = kind
		pw = value
		return nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--password" || a == "-p" {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--password requires a value")
			}
			if err := set(args[i+1], "--password"); err != nil {
				return "", nil, err
			}
			i++
			continue
		}
		if strings.HasPrefix(a, "--password=") {
			if err := set(strings.TrimPrefix(a, "--password="), "--password"); err != nil {
				return "", nil, err
			}
			continue
		}
		if a == "--password-stdin" {
			value, readErr := readPasswordStdin()
			if readErr != nil {
				return "", nil, readErr
			}
			if err := set(value, "--password-stdin"); err != nil {
				return "", nil, err
			}
			continue
		}
		if a == "--password-prompt" {
			value, readErr := readHiddenPassword("password: ")
			if readErr != nil {
				return "", nil, readErr
			}
			if err := set(value, "--password-prompt"); err != nil {
				return "", nil, err
			}
			continue
		}
		if a == "--password-file" || strings.HasPrefix(a, "--password-file=") {
			path := ""
			if a == "--password-file" {
				if i+1 >= len(args) {
					return "", nil, fmt.Errorf("--password-file requires a path")
				}
				path = args[i+1]
				i++
			} else {
				path = strings.TrimPrefix(a, "--password-file=")
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return "", nil, fmt.Errorf("read password file: %w", readErr)
			}
			if err := set(strings.TrimRight(string(data), "\r\n"), "--password-file"); err != nil {
				return "", nil, err
			}
			continue
		}
		rest = append(rest, a)
	}
	return pw, rest, nil
}

func readPasswordStdin() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read password from stdin: %w", err)
	}
	return strings.TrimRight(value, "\r\n"), nil
}

func flagProfile(args []string) (profile string, rest []string) {
	profile = "standard"
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--profile" {
			if i+1 < len(args) {
				profile = args[i+1]
				i++
			}
			continue
		}
		if strings.HasPrefix(a, "--profile=") {
			profile = strings.TrimPrefix(a, "--profile=")
			continue
		}
		rest = append(rest, a)
	}
	return profile, rest
}

func flagProfileOverride(args []string) (profile string, rest []string, err error) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--profile" {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
				return "", nil, fmt.Errorf("--profile requires standard or hardened")
			}
			profile = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(a, "--profile=") {
			profile = strings.TrimPrefix(a, "--profile=")
			continue
		}
		rest = append(rest, a)
	}
	return profile, rest, nil
}

func flagCarrier(args []string) (carrier string, rest []string, err error) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--carrier" {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--carrier requires a path")
			}
			carrier = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(a, "--carrier=") {
			carrier = strings.TrimPrefix(a, "--carrier=")
			continue
		}
		rest = append(rest, a)
	}
	return carrier, rest, nil
}

func warnPasswordPolicy(pw string) {
	if msg := vault.PasswordPolicyError(pw); msg != "" {
		printWarn("weak password: " + msg)
	}
}

func passwordOrPrompt(password, label string) (string, error) {
	if password != "" {
		return password, nil
	}
	if !stdinIsTTY() {
		return "", fmt.Errorf("password required; use --password-prompt, --password-stdin, or --password-file")
	}
	return readHiddenPassword(label)
}

func hasMediaExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".mp4", ".mov", ".webm":
		return true
	default:
		return false
	}
}

// resolveCreateDest appends .pulsevault only for bare names without a carrier
// or media extension (so hidden.png / --carrier dests stay as given).
func resolveCreateDest(path, carrier string) string {
	if carrier != "" || hasMediaExt(path) {
		return path
	}
	if !strings.HasSuffix(strings.ToLower(path), ".pulsevault") {
		return path + ".pulsevault"
	}
	return path
}

func cmdCreate(args []string) error {
	carrier, args, err := flagCarrier(args)
	if err != nil {
		return err
	}
	profile, args := flagProfile(args)
	pw, args, err := flagPassword(args)
	if err != nil {
		return err
	}
	if len(args) < 1 {
		return fmt.Errorf("usage: create <vault> [--profile standard|hardened] [--carrier <path>]")
	}
	if profile == "fast" && os.Getenv("PULSEVAULT_TEST_FAST_KDF") != "1" {
		return fmt.Errorf("the fast KDF profile is test-only; use standard or hardened")
	}
	if profile != "standard" && profile != "hardened" && !(profile == "fast" && os.Getenv("PULSEVAULT_TEST_FAST_KDF") == "1") {
		return fmt.Errorf("unknown KDF profile %q; choose standard or hardened", profile)
	}
	pw, err = passwordOrPrompt(pw, "password: ")
	if err != nil {
		return err
	}
	path := resolveCreateDest(args[0], carrier)
	if carrier != "" {
		if _, err := os.Stat(carrier); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("carrier file not found: %s", carrier)
			}
			return fmt.Errorf("carrier: %w", err)
		}
	}
	warnPasswordPolicy(pw)
	v := vault.New(path)
	start := time.Now()
	if carrier != "" {
		if err := v.CreateWithCarrier(pw, profile, carrier); err != nil {
			return err
		}
		printCreated(path, profile, true, time.Since(start))
		return nil
	}
	if err := v.Create(pw, profile); err != nil {
		return err
	}
	printCreated(path, profile, false, time.Since(start))
	return nil
}

func cmdList(args []string) error {
	pw, args, err := flagPassword(args)
	if err != nil {
		return err
	}
	if len(args) < 1 {
		return fmt.Errorf("usage: list <vault>")
	}
	pw, err = passwordOrPrompt(pw, "password: ")
	if err != nil {
		return err
	}
	v := vault.New(args[0])
	if err := v.Unlock(pw); err != nil {
		return err
	}
	defer v.Lock()
	return printList(v)
}

func cmdAdd(args []string) error {
	overwrite, args := flagBool(args, "--overwrite")
	pw, args, err := flagPassword(args)
	if err != nil {
		return err
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: add <vault> <file-or-folder> [file-or-folder ...]")
	}
	pw, err = passwordOrPrompt(pw, "password: ")
	if err != nil {
		return err
	}
	v := vault.New(args[0])
	if err := v.Unlock(pw); err != nil {
		return err
	}
	defer v.Lock()
	start := time.Now()
	if err := v.AddPaths(args[1:], overwrite); err != nil {
		return err
	}
	printAdded(len(args)-1, time.Since(start))
	return nil
}

func flagNewPassword(args []string) (pw string, rest []string, err error) {
	rest = make([]string, 0, len(args))
	var source string
	set := func(value, kind string) error {
		if source != "" {
			return fmt.Errorf("new password specified more than once (%s and %s)", source, kind)
		}
		source = kind
		pw = value
		return nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--new-password" {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--new-password requires a value")
			}
			if err := set(args[i+1], "--new-password"); err != nil {
				return "", nil, err
			}
			i++
			continue
		}
		if strings.HasPrefix(a, "--new-password=") {
			if err := set(strings.TrimPrefix(a, "--new-password="), "--new-password"); err != nil {
				return "", nil, err
			}
			continue
		}
		if a == "--new-password-stdin" {
			value, readErr := readPasswordStdin()
			if readErr != nil {
				return "", nil, readErr
			}
			if err := set(value, "--new-password-stdin"); err != nil {
				return "", nil, err
			}
			continue
		}
		if a == "--new-password-prompt" {
			value, readErr := readHiddenPassword("new password: ")
			if readErr != nil {
				return "", nil, readErr
			}
			if err := set(value, "--new-password-prompt"); err != nil {
				return "", nil, err
			}
			continue
		}
		if a == "--new-password-file" || strings.HasPrefix(a, "--new-password-file=") {
			path := ""
			if a == "--new-password-file" {
				if i+1 >= len(args) {
					return "", nil, fmt.Errorf("--new-password-file requires a path")
				}
				path = args[i+1]
				i++
			} else {
				path = strings.TrimPrefix(a, "--new-password-file=")
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return "", nil, fmt.Errorf("read new password file: %w", readErr)
			}
			if err := set(strings.TrimRight(string(data), "\r\n"), "--new-password-file"); err != nil {
				return "", nil, err
			}
			continue
		}
		rest = append(rest, a)
	}
	return pw, rest, nil
}

func cmdVerify(args []string) error {
	pw, args, err := flagPassword(args)
	if err != nil {
		return err
	}
	if len(args) < 1 {
		return fmt.Errorf("usage: verify <vault>")
	}
	pw, err = passwordOrPrompt(pw, "password: ")
	if err != nil {
		return err
	}
	v := vault.New(args[0])
	if err := v.Unlock(pw); err != nil {
		return err
	}
	defer v.Lock()
	fc, bc, hc, err := v.VerifyAll()
	if err != nil {
		return err
	}
	printVerified(fc, bc, hc)
	return nil
}

func cmdChangePassword(args []string) error {
	profile, args, err := flagProfileOverride(args)
	if err != nil {
		return err
	}
	if profile != "" && profile != "standard" && profile != "hardened" {
		return fmt.Errorf("unknown KDF profile %q; choose standard or hardened", profile)
	}
	newPW, args, err := flagNewPassword(args)
	if err != nil {
		return err
	}
	oldPW, args, err := flagPassword(args)
	if err != nil {
		return err
	}
	if len(args) < 1 {
		return fmt.Errorf("usage: change-password <vault> [--profile standard|hardened]")
	}
	oldPW, err = passwordOrPrompt(oldPW, "current password: ")
	if err != nil {
		return err
	}
	newPW, err = passwordOrPrompt(newPW, "new password: ")
	if err != nil {
		return err
	}
	warnPasswordPolicy(newPW)
	v := vault.New(args[0])
	if err := v.Unlock(oldPW); err != nil {
		return err
	}
	defer v.Lock()
	if err := v.ChangePasswordWithProfile(oldPW, newPW, profile); err != nil {
		return err
	}
	printPasswordChanged(v.Profile)
	return nil
}

func cmdExtract(args []string) error {
	overwrite, args := flagBool(args, "--overwrite")
	pw, args, err := flagPassword(args)
	if err != nil {
		return err
	}
	if len(args) < 3 {
		return fmt.Errorf("usage: extract <vault> <name> <outdir> [--overwrite]")
	}
	pw, err = passwordOrPrompt(pw, "password: ")
	if err != nil {
		return err
	}
	v := vault.New(args[0])
	if err := v.Unlock(pw); err != nil {
		return err
	}
	defer v.Lock()
	outPath, err := v.ExtractFile(args[1], args[2], overwrite)
	if err != nil {
		return err
	}
	printExtracted(outPath)
	return nil
}

func cmdExtractAll(args []string) error {
	overwrite, args := flagBool(args, "--overwrite")
	pw, args, err := flagPassword(args)
	if err != nil {
		return err
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: extract-all <vault> <outdir> [--overwrite]")
	}
	pw, err = passwordOrPrompt(pw, "password: ")
	if err != nil {
		return err
	}
	v := vault.New(args[0])
	if err := v.Unlock(pw); err != nil {
		return err
	}
	defer v.Lock()
	paths, err := v.ExtractAll(args[1], overwrite)
	if err != nil {
		return err
	}
	fmt.Fprintf(out(), "Restored %d file(s) to %s\n", len(paths), args[1])
	return nil
}

func cmdDelete(args []string) error {
	pw, args, err := flagPassword(args)
	if err != nil {
		return err
	}
	if len(args) < 2 {
		return fmt.Errorf("usage: delete <vault> <name> [more names ...]")
	}
	pw, err = passwordOrPrompt(pw, "password: ")
	if err != nil {
		return err
	}
	v := vault.New(args[0])
	if err := v.Unlock(pw); err != nil {
		return err
	}
	defer v.Lock()
	names := args[1:]
	if err := v.DeleteFiles(names); err != nil {
		return err
	}
	if len(names) == 1 {
		printDeleted(names[0])
	} else {
		printDeletedBatch(len(names))
	}
	return nil
}

func flagBool(args []string, name string) (bool, []string) {
	found := false
	rest := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == name {
			found = true
			continue
		}
		rest = append(rest, arg)
	}
	return found, rest
}

func cmdMigrate(args []string) error {
	pw, args, err := flagPassword(args)
	if err != nil {
		return err
	}
	if len(args) < 1 {
		return fmt.Errorf("usage: migrate <vault>")
	}
	pw, err = passwordOrPrompt(pw, "legacy password: ")
	if err != nil {
		return err
	}
	v := vault.New(args[0])
	if err := v.Unlock(pw); err != nil {
		return err
	}
	defer v.Lock()
	if err := v.ChangePassword(pw, pw); err != nil {
		return err
	}
	printMigrated(v.Format)
	return nil
}

func cmdInfo(args []string) error {
	pw, args, err := flagPassword(args)
	if err != nil {
		return err
	}
	if len(args) < 1 {
		return fmt.Errorf("usage: info <vault>")
	}
	path := args[0]
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	rec, recErr := vault.PeekKDFProfile(path)
	prefix, pErr := vault.PeekCarrierPrefix(path)
	var unlocked *vault.Vault
	if pw != "" {
		v := vault.New(path)
		if uErr := v.Unlock(pw); uErr != nil {
			return uErr
		}
		defer v.Lock()
		unlocked = v
	}
	printInfo(path, st.Size(), rec, recErr == nil, recErr, prefix, pErr, unlocked)
	return nil
}
