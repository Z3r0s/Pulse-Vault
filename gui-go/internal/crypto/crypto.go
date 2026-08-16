// Package crypto is the internal V5/V6 engine. Apps should import gui-go/crypto, not this.
package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/scrypt"
)

const (
	SaltSize    = 16
	NonceSize   = 12
	KeySize     = 32
	V3KeySize   = 64
	ChunkSize   = 1024 * 1024
	MaxEncChunk = 64 * 1024 * 1024
	// 1 MiB dict = xz preset 1. 8 MiB was stupid slow.
	xzFastDictCap = 1 << 20
	streamCopyBuf = 128 * 1024
	sniffBytes    = 4096
	// jpgs/video usually hit this; source code doesn't.
	uniqueByteSkip = 240

	// 0 none, 1 old xz, 2 zstd (what we write now)
	compressNone = byte(0)
	compressXZ   = byte(1)
	compressZstd = byte(2)
	MaxScryptN   = 1 << 20
	MaxScryptR   = 32
	MaxScryptP   = 16
	// MaxScryptMemoryBytes and MaxScryptWorkFactor bound attacker-controlled
	// KDF records before scrypt allocates memory or burns CPU. The hardened
	// profile remains valid at exactly 1 GiB / 8,388,608 work units.
	MaxScryptMemoryBytes   = uint64(1 << 30)
	MaxScryptWorkFactor    = uint64(1 << 24)
	LegacyKeySize          = 32
	LegacyPBKDF2Iterations = 600000
)

var (
	StreamV5Magic = []byte("PV5STRM1")
	StreamV6Magic = []byte("PV6STRM1")
	ErrCrypto     = errors.New("crypto error")
)

// Profile describes one supported Scrypt cost profile.
type Profile struct {
	N int
	R int
	P int
}

// Profiles maps scrypt profile names to n, r, p.
var Profiles = map[string]Profile{
	"fast":     {N: 16, R: 8, P: 1},
	"standard": {N: 1 << 15, R: 8, P: 1},
	"hardened": {N: 1 << 20, R: 8, P: 1},
}

func DeriveKeyScrypt(password string, salt []byte, n, r, p int) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("%w: password cannot be empty", ErrCrypto)
	}
	if len(salt) != SaltSize {
		return nil, fmt.Errorf("%w: invalid salt size", ErrCrypto)
	}
	if err := ValidateScryptParams(n, r, p); err != nil {
		return nil, err
	}
	key, err := scrypt.Key([]byte(password), salt, n, r, p, V3KeySize)
	if err != nil {
		return nil, fmt.Errorf("%w: scrypt: %v", ErrCrypto, err)
	}
	return key, nil
}

// ValidateScryptParams rejects malformed or resource-exhausting persisted KDF
// records before calling the memory-hard implementation.
func ValidateScryptParams(n, r, p int) error {
	if n <= 1 || n > MaxScryptN || n&(n-1) != 0 || r <= 0 || r > MaxScryptR || p <= 0 || p > MaxScryptP {
		return fmt.Errorf("%w: invalid scrypt parameters", ErrCrypto)
	}
	memory := uint64(128) * uint64(n) * uint64(r) * uint64(p)
	if memory > MaxScryptMemoryBytes {
		return fmt.Errorf("%w: scrypt memory budget exceeded", ErrCrypto)
	}
	work := uint64(n) * uint64(r) * uint64(p)
	if work > MaxScryptWorkFactor {
		return fmt.Errorf("%w: scrypt work budget exceeded", ErrCrypto)
	}
	return nil
}

// DeriveKeyLegacy matches the PBKDF2-SHA256 construction used by V1/V2.
func DeriveKeyLegacy(password string, salt []byte) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("%w: password cannot be empty", ErrCrypto)
	}
	if len(salt) != SaltSize {
		return nil, fmt.Errorf("%w: invalid salt size", ErrCrypto)
	}
	return pbkdf2.Key([]byte(password), salt, LegacyPBKDF2Iterations, LegacyKeySize, sha256.New), nil
}

