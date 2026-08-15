package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/vault"
	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/version"
)

func out() io.Writer { return os.Stdout }

func printBanner() {
	ver := version.Version
	lines := []string{
		bold(fg("PULSE-VAULT")) + muted("   v"+ver),
		muted("offline sealed storage  ·  V5 cascade"),
	}
	fmt.Fprintln(out(), box("⬡  "+productName, lines, 56))
}

func printSlimHead(op string) {
	fmt.Fprintf(out(), "  %s  %s  %s  %s\n",
		glyph(),
		bold(cyan("pulse-vault")),
		muted("·"),
		muted(op),
	)
}

func printHelp() {
	printBanner()
	fmt.Fprintln(out())
	fmt.Fprintln(out(), "  "+bold(cyan("COMMANDS")))
	cmds := [][2]string{
		{"create", "Seal a new vault (or hide one inside a picture)"},
		{"open", "Interactive vault console"},
		{"list", "Inventory of sealed files"},
		{"add", "Encrypt a file into the vault"},
		{"extract", "Decrypt a file out to disk"},
		{"delete", "Remove a sealed file"},
		{"verify", "Integrity-check every entry"},
		{"info", "Inspect the container (no password)"},
		{"change-password", "Rotate the key (full re-encrypt)"},
		{"migrate", "Upgrade a legacy vault to V5"},
		{"version", "Print product version"},
		{"help", "This screen"},
	}
	for _, c := range cmds {
		fmt.Fprintf(out(), "    %s  %s\n", cyan(padRight(c[0], 18)), muted(c[1]))
	}
	fmt.Fprintln(out())
	fmt.Fprintln(out(), "  "+bold(cyan("USAGE")))
	usages := []string{
		"create  <vault> --password <pw> [--profile standard|fast|hardened] [--carrier <image>]",
		"open    <vault> --password <pw>",
		"list    <vault> --password <pw>",
		"add     <vault> --password <pw> <file> [--overwrite]",
		"extract <vault> --password <pw> <name> <outdir> [--overwrite]",
		"delete  <vault> --password <pw> <name>",
		"verify  <vault> --password <pw>",
		"change-password <vault> --password <old> --new-password <new>",
		"migrate <vault> --password <legacy-password>",
		"info    <vault> [--password <pw>]",
	}
	for _, u := range usages {
		fmt.Fprintf(out(), "    %s %s\n", muted("pulse-vault"), fg(u))
	}
	fmt.Fprintln(out())
	fmt.Fprintln(out(), "  "+muted("create destination may be a .pulsevault file or an image/video path (e.g. hidden.png)."))
	fmt.Fprintln(out(), "  "+muted("--carrier prepends that picture/video so the vault is hidden inside the file."))
	fmt.Fprintln(out(), "  "+muted("Use --password-stdin / --password-file to avoid exposing passwords in process listings."))
	fmt.Fprintln(out(), "  "+muted("--plain or NO_COLOR disables color. Primary product is this Go binary (and the desktop GUI)."))
}

func printVersion() {
	printBanner()
	fmt.Fprintln(out())
	fmt.Fprintln(out(), productName)
	fmt.Fprintf(out(), "version: %s\n", version.Version)
	fmt.Fprintln(out(), "Go native CLI — V5 encrypted vault")
}

func printCreated(path, profile string, hidden bool, elapsed time.Duration) {
	printSlimHead("create")
	lines := []string{
		markOK() + "  " + bold("vault sealed"),
		kv("path", path),
		kv("profile", profile),
		kv("format", "V5"),
		kv("elapsed", elapsed.Round(time.Millisecond).String()),
	}
	if hidden {
		lines = append(lines, kv("carrier", "yes — vault is hidden inside the picture"))
		lines = append(lines, muted("  still opens as the cover image"))
	}
	fmt.Fprintln(out(), box("create", lines, 62))
	if hidden {
		fmt.Fprintf(out(), "  %s Vault is hidden inside the picture: %s\n", glyph(), path)
	}
}

func printList(v *vault.Vault) error {
	printSlimHead("list")
	names, err := v.ListFiles()
	if err != nil {
		return err
	}
	title := "inventory  " + filepath.Base(v.Path)
	if v.HasCarrier() {
		title += "  ·  hidden"
	}
	if len(names) == 0 {
		fmt.Fprintln(out(), box(title, []string{
			muted("(empty vault)"),
			muted("add a file to seal it inside"),
		}, 64))
		return nil
	}
	lines := []string{
		muted(padRight("#", 4) + padRight("name", 28) + padRight("size", 10) + "added"),
	}
	var total int64
	for i, n := range names {
		meta, mErr := v.GetFileMeta(n)
		size := int64(0)
		when := "—"
		if mErr == nil {
			size = meta.Size
			if meta.AddedAt > 0 {
				when = time.Unix(meta.AddedAt, 0).Format("2006-01-02")
			}
		}
		total += size
		lines = append(lines, fmt.Sprintf("%s%s%s%s",
			muted(padRight(fmt.Sprintf("%d", i+1), 4)),
			fg(padRight(truncate(n, 27), 28)),
			padRight(humanSize(size), 10),
			muted(when),
		))
	}
	foot := fmt.Sprintf("%d files  ·  %s sealed", len(names), humanSize(total))
	if v.HasCarrier() {
		foot += fmt.Sprintf("  ·  hidden inside picture (%s prefix)", humanSize(v.CarrierPrefix()))
	}
	lines = append(lines, muted(foot))
	fmt.Fprintln(out(), box(title, lines, 68))
	return nil
}

