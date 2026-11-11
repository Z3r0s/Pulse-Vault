package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChangePasswordCanRaiseKDFProfileWithoutPlaintextExport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile-upgrade.pulsevault")
	oldPassword := "OriginalFastProfilePassphrase!"
	newPassword := "NewStrongProfilePassphrase2026!"
	v := New(path)
	if err := v.Create(oldPassword, "fast"); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(source, []byte("authenticated profile rotation"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(source, false); err != nil {
		t.Fatal(err)
	}
	if err := v.ChangePasswordWithProfile(oldPassword, newPassword, "standard"); err != nil {
		t.Fatal(err)
	}
	record, err := PeekKDFProfile(path)
	if err != nil {
		t.Fatal(err)
	}
	if record.Profile != "standard" || record.N != 1<<15 || record.R != 8 || record.P != 1 {
		t.Fatalf("rotated KDF profile = %+v", record)
	}
	v.Lock()
	v = New(path)
	if err := v.Unlock(newPassword); err != nil {
		t.Fatalf("new password did not unlock upgraded profile: %v", err)
	}
	if _, _, _, err := v.VerifyAll(); err != nil {
		t.Fatalf("profile upgrade damaged authenticated streams: %v", err)
	}
	v.Lock()
	if err := New(path).Unlock(oldPassword); err == nil {
		t.Fatal("old password must not unlock after password and KDF rotation")
	}
}

func TestChangePasswordRejectsUnknownKDFProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile-invalid.pulsevault")
	v := New(path)
	if err := v.Create("ProfileValidationPassphrase!", "fast"); err != nil {
		t.Fatal(err)
	}
	if err := v.ChangePasswordWithProfile("ProfileValidationPassphrase!", "ReplacementPassphrase!2026", "unknown"); err == nil {
		t.Fatal("unknown profile should be rejected")
	}
	if v.Profile != "fast" {
		t.Fatalf("failed profile change mutated the vault profile: %q", v.Profile)
	}
}