// DecryptDataLegacy reverses the single-layer AES-GCM V1/V2 construction.
func DecryptDataLegacy(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	if len(key) != LegacyKeySize || len(nonce) != NonceSize {
		return nil, fmt.Errorf("%w: invalid legacy crypto sizes", ErrCrypto)
	}
	aead, err := aesGCM(key)
	if err != nil {
		return nil, err
	}
	plain, err := aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, fmt.Errorf("%w: legacy decrypt failed", ErrCrypto)
	}
	return plain, nil
}

func SplitV3Key(key64 []byte) (chachaKey, aesKey []byte, err error) {
	if len(key64) != V3KeySize {
		return nil, nil, fmt.Errorf("%w: V3 key must be 64 bytes", ErrCrypto)
	}
	return append([]byte(nil), key64[:32]...), append([]byte(nil), key64[32:]...), nil
}

func chunkNonce(base []byte, idx uint32) []byte {
	nonce := make([]byte, NonceSize)
	copy(nonce, base[:NonceSize])
	var idxBytes [4]byte
	binary.BigEndian.PutUint32(idxBytes[:], idx)
	for i := 0; i < 4; i++ {
		nonce[i] ^= idxBytes[i]
	}
	return nonce
}

func streamAADFor(magic []byte, flag byte, chachaNonce, aesNonce []byte, idx uint32, recordKind byte) []byte {
	aad := make([]byte, 0, len(magic)+1+len(chachaNonce)+len(aesNonce)+5)
	aad = append(aad, magic...)
	aad = append(aad, flag)
	aad = append(aad, chachaNonce...)
	aad = append(aad, aesNonce...)
	if magic == nil || bytes.Equal(magic, StreamV5Magic) {
		// V5 authenticates data chunks with the historical AAD layout.
	} else {
		aad = append(aad, recordKind)
	}
	var idxBytes [4]byte
	binary.BigEndian.PutUint32(idxBytes[:], idx)
	aad = append(aad, idxBytes[:]...)
	return aad
}

func streamAAD(flag byte, chachaNonce, aesNonce []byte, idx uint32) []byte {
	return streamAADFor(StreamV5Magic, flag, chachaNonce, aesNonce, idx, 0)
}

func aesGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// EncryptDataV3 cascade-encrypts an in-memory blob (metadata).
func EncryptDataV3(key64, plaintext []byte) (chachaNonce, aesNonce, outer []byte, err error) {
	chachaKey, aesKey, err := SplitV3Key(key64)
	if err != nil {
		return nil, nil, nil, err
	}
	chachaNonce = make([]byte, NonceSize)
	aesNonce = make([]byte, NonceSize)
	if _, err = rand.Read(chachaNonce); err != nil {
		return nil, nil, nil, err
	}
	if _, err = rand.Read(aesNonce); err != nil {
		return nil, nil, nil, err
	}

	aeadC, err := chacha20poly1305.New(chachaKey)
	if err != nil {
		return nil, nil, nil, err
	}
	aeadA, err := aesGCM(aesKey)
	if err != nil {
		return nil, nil, nil, err
	}

	inner := aeadC.Seal(nil, chachaNonce, plaintext, nil)
	outer = aeadA.Seal(nil, aesNonce, inner, nil)
	return chachaNonce, aesNonce, outer, nil
}

// EncryptDataV3WithNonces is EncryptDataV3 with fixed nonces (tests / vectors).
func EncryptDataV3WithNonces(key64, plaintext, chachaNonce, aesNonce []byte) ([]byte, error) {
	chachaKey, aesKey, err := SplitV3Key(key64)
	if err != nil {
		return nil, err
	}
	if len(chachaNonce) != NonceSize || len(aesNonce) != NonceSize {
		return nil, fmt.Errorf("%w: invalid nonce size", ErrCrypto)
	}
	aeadC, err := chacha20poly1305.New(chachaKey)
	if err != nil {
		return nil, err
	}
	aeadA, err := aesGCM(aesKey)
	if err != nil {
		return nil, err
	}
	inner := aeadC.Seal(nil, chachaNonce, plaintext, nil)
	return aeadA.Seal(nil, aesNonce, inner, nil), nil
}

