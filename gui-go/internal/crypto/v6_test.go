package crypto

import (
	"bytes"
	"testing"
)

func TestStreamV6RoundTripsAndFinalizes(t *testing.T) {
	key := testKey(t)
	payload := bytes.Repeat([]byte("Pulse-Vault V6 finalized stream\n"), 90_000)
	for _, compress := range []bool{false, true} {
		var encrypted bytes.Buffer
		if err := EncryptStreamV6(key, bytes.NewReader(payload), &encrypted, compress); err != nil {
			t.Fatalf("compress=%v encrypt: %v", compress, err)
		}
		if !bytes.HasPrefix(encrypted.Bytes(), StreamV6Magic) {
			t.Fatalf("compress=%v missing V6 magic", compress)
		}
		var decrypted bytes.Buffer
		if err := DecryptStreamV6(key, bytes.NewReader(encrypted.Bytes()), &decrypted); err != nil {
			t.Fatalf("compress=%v decrypt: %v", compress, err)
		}
		if !bytes.Equal(decrypted.Bytes(), payload) {
			t.Fatalf("compress=%v payload mismatch", compress)
		}
	}
}

func TestStreamV6RejectsMissingOrTamperedTerminal(t *testing.T) {
	key := testKey(t)
	var encrypted bytes.Buffer
	if err := EncryptStreamV6(key, bytes.NewReader([]byte("terminal integrity")), &encrypted, true); err != nil {
		t.Fatal(err)
	}
	blob := encrypted.Bytes()
	for _, cut := range []int{len(blob) - 1, len(blob) - 8, len(blob) / 2} {
		var decrypted bytes.Buffer
		if err := DecryptStreamV6(key, bytes.NewReader(blob[:cut]), &decrypted); err == nil {
			t.Fatalf("truncation at %d was accepted", cut)
		}
	}

	tampered := append([]byte(nil), blob...)
	tampered[len(tampered)-1] ^= 0x01
	var decrypted bytes.Buffer
	if err := DecryptStreamV6(key, bytes.NewReader(tampered), &decrypted); err == nil {
		t.Fatal("tampered terminal was accepted")
	}
}

func TestStreamV6EmptyRoundTrip(t *testing.T) {
	key := testKey(t)
	var encrypted bytes.Buffer
	if err := EncryptStreamV6(key, bytes.NewReader(nil), &encrypted, true); err != nil {
		t.Fatal(err)
	}
	var decrypted bytes.Buffer
	if err := DecryptStreamV6(key, bytes.NewReader(encrypted.Bytes()), &decrypted); err != nil {
		t.Fatal(err)
	}
	if decrypted.Len() != 0 {
		t.Fatalf("got %d plaintext bytes", decrypted.Len())
	}
}

func TestScryptCombinedBudgetsRejectBeforeDerivation(t *testing.T) {
	salt := bytes.Repeat([]byte{0x42}, SaltSize)
	for _, params := range [][3]int{
		{1 << 20, 8, 2},  // memory exceeds the 1 GiB cap
		{1 << 20, 16, 1}, // memory exceeds the 1 GiB cap
		{1 << 20, 8, 16}, // CPU work exceeds the work cap
	} {
		if _, err := DeriveKeyScrypt("budget-test-password", salt, params[0], params[1], params[2]); err == nil {
			t.Fatalf("parameters %v were accepted", params)
		}
	}
}
