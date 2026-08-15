package crypto

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestScryptFastVector(t *testing.T) {
	// Matches tests/vectors/scrypt_fast.json + vector_constants.VECTOR_TEST_PASSWORD
	password := "vector-test-password!"
	salt, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	wantKey, _ := hex.DecodeString("e1e88dd057658b4ae8e20287b3bdfcc8757307cff7fa1697cc2a4da7396a322e1c43ef72c2b635123394868e5b157ef8d3dc003393490f8aecd3cebd9cc6143e")

	key, err := DeriveKeyScrypt(password, salt, 16, 8, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(key, wantKey) {
		t.Fatalf("key mismatch\n got %x\nwant %x", key, wantKey)
	}
	chacha, aes, err := SplitV3Key(key)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(chacha) != "e1e88dd057658b4ae8e20287b3bdfcc8757307cff7fa1697cc2a4da7396a322e" {
		t.Fatal("chacha key split mismatch")
	}
	if hex.EncodeToString(aes) != "1c43ef72c2b635123394868e5b157ef8d3dc003393490f8aecd3cebd9cc6143e" {
		t.Fatal("aes key split mismatch")
	}
}

func TestMetadataV3FastVector(t *testing.T) {
	// Prefer repo vector file when present (run from gui-go).
	vectorPath := filepath.Join("..", "..", "..", "tests", "vectors", "metadata_v3_fast.json")
	raw, err := os.ReadFile(vectorPath)
	if err != nil {
		t.Skip("metadata vector not available:", err)
	}
	var vec struct {
		SaltHex        string `json:"salt_hex"`
		PlaintextHex   string `json:"plaintext_hex"`
		ChaChaNonceHex string `json:"chacha_nonce_hex"`
		AESNonceHex    string `json:"aes_nonce_hex"`
		CiphertextHex  string `json:"ciphertext_hex"`
	}
	if err := json.Unmarshal(raw, &vec); err != nil {
		t.Fatal(err)
	}
	password := "vector-test-password!"
	salt, _ := hex.DecodeString(vec.SaltHex)
	key, err := DeriveKeyScrypt(password, salt, 16, 8, 1)
	if err != nil {
		t.Fatal(err)
	}
	cN, _ := hex.DecodeString(vec.ChaChaNonceHex)
	aN, _ := hex.DecodeString(vec.AESNonceHex)
	ct, _ := hex.DecodeString(vec.CiphertextHex)
	plain, err := DecryptDataV3(key, cN, aN, ct)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := hex.DecodeString(vec.PlaintextHex)
	if !bytes.Equal(plain, want) {
		t.Fatalf("plaintext mismatch")
	}
}

func TestStreamRoundTrip(t *testing.T) {
	password := "stream-roundtrip-password!!"
	salt := bytes.Repeat([]byte{0x42}, SaltSize)
	key, err := DeriveKeyScrypt(password, salt, 16, 8, 1)
	if err != nil {
		t.Fatal(err)
	}
	payload := []byte("Pulse-Vault Go stream payload\nwith some data")
	var enc bytes.Buffer
	if err := EncryptStreamV5(key, bytes.NewReader(payload), &enc, true); err != nil {
		t.Fatal(err)
	}
	var dec bytes.Buffer
	if err := DecryptStreamV5(key, bytes.NewReader(enc.Bytes()), &dec); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dec.Bytes(), payload) {
		t.Fatalf("round-trip failed:\n got %q\nwant %q", dec.Bytes(), payload)
	}
}

func TestStreamWrongKeyFails(t *testing.T) {
	salt := bytes.Repeat([]byte{0x11}, SaltSize)
	key, err := DeriveKeyScrypt("correct-password!!!!!", salt, 16, 8, 1)
	if err != nil {
		t.Fatal(err)
	}
	wrong, err := DeriveKeyScrypt("wrong-password!!!!!!!", salt, 16, 8, 1)
	if err != nil {
		t.Fatal(err)
	}
	var enc bytes.Buffer
	if err := EncryptStreamV5(key, bytes.NewReader([]byte("secret")), &enc, true); err != nil {
		t.Fatal(err)
	}
	var dec bytes.Buffer
	if err := DecryptStreamV5(wrong, bytes.NewReader(enc.Bytes()), &dec); err == nil {
		t.Fatal("expected decrypt failure with wrong key")
	}
}
