package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 1×1 transparent PNG (valid, viewers accept trailing data after IEND).
var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
	0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// TestCLIRoundTrip drives the real pulse-vault entrypoint binary path via `go run`.
func TestCLIRoundTrip(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "cli.pulsevault")
	src := filepath.Join(dir, "payload.bin")
	payload := []byte("cli-round-trip-bytes-12345")
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	pw := "CLIRoundTripPassword!!"
	run := func(args ...string) (string, error) {
		cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
		cmd.Dir = "." // package dir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	out, err := run("create", vault, "--password", pw, "--profile", "fast")
	if err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	out, err = run("add", vault, "--password", pw, src)
	if err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	out, err = run("list", vault, "--password", pw)
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	if !strings.Contains(out, "payload.bin") {
		t.Fatalf("list missing file: %s", out)
	}
	outDir := filepath.Join(dir, "out")
	out, err = run("extract", vault, "--password", pw, "payload.bin", outDir)
	if err != nil {
		t.Fatalf("extract: %v\n%s", err, out)
	}
	got, err := os.ReadFile(filepath.Join(outDir, "payload.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("bytes mismatch: %q", got)
	}
}

func TestCLIWrongPassword(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "bad.pulsevault")
	run := func(args ...string) (string, error) {
		cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
		cmd.Dir = "."
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	out, err := run("create", vault, "--password", "GoodPasswordForCLI!!", "--profile", "fast")
	if err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	out, err = run("list", vault, "--password", "WrongPasswordForCLI!!")
	if err == nil {
		t.Fatalf("expected failure, got: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "invalid password") && !strings.Contains(strings.ToLower(out), "error") {
		t.Fatalf("unexpected error text: %s", out)
	}
}

func TestCLIVersionBanner(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "version")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatal(err, string(out))
	}
	s := string(out)
	if !strings.Contains(s, "Pulse-Vault") {
		t.Fatalf("missing product name: %s", out)
	}
	if !strings.Contains(s, "version:") {
		t.Fatalf("missing version line: %s", out)
	}
	if !strings.Contains(s, "V6") {
		t.Fatalf("missing V6 format mention: %s", out)
	}
}

func TestCLIHelpAndUnknown(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "help")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("help: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "create") || !strings.Contains(string(out), "extract") {
		t.Fatalf("help missing commands: %s", out)
	}

	cmd = exec.Command("go", "run", ".", "not-a-command")
	cmd.Dir = "."
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for unknown command, got: %s", out)
	}
	if !strings.Contains(strings.ToLower(string(out)), "unknown") {
		t.Fatalf("expected unknown command message: %s", out)
	}
}

func TestCLIInfo(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "info.pulsevault")
	run := func(args ...string) (string, error) {
		cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
		cmd.Dir = "."
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	out, err := run("create", vault, "--password", "InfoTestPassword!!", "--profile", "fast")
	if err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	out, err = run("info", vault)
	if err != nil {
		t.Fatalf("info: %v\n%s", err, out)
	}
	if !strings.Contains(out, "V6") || !strings.Contains(out, "path:") {
		t.Fatalf("info missing fields: %s", out)
	}
	if !strings.Contains(out, "kdf:") {
		t.Fatalf("info missing kdf: %s", out)
	}
}

