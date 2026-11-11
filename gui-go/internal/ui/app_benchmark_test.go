package ui

import "testing"

func BenchmarkUpdateFileListRow(b *testing.B) {
	row := newFileListRow().(*fileListRow)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		updateFileListRow(row, "Documents/records.pdf", "2.4 MB · Added Nov 7")
	}
}