func printAdded(name string, size int64, elapsed time.Duration) {
	printSlimHead("add")
	fmt.Fprintln(out(), box("add", []string{
		markOK() + "  " + bold("sealed  ") + fg(name),
		kv("size", humanSize(size)),
		kv("elapsed", elapsed.Round(time.Millisecond).String()),
	}, 56))
	fmt.Fprintf(out(), "Added %s\n", name)
}

func printExtracted(path string) {
	printSlimHead("extract")
	fmt.Fprintln(out(), box("extract", []string{
		markOK() + "  " + bold("unlocked to disk"),
		kv("out", path),
	}, 62))
	fmt.Fprintf(out(), "Extracted %s\n", path)
}

func printDeleted(name string) {
	printSlimHead("delete")
	fmt.Fprintln(out(), box("delete", []string{
		markOK() + "  " + bold("removed  ") + fg(name),
		muted("blob rewritten out of the container"),
	}, 56))
	fmt.Fprintf(out(), "Deleted %s\n", name)
}

func printVerified(fc int, bc int64, hc int) {
	printSlimHead("verify")
	fmt.Fprintln(out(), box("verify", []string{
		markOK() + "  " + bold("integrity holds"),
		kv("files", fmt.Sprintf("%d", fc)),
		kv("bytes", humanSize(bc)),
		kv("hashed", fmt.Sprintf("%d", hc)),
	}, 52))
	fmt.Fprintf(out(), "Verify OK: %d files, %d bytes checked, %d full hash-verified.\n", fc, bc, hc)
}

func printPasswordChanged() {
	printSlimHead("change-password")
	fmt.Fprintln(out(), box("key rotate", []string{
		markOK() + "  " + bold("password rotated"),
		muted("full re-encrypt with a new salt + key"),
	}, 52))
	fmt.Fprintln(out(), "Password rotated successfully (full re-encrypt with new key).")
}

func printMigrated() {
	printSlimHead("migrate")
	fmt.Fprintln(out(), box("migrate", []string{
		markOK() + "  " + bold("now V5"),
		muted("legacy container rewritten in place"),
	}, 52))
	fmt.Fprintln(out(), "Legacy vault migrated to the current V5 format.")
}

func printInfo(path string, size int64, rec vault.KDFRecord, recOK bool, recErr error, prefix int64, prefixErr error, unlocked *vault.Vault) {
	printSlimHead("info")
	lines := []string{
		kv("path", path),
		kv("size", fmt.Sprintf("%d bytes (%s)", size, humanSize(size))),
	}
	if recOK {
		lines = append(lines, kv("kdf", fmt.Sprintf("%s profile=%s n=%d r=%d p=%d", rec.Algorithm, rec.Profile, rec.N, rec.R, rec.P)))
		lines = append(lines, kv("format", "V5"))
	} else {
		lines = append(lines, kv("kdf", fmt.Sprintf("(unavailable: %v)", recErr)))
		lines = append(lines, kv("format", "legacy or V5 (unlock required for exact format)"))
	}
	if prefixErr != nil {
		lines = append(lines, kv("carrier", fmt.Sprintf("(unavailable: %v)", prefixErr)))
	} else if prefix > 0 {
		lines = append(lines, kv("carrier", fmt.Sprintf("yes, prefix=%d bytes", prefix)))
	} else {
		lines = append(lines, kv("carrier", "no"))
	}
	if unlocked != nil {
		lines = append(lines, kv("files", fmt.Sprintf("%d", unlocked.FileCount())))
		if unlocked.CreatedAt() > 0 {
			lines = append(lines, kv("created", time.Unix(unlocked.CreatedAt(), 0).Format(time.RFC3339)))
		}
		state := markOpen() + " unlocked"
		if unlocked.HasCarrier() {
			state += "  ·  hidden inside picture"
		}
		lines = append(lines, kv("state", state))
	} else {
		lines = append(lines, kv("state", markLock()+" locked  (pass --password to peek inside)"))
	}
	fmt.Fprintln(out(), box("container", lines, 72))
	// Keep machine-friendly keys for existing tests / scripts.
	fmt.Fprintf(out(), "  path: %s\n", path)
	fmt.Fprintf(out(), "  size: %d bytes\n", size)
	if recOK {
		fmt.Printf("  kdf: %s profile=%s n=%d r=%d p=%d\n", rec.Algorithm, rec.Profile, rec.N, rec.R, rec.P)
		fmt.Println("  format: V5")
	} else {
		fmt.Printf("  kdf: (unavailable: %v)\n", recErr)
		fmt.Println("  format: legacy or V5 (unlock required for exact format)")
	}
	if prefixErr != nil {
		fmt.Printf("  carrier: (unavailable: %v)\n", prefixErr)
	} else if prefix > 0 {
		fmt.Printf("  carrier: yes, prefix=%d bytes\n", prefix)
	} else {
		fmt.Println("  carrier: no")
	}
}

func printWarn(msg string) {
	fmt.Fprintf(os.Stderr, "%s  %s %s\n", markWarn(), yellow("warning"), msg)
}

func printError(err error) {
	fmt.Fprintf(os.Stderr, "%s  %s %v\n", markErr(), red("Error:"), err)
}

func printUnknown(cmd string) {
	fmt.Fprintf(os.Stderr, "%s  unknown command: %s\n", markErr(), cmd)
}

func formatWhen(ts int64) string {
	if ts <= 0 {
		return "—"
	}
	return time.Unix(ts, 0).Format("2006-01-02 15:04")
}

func consumeGlobals(args []string) []string {
	rest := make([]string, 0, len(args))
	for _, a := range args {
		switch a {
		case "--plain", "--no-color":
			plainMode = true
		default:
			rest = append(rest, a)
		}
	}
	return rest
}