// DecryptDataV3 reverses EncryptDataV3.
func DecryptDataV3(key64, chachaNonce, aesNonce, ciphertext []byte) ([]byte, error) {
	chachaKey, aesKey, err := SplitV3Key(key64)
	if err != nil {
		return nil, err
	}
	if len(chachaNonce) != NonceSize || len(aesNonce) != NonceSize {
		return nil, fmt.Errorf("%w: invalid nonce size", ErrCrypto)
	}
	aeadA, err := aesGCM(aesKey)
	if err != nil {
		return nil, err
	}
	aeadC, err := chacha20poly1305.New(chachaKey)
	if err != nil {
		return nil, err
	}
	inner, err := aeadA.Open(nil, aesNonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: cascade decrypt failed", ErrCrypto)
	}
	plain, err := aeadC.Open(nil, chachaNonce, inner, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: cascade decrypt failed", ErrCrypto)
	}
	return plain, nil
}

func newFastXZWriter(dst io.Writer) (io.WriteCloser, error) {
	return xz.WriterConfig{DictCap: xzFastDictCap}.NewWriter(dst)
}

func newZstdWriter(dst io.Writer) (io.WriteCloser, error) {
	return zstd.NewWriter(dst,
		zstd.WithEncoderLevel(zstd.SpeedDefault),
		zstd.WithWindowSize(1<<20),
	)
}

func looksIncompressible(sample []byte) bool {
	if len(sample) < 512 {
		return false
	}
	n := len(sample)
	if n > sniffBytes {
		n = sniffBytes
	}
	var seen [256]byte
	uniq := 0
	for _, b := range sample[:n] {
		if seen[b] == 0 {
			seen[b] = 1
			uniq++
			if uniq >= uniqueByteSkip {
				return true
			}
		}
	}
	return false
}

// EncryptStreamV5 writes a V5 encrypted stream to dst.
// Format: magic(8) | flag(1) | chacha_nonce(16) | aes_nonce(12) | repeated: len(4 BE) | chunk
func EncryptStreamV5(key64 []byte, src io.Reader, dst io.Writer, compress bool) error {
	chachaNonce := make([]byte, 16)
	aesNonce := make([]byte, NonceSize)
	if _, err := rand.Read(chachaNonce); err != nil {
		return err
	}
	if _, err := rand.Read(aesNonce); err != nil {
		return err
	}
	return encryptStreamV5WithNonces(key64, src, dst, compress, false, chachaNonce, aesNonce)
}

// old xz writer (flag 1). tests / compat only. new writes are zstd.
func encryptStreamV5XZ(key64 []byte, src io.Reader, dst io.Writer) error {
	chachaNonce := make([]byte, 16)
	aesNonce := make([]byte, NonceSize)
	if _, err := rand.Read(chachaNonce); err != nil {
		return err
	}
	if _, err := rand.Read(aesNonce); err != nil {
		return err
	}
	return encryptStreamV5WithNonces(key64, src, dst, true, true, chachaNonce, aesNonce)
}

