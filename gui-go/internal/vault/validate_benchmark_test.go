package vault

import (
	"archive/zip"
	"fmt"
	"testing"
)

func TestValidateZipPathSegments(t *testing.T) {
	file := func(name string) *zip.File {
		return &zip.File{FileHeader: zip.FileHeader{
			Name:               name,
			Method:             zip.Store,
			UncompressedSize64: 1024,
		}}
	}
	for _, name := range []string{"data/abc.enc", "data/nested/abc.enc"} {
		if err := validateZip([]*zip.File{file(name)}); err != nil {
			t.Errorf("validateZip(%q) = %v, want valid path", name, err)
		}
	}
	for _, name := range []string{"data/../abc.enc", "data/nested/../../abc.enc", "data/..", "/data/abc.enc", `data\\abc.enc`} {
		if err := validateZip([]*zip.File{file(name)}); err == nil {
			t.Errorf("validateZip(%q) accepted an unsafe path", name)
		}
	}
}

func BenchmarkValidateZip(b *testing.B) {
	for _, size := range []int{1_000, 10_000} {
		files := make([]*zip.File, size)
		for i := range files {
			files[i] = &zip.File{FileHeader: zip.FileHeader{
				Name:               fmt.Sprintf("data/%032x.enc", i),
				Method:             zip.Store,
				UncompressedSize64: 1024,
			}}
		}
		b.Run(fmt.Sprintf("entries-%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if err := validateZip(files); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
