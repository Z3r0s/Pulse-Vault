package vault

import (
	"bytes"
	"os"
	"path/filepath"
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

func writePNG(t *testing.T, path string, extra []byte) {
	t.Helper()
	if err := os.WriteFile(path, append(append([]byte{}, tinyPNG...), extra...), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCreateWithCarrierRoundTrip(t *testing.T) {
	dir := t.TempDir()
	carrier := filepath.Join(dir, "cover.png")
	writePNG(t, carrier, nil)
	out := filepath.Join(dir, "hidden.png")
	pw := "CarrierHidePassword!!"

	v := New(out)
	if err := v.CreateWithCarrier(pw, "fast", carrier); err != nil {
		t.Fatalf("create with carrier: %v", err)
	}
	if !v.HasCarrier() {
		t.Fatal("expected HasCarrier after create")
	}
	if v.CarrierPrefix() != int64(len(tinyPNG)) {
		t.Fatalf("prefix=%d want %d", v.CarrierPrefix(), len(tinyPNG))
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, tinyPNG) {
		t.Fatal("output must start with the carrier PNG so it still opens as a picture")
	}
	if len(raw) <= len(tinyPNG) {
		t.Fatal("vault ZIP must be appended after the picture")
	}

	src := filepath.Join(dir, "secret.txt")
	payload := []byte("hidden-inside-the-picture")
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatalf("add: %v", err)
	}
	v.Lock()

	re := New(out)
	if err := re.Unlock(pw); err != nil {
		t.Fatalf("unlock picture vault: %v", err)
	}
	if !re.HasCarrier() {
		t.Fatal("reopen must detect carrier")
	}
	names, err := re.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "secret.txt" {
		t.Fatalf("list=%v", names)
	}
	gotPath, err := re.ExtractFile("secret.txt", filepath.Join(dir, "out"), false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(gotPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("extracted %q", got)
	}
	after, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(after, tinyPNG) {
		t.Fatal("add/rewrite must keep the picture prefix")
	}
}

func TestCreateWithCarrierEmbedInPlace(t *testing.T) {
	dir := t.TempDir()
	picture := filepath.Join(dir, "photo.png")
	writePNG(t, picture, nil)
	pw := "EmbedInPlacePassword!!"

	v := New(picture)
	if err := v.CreateWithCarrier(pw, "fast", picture); err != nil {
		t.Fatalf("embed in place: %v", err)
	}
	raw, err := os.ReadFile(picture)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, tinyPNG) {
		t.Fatal("picture magic must survive in-place embed")
	}
	src := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(src, []byte("in-place"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	re := New(picture)
	if err := re.Unlock(pw); err != nil {
		t.Fatalf("unlock embedded picture: %v", err)
	}
	names, err := re.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "note.txt" {
		t.Fatalf("list=%v", names)
	}
}

func TestCarrierPrefixIgnoresFalseLocalHeaderInImage(t *testing.T) {
	dir := t.TempDir()
	// A PNG-like prefix that contains PK\x03\x04 — first-match scan would be wrong.
	poison := append([]byte{}, tinyPNG...)
	poison = append(poison, 0x50, 0x4b, 0x03, 0x04, 'f', 'a', 'k', 'e')
	carrier := filepath.Join(dir, "poison.png")
	if err := os.WriteFile(carrier, poison, 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "poison-out.png")
	pw := "PoisonPrefixPassword!!"
	v := New(out)
	if err := v.CreateWithCarrier(pw, "fast", carrier); err != nil {
		t.Fatal(err)
	}
	if v.CarrierPrefix() != int64(len(poison)) {
		t.Fatalf("prefix=%d want %d", v.CarrierPrefix(), len(poison))
	}
	src := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(src, []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatal(err)
	}
	off, err := PeekCarrierPrefix(out)
	if err != nil {
		t.Fatal(err)
	}
	if off != int64(len(poison)) {
		t.Fatalf("PeekCarrierPrefix=%d want %d (false PK in image must be ignored)", off, len(poison))
	}
	v.Lock()
	re := New(out)
	if err := re.Unlock(pw); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if re.CarrierPrefix() != int64(len(poison)) {
		t.Fatalf("reopen prefix=%d", re.CarrierPrefix())
	}
}

func TestCarrierSurvivesDeleteAndPasswordChange(t *testing.T) {
	dir := t.TempDir()
	carrier := filepath.Join(dir, "cover.png")
	writePNG(t, carrier, nil)
	out := filepath.Join(dir, "hidden.png")
	oldPW, newPW := "CarrierOldPassword!!", "CarrierNewPassword!!"
	v := New(out)
	if err := v.CreateWithCarrier(oldPW, "fast", carrier); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("keep-me"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatal(err)
	}
	src2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(src2, []byte("drop-me"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src2, false); err != nil {
		t.Fatal(err)
	}
	if err := v.DeleteFile("b.txt"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, tinyPNG) {
		t.Fatal("delete must keep picture prefix")
	}
	if err := v.ChangePassword(oldPW, newPW); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw, tinyPNG) {
		t.Fatal("password change must keep picture prefix")
	}
	v.Lock()
	if err := New(out).Unlock(oldPW); err == nil {
		t.Fatal("old password must fail")
	}
	re := New(out)
	if err := re.Unlock(newPW); err != nil {
		t.Fatal(err)
	}
	if !re.HasCarrier() {
		t.Fatal("carrier flag lost after password change")
	}
	names, err := re.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "a.txt" {
		t.Fatalf("list=%v", names)
	}
}

func TestCreateWithCarrierRejectsMissingAndEmpty(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "x.pulsevault")
	v := New(out)
	if err := v.CreateWithCarrier("PasswordForCarrier!!", "fast", ""); err == nil {
		t.Fatal("empty carrier path must fail")
	}
	if err := v.CreateWithCarrier("PasswordForCarrier!!", "fast", filepath.Join(dir, "missing.png")); err == nil {
		t.Fatal("missing carrier must fail")
	}
	empty := filepath.Join(dir, "empty.png")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.CreateWithCarrier("PasswordForCarrier!!", "fast", empty); err == nil {
		t.Fatal("empty carrier must fail")
	}
}

func TestCreateWithCarrierRefusesOccupiedDest(t *testing.T) {
	dir := t.TempDir()
	carrier := filepath.Join(dir, "c.png")
	writePNG(t, carrier, nil)
	dest := filepath.Join(dir, "occupied.png")
	if err := os.WriteFile(dest, []byte("already-here"), 0o600); err != nil {
		t.Fatal(err)
	}
	v := New(dest)
	if err := v.CreateWithCarrier("PasswordForCarrier!!", "fast", carrier); err == nil {
		t.Fatal("must not overwrite an unrelated existing dest")
	}
}