func encryptStreamV5WithNonces(key64 []byte, src io.Reader, dst io.Writer, compress, legacyXZ bool, chachaNonce, aesNonce []byte) error {
	chachaKey, aesKey, err := SplitV3Key(key64)
	if err != nil {
		return err
	}
	if len(chachaNonce) != 16 || len(aesNonce) != NonceSize {
		return fmt.Errorf("%w: invalid stream nonce size", ErrCrypto)
	}
	if compress {
		prefix := make([]byte, sniffBytes)
		n, readErr := io.ReadFull(src, prefix)
		if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
			return readErr
		}
		prefix = prefix[:n]
		if n == 0 && errors.Is(readErr, io.EOF) {
			// The zstd writer emits no frame for an empty input; use the
			// authenticated uncompressed representation instead.
			compress = false
		}
		if looksIncompressible(prefix) {
			compress = false
		}
		src = io.MultiReader(bytes.NewReader(prefix), src)
	}
	flag := compressNone
	if compress {
		if legacyXZ {
			flag = compressXZ
		} else {
			flag = compressZstd
		}
	}

	if err = writeFull(dst, StreamV5Magic); err != nil {
		return err
	}
	if err = writeFull(dst, []byte{flag}); err != nil {
		return err
	}
	if err = writeFull(dst, chachaNonce); err != nil {
		return err
	}
	if err = writeFull(dst, aesNonce); err != nil {
		return err
	}

	aeadC, err := chacha20poly1305.New(chachaKey)
	if err != nil {
		return err
	}
	aeadA, err := aesGCM(aesKey)
	if err != nil {
		return err
	}

	innerCap := ChunkSize + aeadC.Overhead()
	chunker := &streamChunkWriter{
		dst:         dst,
		magic:       StreamV5Magic,
		flag:        flag,
		chachaNonce: chachaNonce,
		aesNonce:    aesNonce,
		aeadC:       aeadC,
		aeadA:       aeadA,
		buf:         make([]byte, 0, ChunkSize),
		inner:       make([]byte, 0, innerCap),
		outer:       make([]byte, 0, innerCap+aeadA.Overhead()),
	}
	copyBuf := make([]byte, streamCopyBuf)
	if compress {
		var compressor io.WriteCloser
		var err error
		if flag == compressXZ {
			compressor, err = newFastXZWriter(chunker)
			if err != nil {
				return fmt.Errorf("%w: xz compress: %v", ErrCrypto, err)
			}
		} else {
			compressor, err = newZstdWriter(chunker)
			if err != nil {
				return fmt.Errorf("%w: zstd compress: %v", ErrCrypto, err)
			}
		}
		if _, err = io.CopyBuffer(compressor, src, copyBuf); err != nil {
			_ = compressor.Close()
			return err
		}
		if err = compressor.Close(); err != nil {
			return fmt.Errorf("%w: compress: %v", ErrCrypto, err)
		}
	} else {
		if _, err = io.CopyBuffer(chunker, src, copyBuf); err != nil {
			return err
		}
	}
	if err = chunker.Close(); err != nil {
		return err
	}
	return nil
}

