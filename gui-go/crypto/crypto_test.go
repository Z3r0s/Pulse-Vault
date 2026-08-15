package pulsecrypto_test

import (
	"bytes"
	"errors"
	"testing"

	pulsecrypto "github.com/Z3r0s/Pulse-Vault/gui-go/crypto"
)

func publicKey(t *testing.T) []byte {
	t.Helper()
	salt := bytes.Repeat([]byte{0x33}, pulsecrypto.SaltSize)
	profile := pulsecrypto.Profiles["fast"]
	key, err := pulsecrypto.DeriveKeyScrypt("public-api-test-password", salt, profile.N, profile.R, profile.P)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestPublicMetadataRoundTripAndTamper(t *testing.T) {
	key := publicKey(t)
	plain := []byte(`{"version":5,"files":{}}`)
	chachaNonce := bytes.Repeat([]byte{0x01}, pulsecrypto.NonceSize)
	aesNonce := bytes.Repeat([]byte{0x02}, pulsecrypto.NonceSize)
	ciphertext, err := pulsecrypto.EncryptDataV3WithNonces(key, plain, chachaNonce, aesNonce)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pulsecrypto.DecryptDataV3(key, chachaNonce, aesNonce, ciphertext)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("round trip failed: %v", err)
	}
	ciphertext[0] ^= 1
	if _, err := pulsecrypto.DecryptDataV3(key, chachaNonce, aesNonce, ciphertext); !errors.Is(err, pulsecrypto.ErrCrypto) {
		t.Fatalf("tamper error = %v, want ErrCrypto", err)
	}
}

func TestPublicEmptyPasswordRejected(t *testing.T) {
	salt := bytes.Repeat([]byte{0x33}, pulsecrypto.SaltSize)
	profile := pulsecrypto.Profiles["fast"]
	if _, err := pulsecrypto.DeriveKeyScrypt("", salt, profile.N, profile.R, profile.P); err == nil {
		t.Fatal("empty password must fail")
	}
}

func TestPublicLegacyKeyAndStreamingAPI(t *testing.T) {
	salt := bytes.Repeat([]byte{0x11}, pulsecrypto.SaltSize)
	legacyKey, err := pulsecrypto.DeriveKeyLegacy("legacy-api-password", salt)
	if err != nil {
		t.Fatal(err)
	}
	if len(legacyKey) != pulsecrypto.LegacyKeySize {
		t.Fatalf("legacy key length = %d", len(legacyKey))
	}

	key := publicKey(t)
	var encrypted bytes.Buffer
	if err := pulsecrypto.EncryptStreamV5(key, bytes.NewReader(bytes.Repeat([]byte("stream"), 2000)), &encrypted, true); err != nil {
		t.Fatal(err)
	}
	var decrypted bytes.Buffer
	if err := pulsecrypto.DecryptStreamV5(key, &encrypted, &decrypted); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted.Bytes(), bytes.Repeat([]byte("stream"), 2000)) {
		t.Fatal("public stream round trip mismatch")
	}
}
