package crypto

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestFileDigestHexMatchesKnownVector(t *testing.T) {
	// SHA-256("abc") from FIPS 180-2. This is a file fingerprint, not a KDF.
	got := FileDigestHex([]byte("abc"))
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if FileDigestHex(nil) != SHA256Hex(nil) {
		t.Fatal("SHA256Hex alias drifted")
	}
}

func TestFileDigestReaderMatchesHex(t *testing.T) {
	payload := bytes.Repeat([]byte("vault-file"), 100)
	fromBuf := FileDigestHex(payload)
	fromR, err := FileDigestReader(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if fromBuf != fromR {
		t.Fatalf("%s != %s", fromBuf, fromR)
	}
	if len(fromBuf) != hex.EncodedLen(32) {
		t.Fatalf("digest length %d", len(fromBuf))
	}
}

func TestPasswordsUseScryptNotFileDigest(t *testing.T) {
	salt := bytes.Repeat([]byte{0x42}, SaltSize)
	key, err := DeriveKeyScrypt("FileDigestIsNotAPassword!!", salt, 16, 8, 1)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(key) == FileDigestHex([]byte("FileDigestIsNotAPassword!!")) {
		t.Fatal("scrypt output must not equal raw SHA-256 of the password")
	}
}
