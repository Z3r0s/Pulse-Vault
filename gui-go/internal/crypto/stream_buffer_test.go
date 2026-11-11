package crypto

import (
	"bytes"
	"io"
	"testing"
)

type unsizedStreamReader struct{}

func (unsizedStreamReader) Read([]byte) (int, error) { return 0, io.EOF }

func TestStreamReaderSize(t *testing.T) {
	if got := streamReaderSize(bytes.NewReader([]byte("12345"))); got != 5 {
		t.Fatalf("streamReaderSize(bytes.Reader) = %d, want 5", got)
	}
	if got := streamReaderSize(unsizedStreamReader{}); got != -1 {
		t.Fatalf("streamReaderSize(unsized reader) = %d, want -1", got)
	}
}

func TestStreamBufferCapacity(t *testing.T) {
	tests := []struct {
		name       string
		size       int64
		compressed bool
		want       int
	}{
		{name: "unknown uncompressed", size: -1, want: streamInitialBufferSize},
		{name: "small compressed", size: 4096, compressed: true, want: 4096},
		{name: "large compressed", size: 8 << 20, compressed: true, want: streamInitialBufferSize},
		{name: "small uncompressed", size: 4096, want: 4096},
		{name: "large uncompressed", size: 8 << 20, want: ChunkSize},
		{name: "empty", size: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := streamBufferCapacity(tt.size, tt.compressed); got != tt.want {
				t.Fatalf("streamBufferCapacity(%d, %t) = %d, want %d", tt.size, tt.compressed, got, tt.want)
			}
		})
	}
}

func TestReusableStreamBuffersMatchProtocolHelpers(t *testing.T) {
	base := bytes.Repeat([]byte{0xa5}, 16)
	for _, index := range []uint32{0, 1, 255, 256, 1<<24 + 7, ^uint32(0)} {
		var got [NonceSize]byte
		fillChunkNonce(base, index, got[:])
		if want := chunkNonce(base, index); !bytes.Equal(got[:], want) {
			t.Fatalf("nonce for index %d = %x, want %x", index, got, want)
		}
	}

	cn := bytes.Repeat([]byte{1}, 16)
	an := bytes.Repeat([]byte{2}, 12)
	for _, index := range []uint32{0, 1, 0x12345678} {
		for _, kind := range []byte{0, 1} {
			got := appendStreamAAD(make([]byte, 0, 42), StreamV6Magic, compressZstd, cn, an, index, kind)
			want := streamAADFor(StreamV6Magic, compressZstd, cn, an, index, kind)
			if !bytes.Equal(got, want) {
				t.Fatalf("AAD for index %d kind %d = %x, want %x", index, kind, got, want)
			}
		}
	}
}