// DecryptStreamV5 decrypts a V5 stream from src to dst.
func DecryptStreamV5(key64 []byte, src io.Reader, dst io.Writer) error {
	chachaKey, aesKey, err := SplitV3Key(key64)
	if err != nil {
		return err
	}
	magic := make([]byte, len(StreamV5Magic))
	if _, err = io.ReadFull(src, magic); err != nil {
		return fmt.Errorf("%w: truncated stream header", ErrCrypto)
	}
	if !bytes.Equal(magic, StreamV5Magic) {
		return fmt.Errorf("%w: unknown stream header", ErrCrypto)
	}

	flagBuf := make([]byte, 1)
	if _, err = io.ReadFull(src, flagBuf); err != nil {
		return fmt.Errorf("%w: truncated stream", ErrCrypto)
	}
	flag := flagBuf[0]
	if flag != compressNone && flag != compressXZ && flag != compressZstd {
		return fmt.Errorf("%w: invalid compression flag", ErrCrypto)
	}

	chachaNonce := make([]byte, 16)
	aesNonce := make([]byte, NonceSize)
	if _, err = io.ReadFull(src, chachaNonce); err != nil {
		return fmt.Errorf("%w: truncated stream", ErrCrypto)
	}
	if _, err = io.ReadFull(src, aesNonce); err != nil {
		return fmt.Errorf("%w: truncated stream", ErrCrypto)
	}

	aeadC, err := chacha20poly1305.New(chachaKey)
	if err != nil {
		return err
	}
	aeadA, err := aesGCM(aesKey)
	if err != nil {
		return err
	}

	chunkReader := &streamChunkReader{
		src:         src,
		magic:       StreamV5Magic,
		flag:        flag,
		chachaNonce: chachaNonce,
		aesNonce:    aesNonce,
		aeadC:       aeadC,
		aeadA:       aeadA,
		bindAAD:     true,
	}
	copyBuf := make([]byte, streamCopyBuf)
	switch flag {
	case compressXZ:
		decompressor, err := xz.NewReader(chunkReader)
		if err != nil {
			return fmt.Errorf("%w: xz decompress: %v", ErrCrypto, err)
		}
		if _, err = io.CopyBuffer(dst, decompressor, copyBuf); err != nil {
			return fmt.Errorf("%w: xz decompress: %v", ErrCrypto, err)
		}
		return nil
	case compressZstd:
		decompressor, err := zstd.NewReader(chunkReader)
		if err != nil {
			return fmt.Errorf("%w: zstd decompress: %v", ErrCrypto, err)
		}
		defer decompressor.Close()
		if _, err = io.CopyBuffer(dst, decompressor, copyBuf); err != nil {
			return fmt.Errorf("%w: zstd decompress: %v", ErrCrypto, err)
		}
		return nil
	default:
		_, err = io.CopyBuffer(dst, chunkReader, copyBuf)
		return err
	}
}

const streamV6EndMarker = "PULSEVAULT6-END"

// EncryptStreamV6 writes a finalized, authenticated stream. V5 remains
// readable for existing vaults; V6 adds a framed terminal record so a clean
// chunk-boundary truncation cannot be mistaken for EOF.
// Format: magic(8) | flag(1) | chacha_nonce(16) | aes_nonce(12) |
// repeated: kind(1) | len(4 BE) | ciphertext, where kind 0 is data and kind
// 1 is the authenticated terminal record.
func EncryptStreamV6(key64 []byte, src io.Reader, dst io.Writer, compress bool) error {
	chachaNonce := make([]byte, 16)
	aesNonce := make([]byte, NonceSize)
	if _, err := rand.Read(chachaNonce); err != nil {
		return err
	}
	if _, err := rand.Read(aesNonce); err != nil {
		return err
	}
	return encryptStreamV6WithNonces(key64, src, dst, compress, chachaNonce, aesNonce)
}

func encryptStreamV6WithNonces(key64 []byte, src io.Reader, dst io.Writer, compress bool, chachaNonce, aesNonce []byte) error {
	chachaKey, aesKey, err := SplitV3Key(key64)
	if err != nil {
		return err
	}
	if len(chachaNonce) != 16 || len(aesNonce) != NonceSize {
		return fmt.Errorf("%w: invalid stream nonce size", ErrCrypto)
	}
	if compress {
		prefix := make([]byte, sniffBytes)
		n, readErr := io.ReadFull(src, prefix)
		if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
			return readErr
		}
		prefix = prefix[:n]
		if n == 0 && errors.Is(readErr, io.EOF) {
			// The zstd writer intentionally emits no frame for an empty input;
			// encode empty streams as uncompressed rather than writing a V6
			// stream that advertises compression without a compressed payload.
			compress = false
		}
		if looksIncompressible(prefix) {
			compress = false
		}
		src = io.MultiReader(bytes.NewReader(prefix), src)
	}
	flag := compressNone
	if compress {
		flag = compressZstd
	}
	if err = writeFull(dst, StreamV6Magic); err != nil {
		return err
	}
	for _, part := range [][]byte{{flag}, chachaNonce, aesNonce} {
		if err = writeFull(dst, part); err != nil {
			return err
		}
	}
	aeadC, err := chacha20poly1305.New(chachaKey)
	if err != nil {
		return err
	}
	aeadA, err := aesGCM(aesKey)
	if err != nil {
		return err
	}
	chunker := &streamChunkWriter{
		dst: dst, magic: StreamV6Magic, framed: true,
		flag: flag, chachaNonce: chachaNonce, aesNonce: aesNonce,
		aeadC: aeadC, aeadA: aeadA,
		buf:   make([]byte, 0, ChunkSize),
		inner: make([]byte, 0, ChunkSize+aeadC.Overhead()),
		outer: make([]byte, 0, ChunkSize+aeadC.Overhead()+aeadA.Overhead()),
	}
	copyBuf := make([]byte, streamCopyBuf)
	if compress {
		compressor, err := newZstdWriter(chunker)
		if err != nil {
			return fmt.Errorf("%w: zstd compress: %v", ErrCrypto, err)
		}
		if _, err = io.CopyBuffer(compressor, src, copyBuf); err != nil {
			_ = compressor.Close()
			return err
		}
		if err = compressor.Close(); err != nil {
			return fmt.Errorf("%w: compress: %v", ErrCrypto, err)
		}
	} else if _, err = io.CopyBuffer(chunker, src, copyBuf); err != nil {
		return err
	}
	return chunker.Close()
}

