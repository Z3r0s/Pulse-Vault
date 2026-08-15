package pulsecrypto_test

import (
	"bytes"
	"testing"

	pulsecrypto "github.com/Z3r0s/Pulse-Vault/gui-go/crypto"
)

func TestStreamExample(t *testing.T) {
	salt := bytes.Repeat([]byte{0x42}, pulsecrypto.SaltSize)
	profile := pulsecrypto.Profiles["fast"]
	key, err := pulsecrypto.DeriveKeyScrypt("example-password", salt, profile.N, profile.R, profile.P)
	if err != nil {
		t.Fatal(err)
	}

	var encrypted bytes.Buffer
	if err := pulsecrypto.EncryptStreamV5(key, bytes.NewReader([]byte("example")), &encrypted, true); err != nil {
		t.Fatal(err)
	}
	var decrypted bytes.Buffer
	if err := pulsecrypto.DecryptStreamV5(key, &encrypted, &decrypted); err != nil {
		t.Fatal(err)
	}
	if decrypted.String() != "example" {
		t.Fatalf("got %q", decrypted.String())
	}
}
