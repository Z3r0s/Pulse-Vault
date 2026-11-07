package vault

import (
	"archive/zip"
	"bytes"
	"context"
	crand "crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFolderImportAndExtractAllPreservesTree(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "folder.pulsevault")
	input := filepath.Join(dir, "project")
	if err := os.MkdirAll(filepath.Join(input, "docs", "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	wantGuide := []byte("A small project guide with exact bytes.\x00\xff\n")
	wantImage := bytes.Repeat([]byte{0, 1, 2, 255}, 2048)
	if err := os.WriteFile(filepath.Join(input, "docs", "nested", "guide.bin"), wantGuide, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "image.dat"), wantImage, 0o600); err != nil {
		t.Fatal(err)
	}

	v := New(vaultPath)
	if err := v.Create("FolderRoundTripPassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	if err := v.AddDirectory(input, false); err != nil {
		t.Fatal(err)
	}
	if v.Format != FormatV7 {
		t.Fatalf("directory import format = %q, want V7", v.Format)
	}
	names, err := v.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	wantNames := map[string]bool{"docs/nested/guide.bin": true, "image.dat": true}
	for _, name := range names {
		delete(wantNames, name)
	}
	if len(wantNames) != 0 {
		t.Fatalf("missing imported paths: %v; actual=%v", wantNames, names)
	}

	v.Lock()
	v = New(vaultPath)
	if err := v.Unlock("FolderRoundTripPassword!!"); err != nil {
		t.Fatalf("reopen V7: %v", err)
	}
	if files, _, hashes, err := v.VerifyAll(); err != nil || files != 2 || hashes != 2 {
		t.Fatalf("verify all: files=%d hashes=%d err=%v", files, hashes, err)
	}
	output := filepath.Join(dir, "restored")
	paths, err := v.ExtractAll(output, false)
	if err != nil {
		t.Fatalf("extract all: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("extracted %d files, want 2", len(paths))
	}
	gotGuide, err := os.ReadFile(filepath.Join(output, "docs", "nested", "guide.bin"))
	if err != nil {
		t.Fatal(err)
	}
	gotImage, err := os.ReadFile(filepath.Join(output, "image.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotGuide, wantGuide) || !bytes.Equal(gotImage, wantImage) {
		t.Fatal("folder round trip changed file bytes")
	}
}

func TestFolderImportUpgradesV5WithoutBreakingExistingStreams(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "mixed.pulsevault")
	oldPassword := "MixedFormatPassword!!"
	v := New(vaultPath)
	if err := v.Create(oldPassword, "fast"); err != nil {
		t.Fatal(err)
	}
	v.Format = FormatV5
	v.meta.Version = 5
	if err := v.writeVault(nil); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(dir, "legacy.txt")
	legacyBytes := []byte("retained V5 stream")
	if err := os.WriteFile(legacyPath, legacyBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(legacyPath, false); err != nil {
		t.Fatal(err)
	}
	if v.meta.Files["legacy.txt"].StreamFormat != "" {
		t.Fatal("V5 container should use its container marker for existing streams")
	}

	folder := filepath.Join(dir, "tree")
	if err := os.MkdirAll(filepath.Join(folder, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	newBytes := []byte("new V6 finalized stream")
	if err := os.WriteFile(filepath.Join(folder, "child", "new.txt"), newBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddDirectory(folder, false); err != nil {
		t.Fatal(err)
	}
	if got := v.meta.Files["legacy.txt"].StreamFormat; got != FormatV5 {
		t.Fatalf("legacy stream marker = %q, want V5", got)
	}
	if got := v.meta.Files["child/new.txt"].StreamFormat; got != FormatV6 {
		t.Fatalf("new stream marker = %q, want V6", got)
	}
	v.Lock()
	v = New(vaultPath)
	if err := v.Unlock(oldPassword); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := v.VerifyAll(); err != nil {
		t.Fatalf("mixed V5/V6 V7 verification: %v", err)
	}
	newPassword := "MixedFormatRotatedPass!!"
	if err := v.ChangePassword(oldPassword, newPassword); err != nil {
		t.Fatalf("rotate mixed-format V7 vault: %v", err)
	}
	if v.Format != FormatV7 {
		t.Fatalf("password rotation changed V7 path semantics: %q", v.Format)
	}
	v.Lock()
	v = New(vaultPath)
	if err := v.Unlock(newPassword); err != nil {
		t.Fatalf("reopen rotated V7: %v", err)
	}
	if _, _, _, err := v.VerifyAll(); err != nil {
		t.Fatalf("rotated V7 verification: %v", err)
	}
	output := filepath.Join(dir, "out")
	if _, err := v.ExtractAll(output, false); err != nil {
		t.Fatal(err)
	}
	gotOld, err := os.ReadFile(filepath.Join(output, "legacy.txt"))
	if err != nil {
		t.Fatal(err)
	}
	gotNew, err := os.ReadFile(filepath.Join(output, "child", "new.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotOld, legacyBytes) || !bytes.Equal(gotNew, newBytes) {
		t.Fatal("mixed-format upgrade changed file contents")
	}
}

func TestExtractAllPreflightsEveryDestination(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "preflight.pulsevault")
	v := New(vaultPath)
	if err := v.Create("ExtractPreflightPassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		source := filepath.Join(dir, name)
		if err := os.WriteFile(source, []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := v.AddFile(source, false); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(dir, "out")
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "b.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := v.ExtractAll(output, false); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected preflight collision, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(output, "a.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preflight conflict left partial plaintext a.txt: %v", err)
	}
	kept, err := os.ReadFile(filepath.Join(output, "b.txt"))
	if err != nil || string(kept) != "keep" {
		t.Fatalf("existing destination changed: %q %v", kept, err)
	}
}

func TestExtractAllCancellationKeepsCompletedFilesAndRemovesPartial(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "cancel-extract-all.pulsevault")
	v := New(vaultPath)
	if err := v.Create("CancelExtractPassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	partial := make([]byte, 2*1024*1024)
	if _, err := crand.Read(partial); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string][]byte{
		"a-complete.txt": []byte("first file is complete"),
		"b-partial.bin":  partial,
	} {
		source := filepath.Join(dir, name)
		if err := os.WriteFile(source, content, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := v.AddFile(source, false); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var cancelled bool
	outputDir := filepath.Join(dir, "restored")
	paths, err := v.ExtractAllWithOptions(outputDir, false, OperationOptions{
		Context: ctx,
		Progress: func(update OperationUpdate) {
			if !cancelled && update.Phase == "Extracting files" && update.Done > int64(len("first file is complete")) {
				cancelled = true
				cancel()
			}
		},
	})
	if !cancelled || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation during second file, reported=%v err=%v", cancelled, err)
	}
	if len(paths) != 1 || filepath.Base(paths[0]) != "a-complete.txt" {
		t.Fatalf("completed paths after cancellation = %v", paths)
	}
	got, err := os.ReadFile(filepath.Join(outputDir, "a-complete.txt"))
	if err != nil || string(got) != "first file is complete" {
		t.Fatalf("completed first file = %q, err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "b-partial.bin")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("partial second file was published: %v", err)
	}
	for _, entry := range mustReadDir(t, outputDir) {
		if strings.HasSuffix(entry.Name(), ".part") {
			t.Fatalf("cancelled extraction left partial file %q", entry.Name())
		}
	}
}

func TestAddCancellationDuringRewriteKeepsOriginalVault(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "cancel-add.pulsevault")
	v := New(vaultPath)
	if err := v.Create("CancelAddPassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	large := make([]byte, 4*1024*1024)
	if _, err := crand.Read(large); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(dir, "first.bin")
	if err := os.WriteFile(first, large, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(first, false); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(dir, "second.bin")
	if err := os.WriteFile(second, bytes.Repeat([]byte("new payload"), 100_000), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var cancelled bool
	err = v.AddPathsWithOptions([]string{second}, false, OperationOptions{
		Context: ctx,
		Progress: func(update OperationUpdate) {
			if !cancelled && update.Phase == "Rebuilding vault" && update.Done > 0 {
				cancelled = true
				cancel()
			}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation while copying archive data, got %v", err)
	}
	after, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("cancelled add replaced the original vault")
	}
	finalizeCtx, cancelFinalize := context.WithCancel(context.Background())
	var finalizeReported bool
	err = v.AddPathsWithOptions([]string{second}, false, OperationOptions{
		Context: finalizeCtx,
		Progress: func(update OperationUpdate) {
			if update.Phase == "Finalizing vault" {
				finalizeReported = true
				cancelFinalize()
			}
		},
	})
	cancelFinalize()
	if !finalizeReported || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation at commit boundary, reported=%v err=%v", finalizeReported, err)
	}
	afterFinalizeCancel, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, afterFinalizeCancel) {
		t.Fatal("cancelled finalization replaced the original vault")
	}
	if v.FileCount() != 1 {
		t.Fatalf("cancelled add left %d metadata entries", v.FileCount())
	}
	if _, err := v.VerifyFile("first.bin"); err != nil {
		t.Fatalf("existing file failed after cancelled add: %v", err)
	}
	for _, entry := range mustReadDir(t, dir) {
		if strings.HasPrefix(entry.Name(), ".pulsevault-") || strings.HasPrefix(entry.Name(), ".cancel-add.pulsevault.") {
			t.Fatalf("temporary file left after cancellation: %s", entry.Name())
		}
	}
}

func TestDeleteCancellationDuringRewriteKeepsOriginalVault(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "cancel-delete.pulsevault")
	v := New(vaultPath)
	if err := v.Create("CancelDeletePassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"remove.txt": strings.Repeat("remove me", 20_000),
		"keep.txt":   strings.Repeat("keep me", 40_000),
	} {
		source := filepath.Join(dir, name)
		if err := os.WriteFile(source, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := v.AddFile(source, false); err != nil {
			t.Fatal(err)
		}
	}
	before, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var cancelled bool
	err = v.DeleteFileWithOptions("remove.txt", OperationOptions{
		Context: ctx,
		Progress: func(update OperationUpdate) {
			if !cancelled && update.Phase == "Rebuilding vault" && update.Done > 0 {
				cancelled = true
				cancel()
			}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected delete cancellation during rewrite, got %v", err)
	}
	after, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("cancelled delete replaced the original vault")
	}
	if v.FileCount() != 2 {
		t.Fatalf("cancelled delete left %d in-memory entries", v.FileCount())
	}
	if _, err := v.VerifyFile("keep.txt"); err != nil {
		t.Fatalf("kept entry failed after cancellation: %v", err)
	}
	for _, entry := range mustReadDir(t, dir) {
		if strings.HasPrefix(entry.Name(), ".cancel-delete.pulsevault.") {
			t.Fatalf("temporary rewrite file left after cancellation: %s", entry.Name())
		}
	}
}

func TestFolderImportRejectsSymlinks(t *testing.T) {
	dir := t.TempDir()
	v := New(filepath.Join(dir, "sym.pulsevault"))
	if err := v.Create("FolderSymlinkPassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "folder")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(folder, "link.txt")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if err := v.AddDirectory(folder, false); err == nil || !strings.Contains(strings.ToLower(err.Error()), "symbolic link") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
	if v.FileCount() != 0 {
		t.Fatalf("failed folder import changed vault metadata: %d entries", v.FileCount())
	}
}

func TestFolderUpgradeRejectsUnportableExistingNamesWithoutChangingVault(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "unportable.pulsevault")
	v := New(vaultPath)
	if err := v.Create("UnportableNamePassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "valid.txt")
	if err := os.WriteFile(source, []byte("existing content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(source, false); err != nil {
		t.Fatal(err)
	}
	meta := v.meta.Files["valid.txt"]
	delete(v.meta.Files, "valid.txt")
	meta.Name = "bad:name.txt"
	v.meta.Files[meta.Name] = meta
	if err := v.writeVault(nil); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "tree")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "child.txt"), []byte("new content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddDirectory(folder, false); err == nil || !strings.Contains(err.Error(), "not a portable relative path") {
		t.Fatalf("expected path upgrade rejection, got %v", err)
	}
	if v.Format != FormatV6 || v.FileCount() != 1 {
		t.Fatalf("failed upgrade changed in-memory vault: format=%q files=%d", v.Format, v.FileCount())
	}
	after, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed path upgrade changed the on-disk vault")
	}
}

func TestV1UnlockChecksSizeBeforeReadingPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oversized-v1")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte(FormatV1)); err != nil {
		t.Fatal(err)
	}
	max := int64(MaxMetadataSize + len(FormatV1) + 16 + 12)
	if err := f.Truncate(max + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	v := New(path)
	if err := v.Unlock("AnyPasswordForLegacy!!"); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("expected early size rejection, got %v", err)
	}
}

func TestPasswordRotationCancellationAndSuccessUseNoPlaintextTemps(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "rotate.pulsevault")
	oldPassword, newPassword := "RotationOldPassword!!", "RotationNewPassword!!"
	v := New(vaultPath)
	if err := v.Create(oldPassword, "fast"); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "large.bin")
	want := bytes.Repeat([]byte("stream-me-"), 700_000)
	if err := os.WriteFile(source, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(source, false); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	var cancelled bool
	err = v.ChangePasswordWithOptions(oldPassword, newPassword, OperationOptions{
		Context: ctx,
		Progress: func(update OperationUpdate) {
			if !cancelled && strings.HasPrefix(update.Phase, "Re-encrypting ") && update.Done > 0 {
				cancelled = true
				cancel()
			}
		},
	})
	cancel()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation during streamed rotation, got %v", err)
	}
	afterCancel, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, afterCancel) {
		t.Fatal("cancelled password rotation changed the original vault")
	}
	for _, entry := range mustReadDir(t, dir) {
		if strings.Contains(entry.Name(), ".pulsevault-plain-") {
			t.Fatalf("plaintext staging file found: %s", entry.Name())
		}
	}
	if _, err := v.VerifyFile("large.bin"); err != nil {
		t.Fatalf("old vault no longer verifies after cancellation: %v", err)
	}
	if err := v.ChangePassword(oldPassword, newPassword); err != nil {
		t.Fatalf("streamed password rotation: %v", err)
	}
	v.Lock()
	if err := New(vaultPath).Unlock(oldPassword); err == nil {
		t.Fatal("old password still unlocks after successful rotation")
	}
	v = New(vaultPath)
	if err := v.Unlock(newPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := v.VerifyFile("large.bin"); err != nil {
		t.Fatalf("rotated entry failed verification: %v", err)
	}
	output, err := v.ExtractFile("large.bin", filepath.Join(dir, "out"), false)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("rotated stream changed content: %v", err)
	}
}

func TestPasswordRotationRejectsTamperedStreamWithoutReplacingVault(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "tampered-stream.pulsevault")
	oldPassword, newPassword := "TamperRotateOldPass!!", "TamperRotateNewPass!!"
	v := New(vaultPath)
	if err := v.Create(oldPassword, "fast"); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(dir, "payload.txt")
	if err := os.WriteFile(source, bytes.Repeat([]byte("authenticated"), 10_000), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(source, false); err != nil {
		t.Fatal(err)
	}
	blob := "data/" + v.meta.Files["payload.txt"].InternalID + ".enc"
	if err := mutateZipEntry(t, vaultPath, blob, func(data []byte) []byte {
		data[len(data)-1] ^= 0x80
		return data
	}); err != nil {
		t.Fatal(err)
	}
	tampered, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.ChangePassword(oldPassword, newPassword); err == nil {
		t.Fatal("password rotation accepted a tampered stream")
	}
	after, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(tampered, after) {
		t.Fatal("failed rotation replaced the tampered original vault")
	}
	for _, entry := range mustReadDir(t, dir) {
		if strings.Contains(entry.Name(), ".pulsevault-plain-") {
			t.Fatalf("plaintext staging file found: %s", entry.Name())
		}
	}
}

func TestTamperedStreamFailsVerifyAndNeverPublishesExtractedPlaintext(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "tampered-extract.pulsevault")
	password := "TamperedExtractPassword!!"
	v := New(vaultPath)
	if err := v.Create(password, "fast"); err != nil {
		t.Fatal(err)
	}
	want := bytes.Repeat([]byte("authenticated content"), 20_000)
	source := filepath.Join(dir, "secret.bin")
	if err := os.WriteFile(source, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(source, false); err != nil {
		t.Fatal(err)
	}
	blobName := "data/" + v.meta.Files["secret.bin"].InternalID + ".enc"
	if err := mutateZipEntry(t, vaultPath, blobName, func(data []byte) []byte {
		data[len(data)-1] ^= 0x40
		return data
	}); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	v = New(vaultPath)
	if err := v.Unlock(password); err != nil {
		t.Fatalf("metadata should still unlock: %v", err)
	}
	if _, err := v.VerifyFile("secret.bin"); err == nil {
		t.Fatal("verify accepted a tampered stream")
	}
	outputDir := filepath.Join(dir, "output")
	if _, err := v.ExtractFile("secret.bin", outputDir, false); err == nil {
		t.Fatal("extract accepted a tampered stream")
	}
	if _, err := os.Stat(filepath.Join(outputDir, "secret.bin")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("tampered extraction published output: %v", err)
	}
	for _, entry := range mustReadDir(t, outputDir) {
		if strings.HasSuffix(entry.Name(), ".part") {
			t.Fatalf("tampered extraction left partial plaintext %q", entry.Name())
		}
	}
}

func TestV7PathValidationRejectsTraversalAndWindowsDevices(t *testing.T) {
	for _, name := range []string{"../secret", "a/../../secret", "/rooted", `a\\b`, "C:/drive", "docs/CON.txt", "docs/trailing."} {
		if _, err := safeVaultPath(name); err == nil {
			t.Errorf("accepted unsafe path %q", name)
		}
	}
	if got, err := safeVaultPath("docs/guide.txt"); err != nil || got != "docs/guide.txt" {
		t.Fatalf("valid path result=%q err=%v", got, err)
	}
	for _, names := range [][]string{
		{"Readme.txt", "README.TXT"},
		{"Docs", "docs/guide.txt"},
		{"a/b.txt", "a/B.txt"},
	} {
		if err := validateVaultPathSet(names); err == nil {
			t.Errorf("accepted conflicting path set %q", names)
		}
	}
}

func TestUnlockRejectsAuthenticatedUnsafeV7Path(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unsafe-v7.pulsevault")
	pw := "UnsafeV7PathPassword!!"
	v := New(path)
	if err := v.Create(pw, "fast"); err != nil {
		t.Fatal(err)
	}
	v.Format = FormatV7
	v.meta.Version = 7
	v.meta.Files["../outside.txt"] = FileMeta{Name: "../outside.txt", Size: 0, Type: "file"}
	if err := v.writeVault(nil); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	if err := New(path).Unlock(pw); err == nil || !strings.Contains(err.Error(), "invalid V7 paths") {
		t.Fatalf("expected unsafe authenticated path rejection, got %v", err)
	}
}

func TestUnlockRejectsV7ContainerMarkerDowngrade(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "downgraded-v7.pulsevault")
	pw := "V7DowngradePassword!!"
	v := New(path)
	if err := v.Create(pw, "fast"); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "tree")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "nested.txt"), []byte("nested"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddDirectory(folder, false); err != nil {
		t.Fatal(err)
	}
	if err := mutateZipEntry(t, path, "format.txt", func([]byte) []byte { return []byte(FormatV6) }); err != nil {
		t.Fatal(err)
	}
	if err := New(path).Unlock(pw); err == nil || !strings.Contains(err.Error(), "marker does not match") {
		t.Fatalf("expected V7 marker downgrade rejection, got %v", err)
	}
}

func mustReadDir(t *testing.T, dir string) []os.DirEntry {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func mutateZipEntry(t *testing.T, path, target string, mutate func([]byte) []byte) error {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".vault-rewrite-*.tmp")
	if err != nil {
		_ = zr.Close()
		return err
	}
	tmpName := tmp.Name()
	zw := zip.NewWriter(tmp)
	found := false
	for _, member := range zr.File {
		r, err := member.Open()
		if err != nil {
			_ = zw.Close()
			_ = tmp.Close()
			_ = zr.Close()
			_ = os.Remove(tmpName)
			return err
		}
		data, readErr := io.ReadAll(r)
		closeErr := r.Close()
		if readErr != nil || closeErr != nil {
			_ = zw.Close()
			_ = tmp.Close()
			_ = zr.Close()
			_ = os.Remove(tmpName)
			if readErr != nil {
				return readErr
			}
			return closeErr
		}
		if member.Name == target {
			data = mutate(data)
			found = true
		}
		header := member.FileHeader
		header.Method = zip.Store
		w, err := zw.CreateHeader(&header)
		if err == nil {
			_, err = w.Write(data)
		}
		if err != nil {
			_ = zw.Close()
			_ = tmp.Close()
			_ = zr.Close()
			_ = os.Remove(tmpName)
			return err
		}
	}
	if err := zw.Close(); err != nil {
		_ = tmp.Close()
		_ = zr.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = zr.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = zr.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := zr.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if !found {
		_ = os.Remove(tmpName)
		return os.ErrNotExist
	}
	return replaceFile(tmpName, path)
}
