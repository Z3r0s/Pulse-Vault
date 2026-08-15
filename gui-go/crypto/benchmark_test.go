package pulsecrypto_test

import (
	"bytes"
	"io"
	"testing"

	pulsecrypto "github.com/Z3r0s/Pulse-Vault/gui-go/crypto"
)

func benchmarkKey(b *testing.B) []byte {
	b.Helper()
	salt := bytes.Repeat([]byte{0x42}, pulsecrypto.SaltSize)
	profile := pulsecrypto.Profiles["fast"]
	key, err := pulsecrypto.DeriveKeyScrypt("benchmark-password", salt, profile.N, profile.R, profile.P)
	if err != nil {
		b.Fatal(err)
	}
	return key
}

func BenchmarkDeriveKeyScryptFast(b *testing.B) {
	salt := bytes.Repeat([]byte{0x42}, pulsecrypto.SaltSize)
	profile := pulsecrypto.Profiles["fast"]
	b.ReportMetric(float64(profile.N), "scrypt_N")
	b.ReportMetric(float64(profile.R), "scrypt_R")
	b.ReportMetric(float64(profile.P), "scrypt_P")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := pulsecrypto.DeriveKeyScrypt("benchmark-password", salt, profile.N, profile.R, profile.P); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncryptStreamV5(b *testing.B) {
	key := benchmarkKey(b)
	payload := bytes.Repeat([]byte("pulse-vault-benchmark-data-"), 2048)
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var encrypted bytes.Buffer
		if err := pulsecrypto.EncryptStreamV5(key, bytes.NewReader(payload), &encrypted, true); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecryptStreamV5(b *testing.B) {
	key := benchmarkKey(b)
	payload := bytes.Repeat([]byte("pulse-vault-benchmark-data-"), 2048)
	var encrypted bytes.Buffer
	if err := pulsecrypto.EncryptStreamV5(key, bytes.NewReader(payload), &encrypted, true); err != nil {
		b.Fatal(err)
	}
	ciphertext := append([]byte(nil), encrypted.Bytes()...)
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := pulsecrypto.DecryptStreamV5(key, bytes.NewReader(ciphertext), io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}
