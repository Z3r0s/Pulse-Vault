// Package crypto is the V5 engine. Apps should import gui-go/crypto, not this.
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
	compressNone           = byte(0)
	compressXZ             = byte(1)
	compressZstd           = byte(2)
	MaxScryptN             = 1 << 20
	MaxScryptR             = 32
	MaxScryptP             = 16
	LegacyKeySize          = 32
	LegacyPBKDF2Iterations = 600000
)

var (
	StreamV5Magic = []byte("PV5STRM1")
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
	if n <= 1 || n > MaxScryptN || n&(n-1) != 0 || r <= 0 || r > MaxScryptR || p <= 0 || p > MaxScryptP {
		return nil, fmt.Errorf("%w: invalid scrypt parameters", ErrCrypto)
	}
	key, err := scrypt.Key([]byte(password), salt, n, r, p, V3KeySize)
	if err != nil {
		return nil, fmt.Errorf("%w: scrypt: %v", ErrCrypto, err)
	}
	return key, nil
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

func streamAAD(flag byte, chachaNonce, aesNonce []byte, idx uint32) []byte {
	aad := make([]byte, 0, len(StreamV5Magic)+1+len(chachaNonce)+len(aesNonce)+4)
	aad = append(aad, StreamV5Magic...)
	aad = append(aad, flag)
	aad = append(aad, chachaNonce...)
	aad = append(aad, aesNonce...)
	var idxBytes [4]byte
	binary.BigEndian.PutUint32(idxBytes[:], idx)
	aad = append(aad, idxBytes[:]...)
	return aad
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

func xzCompress(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := newFastXZWriter(&buf)
	if err != nil {
		return nil, err
	}
	if _, err = w.Write(src); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err = w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func xzDecompress(src []byte) ([]byte, error) {
	r, err := xz.NewReader(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
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
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
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
	aad := streamAAD(w.flag, w.chachaNonce, w.aesNonce, w.idx)
	w.inner = w.aeadC.Seal(w.inner[:0], chunkNonce(w.chachaNonce, w.idx), w.buf, aad)
	w.outer = w.aeadA.Seal(w.outer[:0], chunkNonce(w.aesNonce, w.idx), w.inner, aad)
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(w.outer)))
	if err := writeFull(w.dst, lenBuf[:]); err != nil {
		return err
	}
	if err := writeFull(w.dst, w.outer); err != nil {
		return err
	}
	w.buf = w.buf[:0]
	w.idx++
	w.wroteChunk = true
	return nil
}

type streamChunkReader struct {
	src         io.Reader
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
}

func (r *streamChunkReader) Read(p []byte) (int, error) {
	for len(r.buf) == 0 && !r.done {
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
			aad = streamAAD(r.flag, r.chachaNonce, r.aesNonce, r.idx)
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
