package crypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	salt := bytes.Repeat([]byte{0x33}, SaltSize)
	key, err := DeriveKeyScrypt("deep-test-password-123!", salt, 16, 8, 1)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func roundTrip(t *testing.T, key, plain []byte, compress bool) {
	t.Helper()
	var enc bytes.Buffer
	if err := EncryptStreamV5(key, bytes.NewReader(plain), &enc, compress); err != nil {
		t.Fatal(err)
	}
	var dec bytes.Buffer
	if err := DecryptStreamV5(key, bytes.NewReader(enc.Bytes()), &dec); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dec.Bytes(), plain) {
		t.Fatalf("round-trip mismatch len got=%d want=%d", len(dec.Bytes()), len(plain))
	}
}

func TestStreamMultiChunkUncompressed(t *testing.T) {
	key := testKey(t)
	// Larger than ChunkSize to force multi-chunk encrypt path
	plain := bytes.Repeat([]byte("x"), ChunkSize+100)
	roundTrip(t, key, plain, false)
}

func TestSealBufferReuseRoundTrip(t *testing.T) {
	key := testKey(t)
	// Two flushes so streamChunkWriter reuses inner/outer Seal buffers.
	const extra = 100
	plain := make([]byte, ChunkSize+extra)
	for i := range plain {
		plain[i] = byte(i) ^ byte(i>>8)
	}
	var enc bytes.Buffer
	if err := EncryptStreamV5(key, bytes.NewReader(plain), &enc, false); err != nil {
		t.Fatal(err)
	}
	var dec bytes.Buffer
	if err := DecryptStreamV5(key, bytes.NewReader(enc.Bytes()), &dec); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dec.Bytes(), plain) {
		t.Fatalf("seal buffer reuse round-trip mismatch got=%d want=%d", len(dec.Bytes()), len(plain))
	}
}

func TestStreamExactChunkBoundary(t *testing.T) {
	key := testKey(t)
	roundTrip(t, key, bytes.Repeat([]byte("b"), ChunkSize), false)
}

func TestStreamEmptyAndSmall(t *testing.T) {
	key := testKey(t)
	roundTrip(t, key, []byte{}, false)
	roundTrip(t, key, []byte{0x01}, true)
	roundTrip(t, key, bytes.Repeat([]byte("z"), 64), true)
}

func TestLooksIncompressible(t *testing.T) {
	text := bytes.Repeat([]byte("pulse-vault-benchmark-data-"), 200)
	if looksIncompressible(text) {
		t.Fatal("repeating text must stay compressible")
	}
	hi := make([]byte, 4096)
	for i := range hi {
		hi[i] = byte(i)
	}
	if !looksIncompressible(hi) {
		t.Fatal("256-symbol 4KiB sample should skip xz")
	}
}

func TestEncryptSkipsXZOnHighEntropy(t *testing.T) {
	key := testKey(t)
	plain := make([]byte, 64*1024)
	for i := range plain {
		plain[i] = byte(i)
	}
	var enc bytes.Buffer
	if err := EncryptStreamV5(key, bytes.NewReader(plain), &enc, true); err != nil {
		t.Fatal(err)
	}
	raw := enc.Bytes()
	if len(raw) < len(StreamV5Magic)+1 {
		t.Fatal("short stream")
	}
	if raw[len(StreamV5Magic)] != 0 {
		t.Fatalf("expected compress flag 0 after entropy skip, got %d", raw[len(StreamV5Magic)])
	}
	var dec bytes.Buffer
	if err := DecryptStreamV5(key, bytes.NewReader(raw), &dec); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dec.Bytes(), plain) {
		t.Fatal("round-trip after skip failed")
	}
}

func TestEncryptUsesZstdOnText(t *testing.T) {
	key := testKey(t)
	plain := bytes.Repeat([]byte("pulse-vault-benchmark-data-"), 400)
	var enc bytes.Buffer
	if err := EncryptStreamV5(key, bytes.NewReader(plain), &enc, true); err != nil {
		t.Fatal(err)
	}
	raw := enc.Bytes()
	if raw[len(StreamV5Magic)] != compressZstd {
		t.Fatalf("expected compress flag %d (zstd) for text, got %d", compressZstd, raw[len(StreamV5Magic)])
	}
	var dec bytes.Buffer
	if err := DecryptStreamV5(key, bytes.NewReader(raw), &dec); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dec.Bytes(), plain) {
		t.Fatal("text round-trip failed")
	}
}

