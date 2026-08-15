package pulsecrypto_test

import (
	"bytes"
	"testing"

	pulsecrypto "github.com/Z3r0s/Pulse-Vault/gui-go/crypto"
)

func FuzzDecryptStreamV5NeverPanics(f *testing.F) {
	key := bytes.Repeat([]byte{0x5a}, pulsecrypto.V3KeySize)
	var seed bytes.Buffer
	if err := pulsecrypto.EncryptStreamV5(key, bytes.NewReader([]byte("fuzz seed")), &seed, true); err != nil {
		f.Fatal(err)
	}
	f.Add(seed.Bytes())
	f.Add([]byte("PV5STRM1"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		var plaintext bytes.Buffer
		_ = pulsecrypto.DecryptStreamV5(key, bytes.NewReader(data), &plaintext)
	})
}

func FuzzDecryptStreamV4NeverPanics(f *testing.F) {
	key := bytes.Repeat([]byte{0xa5}, pulsecrypto.V3KeySize)
	f.Add([]byte{})
	f.Add(bytes.Repeat([]byte{0}, 32))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		var plaintext bytes.Buffer
		_ = pulsecrypto.DecryptStreamV4(key, bytes.NewReader(data), &plaintext)
	})
}