// DecryptStreamV6 requires and authenticates the terminal record, including
// after decompression has reached its logical EOF.
func DecryptStreamV6(key64 []byte, src io.Reader, dst io.Writer) error {
	chachaKey, aesKey, err := SplitV3Key(key64)
	if err != nil {
		return err
	}
	magic := make([]byte, len(StreamV6Magic))
	if _, err = io.ReadFull(src, magic); err != nil || !bytes.Equal(magic, StreamV6Magic) {
		return fmt.Errorf("%w: invalid or truncated V6 stream header", ErrCrypto)
	}
	flagBuf := make([]byte, 1)
	if _, err = io.ReadFull(src, flagBuf); err != nil {
		return fmt.Errorf("%w: truncated V6 stream", ErrCrypto)
	}
	flag := flagBuf[0]
	if flag != compressNone && flag != compressZstd {
		return fmt.Errorf("%w: invalid V6 compression flag", ErrCrypto)
	}
	chachaNonce := make([]byte, 16)
	aesNonce := make([]byte, NonceSize)
	if _, err = io.ReadFull(src, chachaNonce); err != nil {
		return fmt.Errorf("%w: truncated V6 stream", ErrCrypto)
	}
	if _, err = io.ReadFull(src, aesNonce); err != nil {
		return fmt.Errorf("%w: truncated V6 stream", ErrCrypto)
	}
	aeadC, err := chacha20poly1305.New(chachaKey)
	if err != nil {
		return err
	}
	aeadA, err := aesGCM(aesKey)
	if err != nil {
		return err
	}
	r := &streamChunkReader{
		src: src, magic: StreamV6Magic, framed: true,
		flag: flag, chachaNonce: chachaNonce, aesNonce: aesNonce,
		aeadC: aeadC, aeadA: aeadA, bindAAD: true,
	}
	copyBuf := make([]byte, streamCopyBuf)
	switch flag {
	case compressZstd:
		decompressor, err := zstd.NewReader(r)
		if err != nil {
			return fmt.Errorf("%w: zstd decompress: %v", ErrCrypto, err)
		}
		_, copyErr := io.CopyBuffer(dst, decompressor, copyBuf)
		decompressor.Close()
		if copyErr != nil {
			return fmt.Errorf("%w: zstd decompress: %v", ErrCrypto, copyErr)
		}
	default:
		if _, err = io.CopyBuffer(dst, r, copyBuf); err != nil {
			return err
		}
	}
	if err := r.finishV6(); err != nil {
		return err
	}
	return nil
}