func TestCLIVerifyAndChangePassword(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "full.pulsevault")
	src := filepath.Join(dir, "payload.bin")
	payload := []byte("cli-verify-change-password")
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	oldPW, newPW := "CLIOldPassword!!!!!", "CLINewPassword!!!!!"
	run := func(args ...string) (string, error) {
		cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
		cmd.Dir = "."
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("create", vault, "--password", oldPW, "--profile", "fast"); err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	if out, err := run("add", vault, "--password", oldPW, src); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	out, err := run("verify", vault, "--password", oldPW)
	if err != nil {
		t.Fatalf("verify: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Verify OK") {
		t.Fatalf("verify output: %s", out)
	}
	if out, err := run("change-password", vault, "--password", oldPW, "--new-password", newPW); err != nil {
		t.Fatalf("chpw: %v\n%s", err, out)
	}
	if out, err := run("list", vault, "--password", newPW); err != nil {
		t.Fatalf("list new: %v\n%s", err, out)
	}
	if out, err := run("list", vault, "--password", oldPW); err == nil {
		t.Fatalf("old password still works: %s", out)
	}
}

func TestCLIPasswordStdinAndExtractNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "stdin.pulsevault")
	src := filepath.Join(dir, "payload.txt")
	outDir := filepath.Join(dir, "out")
	if err := os.WriteFile(src, []byte("stdin-password-payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	pw := "PasswordFromPipe!!"
	run := func(stdin string, args ...string) (string, error) {
		cmd := exec.Command("go", append([]string{"run", "."}, args...)...)
		cmd.Dir = "."
		cmd.Stdin = strings.NewReader(stdin)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run(pw+"\n", "create", vault, "--password-stdin", "--profile", "fast"); err != nil {
		t.Fatalf("create via stdin: %v\n%s", err, out)
	}
	if out, err := run(pw+"\n", "add", vault, "--password-stdin", src); err != nil {
		t.Fatalf("add via stdin: %v\n%s", err, out)
	}
	if out, err := run(pw+"\n", "extract", vault, "--password-stdin", "payload.txt", outDir); err != nil {
		t.Fatalf("extract via stdin: %v\n%s", err, out)
	}
	if out, err := run(pw+"\n", "extract", vault, "--password-stdin", "payload.txt", outDir); err == nil {
		t.Fatalf("expected no-overwrite failure, got %s", out)
	}
	if out, err := run(pw+"\n", "extract", vault, "--password-stdin", "payload.txt", outDir, "--overwrite"); err != nil {
		t.Fatalf("extract overwrite: %v\n%s", err, out)
	}
}

func TestCLICarrierHideInPicture(t *testing.T) {
	dir := t.TempDir()
	cover := filepath.Join(dir, "cover.png")
	hidden := filepath.Join(dir, "hidden.png")
	src := filepath.Join(dir, "secret.bin")
	payload := []byte("hidden-in-picture-cli-bytes")
	if err := os.WriteFile(cover, tinyPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	pw := "CLICarrierPassword!!"

	out, err := runCLI(t, "create", hidden, "--carrier", cover, "--password", pw, "--profile", "fast")
	if err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	if !strings.Contains(strings.ToLower(out), "hidden") || !strings.Contains(out, hidden) {
		t.Fatalf("create should mention hidden vault dest: %s", out)
	}

	got, err := os.ReadFile(hidden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(got, tinyPNG[:8]) {
		t.Fatalf("dest must start with PNG magic, first bytes=%x", got[:8])
	}
	if len(got) <= len(tinyPNG) {
		t.Fatalf("dest size %d must exceed carrier %d", len(got), len(tinyPNG))
	}

	out, err = runCLI(t, "add", hidden, "--password", pw, src)
	if err != nil {
		t.Fatalf("add via picture: %v\n%s", err, out)
	}
	out, err = runCLI(t, "list", hidden, "--password", pw)
	if err != nil {
		t.Fatalf("list via picture: %v\n%s", err, out)
	}
	if !strings.Contains(out, "secret.bin") {
		t.Fatalf("list missing file: %s", out)
	}
	outDir := filepath.Join(dir, "out")
	out, err = runCLI(t, "extract", hidden, "--password", pw, "secret.bin", outDir)
	if err != nil {
		t.Fatalf("extract via picture: %v\n%s", err, out)
	}
	extracted, err := os.ReadFile(filepath.Join(outDir, "secret.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if string(extracted) != string(payload) {
		t.Fatalf("bytes mismatch: %q", extracted)
	}

	out, err = runCLI(t, "info", hidden)
	if err != nil {
		t.Fatalf("info: %v\n%s", err, out)
	}
	if !strings.Contains(out, "carrier: yes") || !strings.Contains(out, "prefix=") {
		t.Fatalf("info missing carrier prefix: %s", out)
	}
}

func TestCLICreateBareNameAppendsPulsevault(t *testing.T) {
	dir := t.TempDir()
	bare := filepath.Join(dir, "myvault")
	out, err := runCLI(t, "create", bare, "--password", "BareNamePassword!!", "--profile", "fast")
	if err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	want := bare + ".pulsevault"
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected %s after bare create: %v\n%s", want, err, out)
	}
	if _, err := os.Stat(bare); err == nil {
		t.Fatalf("bare dest should not be written without .pulsevault suffix")
	}
}

func TestCLICarrierMissingFile(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.png")
	dest := filepath.Join(dir, "hidden.png")
	out, err := runCLI(t, "create", dest, "--carrier="+missing, "--password", "MissingCarrierPW!!", "--profile", "fast")
	if err == nil {
		t.Fatalf("expected failure for missing carrier, got: %s", out)
	}
	low := strings.ToLower(out)
	if !strings.Contains(low, "carrier") && !strings.Contains(low, "not found") && !strings.Contains(low, "no such") {
		t.Fatalf("unexpected missing-carrier error: %s", out)
	}
}

func TestCLIHelpMentionsCarrier(t *testing.T) {
	out, err := runCLI(t, "help")
	if err != nil {
		t.Fatalf("help: %v\n%s", err, out)
	}
	if !strings.Contains(out, "--carrier") {
		t.Fatalf("help missing --carrier: %s", out)
	}
	low := strings.ToLower(out)
	if !strings.Contains(low, "image") && !strings.Contains(low, "picture") {
		t.Fatalf("help should say dest can be an image: %s", out)
	}
}

func TestCLIDeleteAndOpenInventory(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "ops.pulsevault")
	src := filepath.Join(dir, "keep.bin")
	if err := os.WriteFile(src, []byte("keep-me"), 0o600); err != nil {
		t.Fatal(err)
	}
	pw := "CLIDeleteOpenPassword!!"
	if out, err := runCLI(t, "create", vault, "--password", pw, "--profile", "fast"); err != nil {
		t.Fatalf("create: %v\n%s", err, out)
	}
	if out, err := runCLI(t, "add", vault, "--password", pw, src); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	out, err := runCLI(t, "open", vault, "--password", pw)
	if err != nil {
		t.Fatalf("open: %v\n%s", err, out)
	}
	if !strings.Contains(out, "keep.bin") {
		t.Fatalf("open inventory missing file: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "unlocked") && !strings.Contains(out, "inventory") {
		t.Fatalf("open should look like a console inventory: %s", out)
	}
	out, err = runCLI(t, "delete", vault, "--password", pw, "keep.bin")
	if err != nil {
		t.Fatalf("delete: %v\n%s", err, out)
	}
	if !strings.Contains(strings.ToLower(out), "deleted") && !strings.Contains(out, "keep.bin") {
		t.Fatalf("delete output: %s", out)
	}
	out, err = runCLI(t, "list", vault, "--password", pw)
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	if strings.Contains(out, "keep.bin") && !strings.Contains(out, "(empty vault)") {
		t.Fatalf("file still listed after delete: %s", out)
	}
}

func TestCLIHelpMentionsOpenAndDelete(t *testing.T) {
	out, err := runCLI(t, "help")
	if err != nil {
		t.Fatalf("help: %v\n%s", err, out)
	}
	for _, n := range []string{"open", "delete", "create", "extract", "--carrier"} {
		if !strings.Contains(out, n) {
			t.Fatalf("help missing %q: %s", n, out)
		}
	}
}

func TestCLIWeakPasswordWarning(t *testing.T) {
	dir := t.TempDir()
	vault := filepath.Join(dir, "weak.pulsevault")
	weak := "aaaaaaaaaaaaaa"
	out, err := runCLI(t, "create", vault, "--password", weak, "--profile", "fast")
	if err != nil {
		t.Fatalf("weak password should still create: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(vault); statErr != nil {
		t.Fatalf("vault not created: %v", statErr)
	}
	low := strings.ToLower(out)
	if !strings.Contains(low, "warning") {
		t.Fatalf("expected weak-password warning, got: %s", out)
	}
	if strings.Contains(out, weak) {
		t.Fatalf("warning must not echo the password: %s", out)
	}

	out, err = runCLI(t, "change-password", vault, "--password", weak, "--new-password", "bbbbbbbbbbbbbb")
	if err != nil {
		t.Fatalf("change-password weak: %v\n%s", err, out)
	}
	if !strings.Contains(strings.ToLower(out), "warning") {
		t.Fatalf("expected weak-password warning on change-password: %s", out)
	}
	if strings.Contains(out, "bbbbbbbbbbbbbb") {
		t.Fatalf("change-password warning must not echo the password: %s", out)
	}
}
