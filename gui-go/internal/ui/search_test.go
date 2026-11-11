package ui

import (
	"fmt"
	"strings"
	"testing"
)

func TestFilterFileNamesUsesCaseInsensitivePathsAndReusesBuffers(t *testing.T) {
	names := []string{"Documents/Annual.TXT", "Photos/BIRDS.jpg", "Readme.md"}
	folded, filtered := filterFileNames(names, nil, ".", nil)
	if len(filtered) != len(names) {
		t.Fatalf("full-match result = %v", filtered)
	}
	foldedPointer := &folded[0]
	filteredPointer := &filtered[0]

	folded, filtered = filterFileNames(names, folded, "  pHoTo  ", filtered)
	if len(filtered) != 1 || filtered[0] != "Photos/BIRDS.jpg" {
		t.Fatalf("filtered = %v", filtered)
	}
	if len(folded) != len(names) {
		t.Fatalf("folded name index = %v", folded)
	}
	if &folded[0] != foldedPointer || &filtered[0] != filteredPointer {
		t.Fatal("repeat filtering should reuse both the folded-name index and result buffer")
	}
	for index, stale := range filtered[:cap(filtered)][len(filtered):] {
		if stale != "" {
			t.Fatalf("filtered buffer retained a stale path at offset %d: %q", index+len(filtered), stale)
		}
	}

	folded, filtered = filterFileNames(names, folded, "DOC", filtered)
	if len(filtered) != 1 || filtered[0] != "Documents/Annual.TXT" {
		t.Fatalf("second filtered result = %v", filtered)
	}

	folded, filtered = filterFileNames(names, folded, "", filtered)
	if folded != nil || len(filtered) != len(names) {
		t.Fatalf("clearing the query should restore all files and release the folded-name cache: folded=%v filtered=%v", folded, filtered)
	}
}

func benchmarkFileNames(count int) []string {
	names := make([]string, count)
	for i := range names {
		names[i] = fmt.Sprintf("Documents/Quarterly/Customer-%08d/Statement.PDF", i)
	}
	return names
}

var fileNameFilterBenchmarkSink []string

func BenchmarkFileNameFilter(b *testing.B) {
	for _, count := range []int{10000, 100000} {
		b.Run(fmt.Sprintf("legacy/%d", count), func(b *testing.B) {
			names := benchmarkFileNames(count)
			query := "statement.pdf"
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				filtered := make([]string, 0, len(names))
				for _, name := range names {
					if strings.Contains(strings.ToLower(name), query) {
						filtered = append(filtered, name)
					}
				}
				fileNameFilterBenchmarkSink = filtered
			}
		})

		b.Run(fmt.Sprintf("indexed-reuse/%d", count), func(b *testing.B) {
			names := benchmarkFileNames(count)
			folded := make([]string, len(names))
			for i, name := range names {
				folded[i] = strings.ToLower(name)
			}
			filtered := make([]string, 0, len(names))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				folded, filtered = filterFileNames(names, folded, "statement.pdf", filtered)
				fileNameFilterBenchmarkSink = filtered
			}
		})

		b.Run(fmt.Sprintf("build-index/%d", count), func(b *testing.B) {
			names := benchmarkFileNames(count)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				folded, _ := filterFileNames(names, nil, "statement.pdf", nil)
				if len(folded) != count {
					b.Fatal("index build failed")
				}
				fileNameFilterBenchmarkSink = folded
			}
		})
	}
}