func TestDecryptLegacyXZStillWorks(t *testing.T) {
	key := testKey(t)
	plain := bytes.Repeat([]byte("legacy-xz-payload-"), 80)
	var enc bytes.Buffer
	if err := encryptStreamV5XZ(key, bytes.NewReader(plain), &enc); err != nil {
		t.Fatal(err)
	}
	if enc.Bytes()[len(StreamV5Magic)] != compressXZ {
		t.Fatalf("legacy helper must write flag 1, got %d", enc.Bytes()[len(StreamV5Magic)])
	}
	var dec bytes.Buffer
	if err := DecryptStreamV5(key, bytes.NewReader(enc.Bytes()), &dec); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dec.Bytes(), plain) {
		t.Fatal("xz compat round-trip failed")
	}
}

func TestZstdRatioRepeatingVsEntropy(t *testing.T) {
	key := testKey(t)
	// 1 MiB of one byte — lossless compressors can shrink this a lot.
	// 50 GB → 2 MB is ~25,000:1 and only happens on data like this, not photos.
	repeat := bytes.Repeat([]byte{'A'}, 1<<20)
	var encRep bytes.Buffer
	if err := EncryptStreamV5(key, bytes.NewReader(repeat), &encRep, true); err != nil {
		t.Fatal(err)
	}
	if encRep.Len() >= len(repeat)/20 {
		t.Fatalf("repeating 1MiB should compress well, cipher stream is %d bytes", encRep.Len())
	}
	var dec bytes.Buffer
	if err := DecryptStreamV5(key, bytes.NewReader(encRep.Bytes()), &dec); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(dec.Bytes(), repeat) {
		t.Fatal("repeating payload round-trip failed")
	}

	entropy := sha256ExpandTest(1 << 20)
	var encEnt bytes.Buffer
	if err := EncryptStreamV5(key, bytes.NewReader(entropy), &encEnt, true); err != nil {
		t.Fatal(err)
	}
	if encEnt.Bytes()[len(StreamV5Magic)] != compressNone {
		t.Fatal("high-entropy 1MiB must skip compression")
	}
	if encEnt.Len() < len(entropy) {
		t.Fatalf("entropy stream unexpectedly smaller than plaintext (%d < %d)", encEnt.Len(), len(entropy))
	}
}

func sha256ExpandTest(n int) []byte {
	block := []byte("seed-pulse-vault-compare-v1")
	out := make([]byte, 0, n+32)
	for len(out) < n {
		sum := sha256.Sum256(block)
		block = sum[:]
		out = append(out, block...)
	}
	return out[:n]
}

func TestStreamCompressFlagVariants(t *testing.T) {
	key := testKey(t)
	plain := []byte("compressible text text text text text\n")
	var a, b bytes.Buffer
	_ = EncryptStreamV5(key, bytes.NewReader(plain), &a, true)
	_ = EncryptStreamV5(key, bytes.NewReader(plain), &b, false)
	if bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("compress true/false should differ on wire")
	}
	roundTrip(t, key, plain, true)
	roundTrip(t, key, plain, false)
}

func TestEmptyStreamRoundTripsBothModes(t *testing.T) {
	key := testKey(t)
	for _, compress := range []bool{false, true} {
		var encrypted bytes.Buffer
		if err := EncryptStreamV5(key, bytes.NewReader(nil), &encrypted, compress); err != nil {
			t.Fatalf("compress=%v encrypt: %v", compress, err)
		}
		if len(encrypted.Bytes()) <= len(StreamV5Magic)+1+16+NonceSize {
			t.Fatalf("compress=%v stream did not emit an authenticated record", compress)
		}
		var decrypted bytes.Buffer
		if err := DecryptStreamV5(key, bytes.NewReader(encrypted.Bytes()), &decrypted); err != nil {
			t.Fatalf("compress=%v decrypt: %v", compress, err)
		}
		if len(decrypted.Bytes()) != 0 {
			t.Fatalf("compress=%v got %d bytes for empty input", compress, len(decrypted.Bytes()))
		}
	}
}

func TestStreamWrongKeyDeepFails(t *testing.T) {
	key := testKey(t)
	wrong, _ := DeriveKeyScrypt("wrong-password!!!!!!!", bytes.Repeat([]byte{0x33}, SaltSize), 16, 8, 1)
	var enc bytes.Buffer
	_ = EncryptStreamV5(key, bytes.NewReader([]byte("secret")), &enc, true)
	var dec bytes.Buffer
	if err := DecryptStreamV5(wrong, bytes.NewReader(enc.Bytes()), &dec); err == nil {
		t.Fatal("expected failure")
	}
}

