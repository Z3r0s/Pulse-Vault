package vault

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaseFoldKeyMatchesEqualFold(t *testing.T) {
	pairs := [][2]string{
		{"Documents/Annual.TXT", "documents/annual.txt"},
		{"ſample.txt", "Sample.TXT"},
		{"Key.txt", "key.TXT"},
		{"Σummary.txt", "ςUMMARY.TXT"},
	}
	for _, pair := range pairs {
		if !strings.EqualFold(pair[0], pair[1]) {
			t.Fatalf("test pair %q and %q should be EqualFold", pair[0], pair[1])
		}
		if caseFoldKey(pair[0]) != caseFoldKey(pair[1]) {
			t.Errorf("case-fold keys differ for %q and %q", pair[0], pair[1])
		}
	}
	if strings.EqualFold("İ.txt", "i.txt") {
		t.Fatal("dotted capital I should not EqualFold to plain i")
	}
	if caseFoldKey("İ.txt") == caseFoldKey("i.txt") {
		t.Fatal("case-fold key merged distinct dotted-I paths")
	}
}

func TestVaultPathSetUsesUnicodeCaseFolding(t *testing.T) {
	if err := validateVaultPathSet([]string{"ſample.txt", "Sample.TXT"}); err == nil {
		t.Fatal("case-fold-equivalent paths must conflict")
	}
	if err := validateVaultPathSet([]string{"İ.txt", "i.txt"}); err != nil {
		t.Fatalf("distinct dotted-I paths should remain valid: %v", err)
	}
}

func TestAddPathUsesIndexedUnicodeCaseConflicts(t *testing.T) {
	dir := t.TempDir()
	v := New(filepath.Join(dir, "indexed-conflict.pulsevault"))
	if err := v.Create("PathIndexPassword!!", "fast"); err != nil {
		t.Fatal(err)
	}
	defer v.Lock()
	// Simulate a legacy/corrupt metadata entry so this checks add-time conflict
	// detection before the operation rewrites any vault contents.
	v.meta.Files["ſample.txt"] = FileMeta{Name: "ſample.txt"}
	source := filepath.Join(dir, "SAMPLE.txt")
	if err := os.WriteFile(source, []byte("collision"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := v.AddFile(source, false); err == nil {
		t.Fatal("add should reject an EqualFold-equivalent existing path")
	}
}

func BenchmarkAddPathConflictValidation(b *testing.B) {
	const existingCount = 10000
	const incomingCount = 100
	existing := make(map[string]FileMeta, existingCount)
	incoming := make([]string, incomingCount)
	for i := 0; i < existingCount; i++ {
		name := fmt.Sprintf("Archive/Group-%02d/Item-%05d.bin", i%100, i)
		existing[name] = FileMeta{Name: name}
	}
	for i := range incoming {
		incoming[i] = fmt.Sprintf("Archive/Group-%02d/Incoming-%05d.bin", i%100, i)
	}

	b.Run("nested-scan", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, name := range incoming {
				for existingName := range existing {
					if strings.EqualFold(existingName, name) {
						b.Fatal("unexpected conflict")
					}
				}
			}
		}
	})

	b.Run("folded-index", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			index, ambiguous := indexExistingPaths(existing)
			for _, name := range incoming {
				if _, conflict := existingPathConflict(caseFoldKey(name), name, index, ambiguous); conflict {
					b.Fatal("unexpected conflict")
				}
			}
		}
	})
}
