package crypto

import (
	"bytes"
	cryptorand "crypto/rand"
	"io"
	"testing"
)

func BenchmarkEncryptStreamV6(b *testing.B) {
	key := bytes.Repeat([]byte{0x42}, V3KeySize)
	compressibleSmall := bytes.Repeat([]byte("pulse-vault-small-file-"), 180)
	compressibleLarge := bytes.Repeat([]byte("pulse-vault-stream-benchmark-"), (1<<20)/len("pulse-vault-stream-benchmark-"))
	randomLarge := make([]byte, 10<<20)
	if _, err := cryptorand.Read(randomLarge); err != nil {
		b.Fatal(err)
	}

	workloads := []struct {
		name    string
		payload []byte
	}{
		{name: "small-repeated", payload: compressibleSmall},
		{name: "large-repeated", payload: compressibleLarge},
		{name: "large-random", payload: randomLarge},
	}
	for _, workload := range workloads {
		b.Run(workload.name, func(b *testing.B) {
			b.SetBytes(int64(len(workload.payload)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := EncryptStreamV6(key, bytes.NewReader(workload.payload), io.Discard, true); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