// DecryptStreamV4 reads the legacy V4 stream format. V4 uses the same
// cascade/chunk construction as V5 but has no magic, compression flag, or AAD.
func DecryptStreamV4(key64 []byte, src io.Reader, dst io.Writer) error {
	chachaKey, aesKey, err := SplitV3Key(key64)
	if err != nil {
		return err
	}
	chachaNonce := make([]byte, 16)
	if _, err = io.ReadFull(src, chachaNonce); err != nil {
		return fmt.Errorf("%w: truncated V4 stream", ErrCrypto)
	}
	aesNonce := make([]byte, NonceSize)
	if _, err = io.ReadFull(src, aesNonce); err != nil {
		return fmt.Errorf("%w: truncated V4 stream", ErrCrypto)
	}
	aeadC, err := chacha20poly1305.New(chachaKey)
	if err != nil {
		return err
	}
	aeadA, err := aesGCM(aesKey)
	if err != nil {
		return err
	}
	r := &streamChunkReader{
		src: src, chachaNonce: chachaNonce, aesNonce: aesNonce,
		aeadC: aeadC, aeadA: aeadA,
	}
	if _, err = io.Copy(dst, r); err != nil {
		return err
	}
	return nil
}

type streamChunkWriter struct {
	dst         io.Writer
	magic       []byte
	framed      bool
	flag        byte
	chachaNonce []byte
	aesNonce    []byte
	aeadC       cipher.AEAD
	aeadA       cipher.AEAD
	buf         []byte
	inner       []byte
	outer       []byte
	idx         uint32
	wroteChunk  bool
}

func (w *streamChunkWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		need := ChunkSize - len(w.buf)
		if need > len(p) {
			need = len(p)
		}
		w.buf = append(w.buf, p[:need]...)
		p = p[need:]
		written += need
		if len(w.buf) == ChunkSize {
			if err := w.flush(); err != nil {
				return written, err
			}
		}
	}
	return written, nil
}

func (w *streamChunkWriter) Close() error {
	if w.framed {
		if len(w.buf) > 0 {
			if err := w.flush(); err != nil {
				return err
			}
		}
		return w.writeEnd()
	}
	if len(w.buf) > 0 {
		return w.flush()
	}
	// empty file still gets one auth'd chunk so the header isn't hanging there naked
	if !w.wroteChunk {
		return w.flush()
	}
	return nil
}

func (w *streamChunkWriter) flush() error {
	if w.idx == ^uint32(0) {
		return fmt.Errorf("%w: too many chunks", ErrCrypto)
	}
	aad := streamAADFor(w.magic, w.flag, w.chachaNonce, w.aesNonce, w.idx, 0)
	w.inner = w.aeadC.Seal(w.inner[:0], chunkNonce(w.chachaNonce, w.idx), w.buf, aad)
	w.outer = w.aeadA.Seal(w.outer[:0], chunkNonce(w.aesNonce, w.idx), w.inner, aad)
	if err := w.writeRecord(0, w.outer); err != nil {
		return err
	}
	w.buf = w.buf[:0]
	w.idx++
	w.wroteChunk = true
	return nil
}

func (w *streamChunkWriter) writeRecord(kind byte, ciphertext []byte) error {
	if w.framed {
		if err := writeFull(w.dst, []byte{kind}); err != nil {
			return err
		}
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(ciphertext)))
	if err := writeFull(w.dst, lenBuf[:]); err != nil {
		return err
	}
	return writeFull(w.dst, ciphertext)
}

func (w *streamChunkWriter) writeEnd() error {
	if w.idx == ^uint32(0) {
		return fmt.Errorf("%w: too many chunks", ErrCrypto)
	}
	aad := streamAADFor(w.magic, w.flag, w.chachaNonce, w.aesNonce, w.idx, 1)
	inner := w.aeadC.Seal(nil, chunkNonce(w.chachaNonce, w.idx), []byte(streamV6EndMarker), aad)
	outer := w.aeadA.Seal(nil, chunkNonce(w.aesNonce, w.idx), inner, aad)
	return w.writeRecord(1, outer)
}

