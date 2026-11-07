package crypto

import (
	"bytes"
	"encoding/binary"
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

func TestStreamV6RejectsCleanChunkBoundaryTruncationAndReordering(t *testing.T) {
	key := testKey(t)
	payload := bytes.Repeat([]byte{0x5a}, ChunkSize*2+17)
	var encrypted bytes.Buffer
	if err := EncryptStreamV6(key, bytes.NewReader(payload), &encrypted, false); err != nil {
		t.Fatal(err)
	}
	blob := encrypted.Bytes()
	first := len(StreamV6Magic) + 1 + 16 + NonceSize
	if blob[first] != 0 {
		t.Fatalf("first record kind = %d, want data", blob[first])
	}
	firstLen := int(binary.BigEndian.Uint32(blob[first+1 : first+5]))
	second := first + 5 + firstLen
	if second >= len(blob) || blob[second] != 0 {
		t.Fatal("second data record missing")
	}
	secondLen := int(binary.BigEndian.Uint32(blob[second+1 : second+5]))
	third := second + 5 + secondLen
	if third >= len(blob) || blob[third] != 0 {
		t.Fatal("third data record missing")
	}
	thirdLen := int(binary.BigEndian.Uint32(blob[third+1 : third+5]))
	terminal := third + 5 + thirdLen
	if terminal >= len(blob) || blob[terminal] != 1 {
		t.Fatal("terminal record missing")
	}

	// Truncating after a complete data record is the case that a plain EOF
	// framed stream cannot distinguish from a successful shorter stream.
	var out bytes.Buffer
	if err := DecryptStreamV6(key, bytes.NewReader(blob[:second]), &out); err == nil {
		t.Fatal("clean-boundary truncation was accepted")
	}

	// Records are authenticated with their position, so swapping complete
	// ciphertext records must fail even though their framing remains valid.
	reordered := append([]byte(nil), blob...)
	firstRecord := append([]byte(nil), blob[first:second]...)
	secondRecord := append([]byte(nil), blob[second:terminal]...)
	reordered = append(reordered[:first], append(secondRecord, append(firstRecord, reordered[terminal:]...)...)...)
	out.Reset()
	if err := DecryptStreamV6(key, bytes.NewReader(reordered), &out); err == nil {
		t.Fatal("reordered data records were accepted")
	}

	trailing := append(append([]byte(nil), blob...), 0)
	out.Reset()
	if err := DecryptStreamV6(key, bytes.NewReader(trailing), &out); err == nil {
		t.Fatal("trailing bytes after terminal were accepted")
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