func TestStreamBitflipFails(t *testing.T) {
	key := testKey(t)
	var enc bytes.Buffer
	_ = EncryptStreamV5(key, bytes.NewReader([]byte("integrity-payload")), &enc, true)
	blob := enc.Bytes()
	// Flip a byte in the body (after header)
	if len(blob) < 40 {
		t.Fatal("blob too small")
	}
	tampered := append([]byte(nil), blob...)
	tampered[len(tampered)/2] ^= 0x01
	var dec bytes.Buffer
	if err := DecryptStreamV5(key, bytes.NewReader(tampered), &dec); err == nil {
		t.Fatal("expected bitflip failure")
	}
}

func TestStreamTruncationsFail(t *testing.T) {
	key := testKey(t)
	var enc bytes.Buffer
	_ = EncryptStreamV5(key, bytes.NewReader(bytes.Repeat([]byte("T"), 8192)), &enc, true)
	blob := enc.Bytes()
	cuts := []int{0, 1, 8, 9, 20, 30, len(blob) / 2, len(blob) - 1}
	for _, cut := range cuts {
		if cut <= 0 || cut >= len(blob) {
			if cut == 0 {
				// empty is OK (no magic)
				continue
			}
			if cut >= len(blob) {
				continue
			}
		}
		var dec bytes.Buffer
		err := DecryptStreamV5(key, bytes.NewReader(blob[:cut]), &dec)
		// Partial headers / chunks must not silently succeed with full plaintext
		if err == nil && cut < len(blob) && len(dec.Bytes()) > 0 && bytes.Equal(dec.Bytes(), bytes.Repeat([]byte("T"), 8192)) {
			t.Fatalf("truncation cut=%d unexpectedly full decrypt", cut)
		}
	}
}

func TestStreamSingleByteCorruptionMatrix(t *testing.T) {
	key := testKey(t)
	var enc bytes.Buffer
	plain := []byte("fuzz baseline payload for corruption")
	_ = EncryptStreamV5(key, bytes.NewReader(plain), &enc, true)
	blob := enc.Bytes()
	// Sample several offsets (full matrix would be slow)
	offsets := []int{0, 8, 9, 15, 20, 25, len(blob) - 1, len(blob) / 3}
	for _, off := range offsets {
		if off < 0 || off >= len(blob) {
			continue
		}
		tampered := append([]byte(nil), blob...)
		tampered[off] ^= 0x5a
		var dec bytes.Buffer
		err := DecryptStreamV5(key, bytes.NewReader(tampered), &dec)
		if err == nil && bytes.Equal(dec.Bytes(), plain) {
			t.Fatalf("offset %d corruption accepted", off)
		}
	}
}

func TestChunkNonceDiffersByIndex(t *testing.T) {
	base := bytes.Repeat([]byte{0xaa}, 16)
	n0 := chunkNonce(base, 0)
	n1 := chunkNonce(base, 1)
	if bytes.Equal(n0, n1) {
		t.Fatal("chunk nonces must differ by index")
	}
	if len(n0) != NonceSize {
		t.Fatal(len(n0))
	}
}

func TestStreamAADIncludesIndex(t *testing.T) {
	flag := byte(1)
	cn := bytes.Repeat([]byte{1}, 16)
	an := bytes.Repeat([]byte{2}, 12)
	a0 := streamAAD(flag, cn, an, 0)
	a1 := streamAAD(flag, cn, an, 1)
	if bytes.Equal(a0, a1) {
		t.Fatal("aad must bind chunk index")
	}
	// last 4 bytes are BE index
	if binary.BigEndian.Uint32(a0[len(a0)-4:]) != 0 {
		t.Fatal("index 0")
	}
	if binary.BigEndian.Uint32(a1[len(a1)-4:]) != 1 {
		t.Fatal("index 1")
	}
}

func TestEncryptDataV3RoundTripAndTamper(t *testing.T) {
	key := testKey(t)
	plain := []byte(`{"version":5,"files":{}}`)
	cN, aN, ct, err := EncryptDataV3(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	out, err := DecryptDataV3(key, cN, aN, ct)
	if err != nil || !bytes.Equal(out, plain) {
		t.Fatal(err, out)
	}
	ct[0] ^= 1
	if _, err := DecryptDataV3(key, cN, aN, ct); err == nil {
		t.Fatal("tamper should fail")
	}
}

func TestDeriveKeyEmptyPasswordFails(t *testing.T) {
	_, err := DeriveKeyScrypt("", bytes.Repeat([]byte{1}, SaltSize), 16, 8, 1)
	if err == nil {
		t.Fatal("empty password")
	}
}

func TestProfilesDocumented(t *testing.T) {
	if _, ok := Profiles["fast"]; !ok {
		t.Fatal("fast")
	}
	if Profiles["hardened"].N <= Profiles["standard"].N {
		t.Fatal("hardened should cost more than standard")
	}
	if Profiles["standard"].N <= Profiles["fast"].N {
		t.Fatal("standard > fast")
	}
}