type streamChunkReader struct {
	src         io.Reader
	magic       []byte
	framed      bool
	flag        byte
	chachaNonce []byte
	aesNonce    []byte
	aeadC       cipher.AEAD
	aeadA       cipher.AEAD
	bindAAD     bool
	buf         []byte
	enc         []byte
	inner       []byte
	plain       []byte
	idx         uint32
	done        bool
	ended       bool
}

func (r *streamChunkReader) Read(p []byte) (int, error) {
	for len(r.buf) == 0 && !r.done {
		var kind byte
		if r.framed {
			var kindBuf [1]byte
			n, err := io.ReadFull(r.src, kindBuf[:])
			if errors.Is(err, io.EOF) && n == 0 {
				return 0, fmt.Errorf("%w: missing V6 terminal record", ErrCrypto)
			}
			if err != nil {
				return 0, fmt.Errorf("%w: truncated V6 record kind", ErrCrypto)
			}
			kind = kindBuf[0]
			if kind != 0 && kind != 1 {
				return 0, fmt.Errorf("%w: invalid V6 record kind", ErrCrypto)
			}
		}
		var lenBuf [4]byte
		n, err := io.ReadFull(r.src, lenBuf[:])
		if errors.Is(err, io.EOF) && n == 0 {
			r.done = true
			break
		}
		if err != nil {
			return 0, fmt.Errorf("%w: truncated chunk length", ErrCrypto)
		}
		if r.idx == ^uint32(0) {
			return 0, fmt.Errorf("%w: too many chunks", ErrCrypto)
		}
		chunkLen := binary.BigEndian.Uint32(lenBuf[:])
		if chunkLen == 0 || chunkLen > MaxEncChunk {
			return 0, fmt.Errorf("%w: invalid chunk length", ErrCrypto)
		}
		if cap(r.enc) < int(chunkLen) {
			r.enc = make([]byte, chunkLen)
		} else {
			r.enc = r.enc[:chunkLen]
		}
		if _, err = io.ReadFull(r.src, r.enc); err != nil {
			return 0, fmt.Errorf("%w: truncated chunk", ErrCrypto)
		}
		aad := []byte(nil)
		if r.bindAAD {
			aad = streamAADFor(r.magic, r.flag, r.chachaNonce, r.aesNonce, r.idx, kind)
		}
		inner, err := r.aeadA.Open(r.inner[:0], chunkNonce(r.aesNonce, r.idx), r.enc, aad)
		if err != nil {
			return 0, fmt.Errorf("%w: cascade decrypt failed", ErrCrypto)
		}
		r.inner = inner
		final, err := r.aeadC.Open(r.plain[:0], chunkNonce(r.chachaNonce, r.idx), r.inner, aad)
		if err != nil {
			return 0, fmt.Errorf("%w: cascade decrypt failed", ErrCrypto)
		}
		if r.framed && kind == 1 {
			if !bytes.Equal(final, []byte(streamV6EndMarker)) {
				return 0, fmt.Errorf("%w: invalid V6 terminal record", ErrCrypto)
			}
			r.ended = true
			r.done = true
			continue
		}
		if r.framed && kind != 0 {
			return 0, fmt.Errorf("%w: invalid V6 data record", ErrCrypto)
		}
		r.plain = final
		r.buf = r.plain
		r.idx++
	}
	if len(r.buf) == 0 && r.done {
		return 0, io.EOF
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

func (r *streamChunkReader) finishV6() error {
	if !r.framed {
		return nil
	}
	for !r.ended {
		var discard [1]byte
		if _, err := r.Read(discard[:]); err != nil {
			return fmt.Errorf("%w: missing V6 terminal record", ErrCrypto)
		}
	}
	var extra [1]byte
	n, err := r.src.Read(extra[:])
	if n != 0 {
		return fmt.Errorf("%w: trailing bytes after V6 terminal record", ErrCrypto)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func writeFull(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}
