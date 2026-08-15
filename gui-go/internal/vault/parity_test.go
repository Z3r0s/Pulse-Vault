package vault

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	crypto "github.com/Z3r0s/Pulse-Vault/gui-go/crypto"
)

// Product-parity tests mirroring the Python suite categories (core lifecycle,
// change-password, verify, KDF persistence, tamper/security).

func TestChangePasswordReencryptsAndOldFails(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "chpw.pulsevault")
	oldPW := "OldPasswordForParity!!"
	newPW := "NewPasswordForParity!!"
	v := New(vp)
	if err := v.Create(oldPW, "fast"); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "a.txt")
	payload := []byte("change-password-payload-bytes")
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatal(err)
	}
	if err := v.ChangePassword(oldPW, newPW); err != nil {
		t.Fatal(err)
	}
	v.Lock()

	bad := New(vp)
	if err := bad.Unlock(oldPW); err == nil {
		t.Fatal("old password must fail after change")
	}
	good := New(vp)
	if err := good.Unlock(newPW); err != nil {
		t.Fatal(err)
	}
	out, err := good.ExtractFile("a.txt", filepath.Join(dir, "out"), true)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %q want %q", got, payload)
	}
	// KDF profile preserved
	rec, err := PeekKDFProfile(vp)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Profile != "fast" || rec.N != 16 {
		t.Fatalf("kdf after chpw: %+v", rec)
	}
}

func TestChangePasswordWrongOldRejected(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "badold.pulsevault")
	v := New(vp)
	if err := v.Create("CorrectOldPassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	if err := v.ChangePassword("WrongOldPassword!!!!!", "NewWhateverPass!!"); err == nil {
		t.Fatal("expected wrong old password to fail")
	}
}

func TestVerifyAllAndHashRecorded(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "verify.pulsevault")
	pw := "VerifyParityPassword!!"
	v := New(vp)
	if err := v.Create(pw, "fast"); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "hash-me.bin")
	payload := bytes.Repeat([]byte{0xab}, 4096)
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatal(err)
	}
	meta, err := v.GetFileMeta("hash-me.bin")
	if err != nil {
		t.Fatal(err)
	}
	if meta.SHA256 != crypto.SHA256Hex(payload) {
		t.Fatalf("sha mismatch recorded: %s", meta.SHA256)
	}
	res, err := v.VerifyFile("hash-me.bin")
	if err != nil {
		t.Fatal(err)
	}
	if !res.HashChecked || res.Size != int64(len(payload)) {
		t.Fatalf("%+v", res)
	}
	fc, bc, hc, err := v.VerifyAll()
	if err != nil || fc != 1 || bc != int64(len(payload)) || hc != 1 {
		t.Fatalf("verifyAll fc=%d bc=%d hc=%d err=%v", fc, bc, hc, err)
	}
}

func TestDeleteFileRemovesBlob(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "del.pulsevault")
	pw := "DeleteFilePassword!!!"
	v := New(vp)
	_ = v.Create(pw, "fast")
	src := filepath.Join(dir, "gone.txt")
	_ = os.WriteFile(src, []byte("bye"), 0o600)
	_ = v.AddFile(src, false)
	if err := v.DeleteFile("gone.txt"); err != nil {
		t.Fatal(err)
	}
	names, _ := v.ListFiles()
	if len(names) != 0 {
		t.Fatal(names)
	}
	// ZIP should not contain data/ for deleted file after rewrite
	zr, err := zip.OpenReader(vp)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if len(f.Name) > 5 && f.Name[:5] == "data/" {
			t.Fatalf("leftover blob %s", f.Name)
		}
	}
}

func TestKDFJSONWrittenAndUsedOnUnlock(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "kdf.pulsevault")
	pw := "KdfPersistPassword!!"
	v := New(vp)
	if err := v.Create(pw, "fast"); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	rec, err := PeekKDFProfile(vp)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Algorithm != "scrypt" || rec.Profile != "fast" || rec.N != 16 || rec.R != 8 || rec.P != 1 {
		t.Fatalf("%+v", rec)
	}
	// Unlock must honor kdf.json even if New defaults to standard
	v2 := New(vp)
	if err := v2.Unlock(pw); err != nil {
		t.Fatal(err)
	}
	if v2.Profile != "fast" || v2.KdfN != 16 {
		t.Fatalf("unlock kdf %+v n=%d", v2.Profile, v2.KdfN)
	}
}

func TestMetadataTamperFailsUnlock(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "tamper.pulsevault")
	pw := "TamperMetaPassword!!"
	v := New(vp)
	_ = v.Create(pw, "fast")
	v.Lock()

	// Corrupt metadata.enc inside zip by rewriting with bitflip
	raw, err := os.ReadFile(vp)
	if err != nil {
		t.Fatal(err)
	}
	// Crude: flip a mid byte of the file
	if len(raw) < 100 {
		t.Fatal("vault too small")
	}
	raw[len(raw)/2] ^= 0xff
	if err := os.WriteFile(vp, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	v2 := New(vp)
	if err := v2.Unlock(pw); err == nil {
		t.Fatal("expected unlock failure after tamper")
	}
}

func TestLockedVaultNoPasswordAttribute(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "mem.pulsevault")
	v := New(vp)
	_ = v.Create("MemorySafePassword!!", "fast")
	if !v.IsUnlocked() {
		t.Fatal("unlocked")
	}
	v.Lock()
	if v.IsUnlocked() || v.key != nil || v.salt != nil {
		t.Fatal("lock must clear key material")
	}
}

func TestMissingBlobFailsVerify(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "missing.pulsevault")
	pw := "MissingBlobPassword!!"
	v := New(vp)
	_ = v.Create(pw, "fast")
	src := filepath.Join(dir, "x.bin")
	_ = os.WriteFile(src, []byte("data"), 0o600)
	_ = v.AddFile(src, false)
	// Manually corrupt metadata to point at wrong internal id
	meta := v.meta.Files["x.bin"]
	meta.InternalID = "00000000-0000-4000-8000-000000000000"
	v.meta.Files["x.bin"] = meta
	// write metadata only by empty writeVault preserving wrong id without blob
	// Force rewrite without the blob
	_ = v.writeVault(map[string][]byte{})
	if _, err := v.VerifyFile("x.bin"); err == nil {
		t.Fatal("expected missing blob verify failure")
	}
}

func TestMultiFileChangePassword(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "multi.pulsevault")
	old, neu := "MultiOldPassword!!!", "MultiNewPassword!!!"
	v := New(vp)
	_ = v.Create(old, "fast")
	for i, name := range []string{"a.txt", "b.txt", "c.txt"} {
		p := filepath.Join(dir, name)
		_ = os.WriteFile(p, []byte{byte('A' + i)}, 0o600)
		if err := v.AddFile(p, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := v.ChangePassword(old, neu); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	v2 := New(vp)
	if err := v2.Unlock(neu); err != nil {
		t.Fatal(err)
	}
	names, _ := v2.ListFiles()
	if len(names) != 3 {
		t.Fatal(names)
	}
	for _, n := range names {
		if _, err := v2.VerifyFile(n); err != nil {
			t.Fatal(n, err)
		}
	}
}
