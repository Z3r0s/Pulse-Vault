package vault

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateAddListExtractRoundTrip(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "test.pulsevault")
	password := "GoVaultTestPassword123!"

	v := New(vaultPath)
	if err := v.Create(password, "fast"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if !v.IsUnlocked() {
		t.Fatal("expected unlocked after create")
	}

	src := filepath.Join(dir, "hello.txt")
	payload := []byte("hello from go vault round-trip\n")
	if err := os.WriteFile(src, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatalf("add: %v", err)
	}

	names, err := v.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "hello.txt" {
		t.Fatalf("list = %v", names)
	}

	outDir := filepath.Join(dir, "out")
	outPath, err := v.ExtractFile("hello.txt", outDir, false)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("extracted mismatch: %q", got)
	}

	// Re-open from disk
	v.Lock()
	v2 := New(vaultPath)
	if err := v2.Unlock(password); err != nil {
		t.Fatalf("re-unlock: %v", err)
	}
	names2, err := v2.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names2) != 1 {
		t.Fatalf("list after reopen: %v", names2)
	}
	out2, err := v2.ExtractFile("hello.txt", filepath.Join(dir, "out2"), false)
	if err != nil {
		t.Fatal(err)
	}
	got2, _ := os.ReadFile(out2)
	if string(got2) != string(payload) {
		t.Fatalf("reopen extract mismatch: %q", got2)
	}
}

func TestCarrierPrefixSurvivesRewrite(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "carrier.pulsevault")
	password := "CarrierPrefixPassword!!"
	v := New(vaultPath)
	if err := v.Create(password, "fast"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	prefix := []byte("carrier-prefix-bytes\x00\x01\x02")
	if err := os.WriteFile(vaultPath, append(append([]byte{}, prefix...), raw...), 0o600); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	if err := v.Unlock(password); err != nil {
		t.Fatalf("unlock prefixed vault: %v", err)
	}
	src := filepath.Join(dir, "carrier.txt")
	if err := os.WriteFile(src, []byte("carrier payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatalf("rewrite prefixed vault: %v", err)
	}
	after, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(after, prefix) {
		t.Fatalf("carrier prefix was not preserved")
	}
}

func TestWrongPasswordFails(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "wrongpw.pulsevault")
	v := New(vaultPath)
	if err := v.Create("CorrectLongPassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	v.Lock()

	v2 := New(vaultPath)
	err := v2.Unlock("WrongPasswordEntirely!!")
	if err == nil {
		t.Fatal("expected unlock failure with wrong password")
	}
	if v2.IsUnlocked() {
		t.Fatal("vault must remain locked after failed unlock")
	}
}

func TestCreateRefusesExisting(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "exists.pulsevault")
	v := New(vaultPath)
	if err := v.Create("SomeLongPassword1234!", "fast"); err != nil {
		t.Fatal(err)
	}
	v.Lock()
	v2 := New(vaultPath)
	if err := v2.Create("SomeLongPassword1234!", "fast"); err == nil {
		t.Fatal("expected create to fail when vault exists")
	}
}

func TestCreateClearsEmptyFileSavePlaceholder(t *testing.T) {
	// Mirrors Fyne ShowFileSave: OS creates an empty file before Create runs.
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "from-dialog.pulsevault")
	if err := os.WriteFile(vaultPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	v := New(vaultPath)
	if err := v.Create("PlaceholderDialogPassword!!", "fast"); err != nil {
		t.Fatalf("Create should succeed after empty placeholder: %v", err)
	}
	if !v.IsUnlocked() {
		t.Fatal("expected unlocked after create")
	}
	st, err := os.Stat(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() == 0 {
		t.Fatal("vault file should be non-empty after create")
	}
}

func TestCreateStillRefusesNonEmptyExisting(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "occupied.pulsevault")
	if err := os.WriteFile(vaultPath, []byte("not-a-vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	v := New(vaultPath)
	if err := v.Create("SomeLongPassword1234!", "fast"); err == nil {
		t.Fatal("expected create to refuse non-empty existing file")
	}
}

func TestAddFileStoresBasenameOnlyAndNoHostPathInCleartext(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "Users", "jared", "Documents", "Coding Projects", "OpenSourceFileVault")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(nested, "notes.txt")
	if err := os.WriteFile(src, []byte("only the bytes belong in the vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	vp := filepath.Join(dir, "private.pulsevault")
	v := New(vp)
	if err := v.Create("PrivacySealPassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatal(err)
	}
	names, err := v.ListFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "notes.txt" {
		t.Fatalf("stored name = %v (must be basename only)", names)
	}
	meta, err := v.GetFileMeta("notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(meta.Name, string(filepath.Separator)) || strings.Contains(meta.Name, "jared") {
		t.Fatalf("metadata name leaked a path: %q", meta.Name)
	}
	raw, err := os.ReadFile(vp)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"jared", "Coding Projects", "OpenSourceFileVault", "Users", nested} {
		if bytes.Contains(raw, []byte(needle)) {
			t.Fatalf("vault cleartext contains host path fragment %q", needle)
		}
	}
}

func TestZipMembersUseNeutralTimestamp(t *testing.T) {
	dir := t.TempDir()
	vp := filepath.Join(dir, "times.pulsevault")
	v := New(vp)
	if err := v.Create("ZipTimePassword!!!!", "fast"); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(vp)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if !f.Modified.IsZero() && f.Modified.Unix() != 0 {
			t.Fatalf("ZIP member %s Modified=%v (must not leak host clock)", f.Name, f.Modified)
		}
		if len(f.Comment) != 0 {
			t.Fatalf("ZIP member %s has a comment", f.Name)
		}
	}
}

func TestAddExtractEmptyFile(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "empty-entry.pulsevault")
	v := New(vaultPath)
	if err := v.Create("EmptyEntryPassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "blank.txt")
	if err := os.WriteFile(src, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatalf("add empty: %v", err)
	}
	out, err := v.ExtractFile("blank.txt", filepath.Join(dir, "out"), false)
	if err != nil {
		t.Fatalf("extract empty: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want empty file, got %d bytes", len(got))
	}
	if _, _, _, err := v.VerifyAll(); err != nil {
		t.Fatal(err)
	}
}

func TestPeekKDFOnCarrierPicture(t *testing.T) {
	dir := t.TempDir()
	carrier := filepath.Join(dir, "cover.png")
	if err := os.WriteFile(carrier, tinyPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "hidden.png")
	v := New(out)
	if err := v.CreateWithCarrier("PeekCarrierKdfPass!!", "fast", carrier); err != nil {
		t.Fatal(err)
	}
	rec, err := PeekKDFProfile(out)
	if err != nil {
		t.Fatalf("peek kdf on picture vault: %v", err)
	}
	if rec.Profile != "fast" || rec.N != 16 {
		t.Fatalf("%+v", rec)
	}
	off, err := PeekCarrierPrefix(out)
	if err != nil {
		t.Fatal(err)
	}
	if off != int64(len(tinyPNG)) {
		t.Fatalf("prefix=%d", off)
	}
}

func TestAddFileRefusesSelf(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "self.pulsevault")
	v := New(vaultPath)
	if err := v.Create("SelfAddPassword1234!", "fast"); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(vaultPath, false); err == nil {
		t.Fatal("adding the vault into itself must fail")
	}
}

func TestAddFileRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "nosym.pulsevault")
	v := New(vaultPath)
	if err := v.Create("NoSymlinkPassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(real, []byte("real"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink not permitted: %v", err)
	}
	if err := v.AddFile(link, false); err == nil {
		t.Fatal("adding a symlink must fail")
	}
}

func TestAddSecondFileDoesNotDropFirst(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "two.pulsevault")
	password := "TwoFileStreamCopyPass!!"
	v := New(vaultPath)
	if err := v.Create(password, "fast"); err != nil {
		t.Fatal(err)
	}
	aPath := filepath.Join(dir, "a.txt")
	bPath := filepath.Join(dir, "b.txt")
	aPayload := []byte("first-file-must-survive-second-add")
	bPayload := []byte("second-file-payload")
	if err := os.WriteFile(aPath, aPayload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bPath, bPayload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(aPath, false); err != nil {
		t.Fatalf("add a.txt: %v", err)
	}
	if err := v.AddFile(bPath, false); err != nil {
		t.Fatalf("add b.txt: %v", err)
	}

	v.Lock()
	v2 := New(vaultPath)
	if err := v2.Unlock(password); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	outDir := filepath.Join(dir, "out")
	gotA, err := v2.ExtractFile("a.txt", outDir, false)
	if err != nil {
		t.Fatalf("extract a.txt: %v", err)
	}
	gotB, err := v2.ExtractFile("b.txt", outDir, false)
	if err != nil {
		t.Fatalf("extract b.txt: %v", err)
	}
	aGot, err := os.ReadFile(gotA)
	if err != nil {
		t.Fatal(err)
	}
	bGot, err := os.ReadFile(gotB)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(aGot, aPayload) {
		t.Fatalf("a.txt mismatch: %q", aGot)
	}
	if !bytes.Equal(bGot, bPayload) {
		t.Fatalf("b.txt mismatch: %q", bGot)
	}
}

func TestExtractDetectsHashMismatch(t *testing.T) {
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "hashbad.pulsevault")
	v := New(vaultPath)
	if err := v.Create("HashMismatchPassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(src, []byte("payload-for-hash-check"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(src, false); err != nil {
		t.Fatal(err)
	}
	meta := v.meta.Files["a.txt"]
	meta.SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
	v.meta.Files["a.txt"] = meta
	outDir := filepath.Join(dir, "out")
	if _, err := v.ExtractFile("a.txt", outDir, false); err == nil {
		t.Fatal("expected hash mismatch")
	}
	if _, err := os.Stat(filepath.Join(outDir, "a.txt")); !os.IsNotExist(err) {
		t.Fatal("mismatched extract must not leave the output file")
	}
}

func TestVaultZipUsesStoreOnly(t *testing.T) {
	// Python core rejects non-ZIP_STORED members; ensure we stay interoperable.
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "store.pulsevault")
	v := New(vaultPath)
	if err := v.Create("StoreOnlyPassword1234!", "fast"); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(f, st.Size())
	if err != nil {
		t.Fatal(err)
	}
	for _, zf := range zr.File {
		if zf.Method != zip.Store {
			t.Fatalf("entry %s method=%d want Store", zf.Name, zf.Method)
		}
	}
}
