// Package pulsecrypto is the public Pulse-Vault format/crypto API.
// Same stuff the app uses. Regular ciphers, not a new one.
package pulsecrypto

import (
	"io"

	inner "github.com/Z3r0s/Pulse-Vault/gui-go/internal/crypto"
)

const (
	SaltSize               = inner.SaltSize
	NonceSize              = inner.NonceSize
	KeySize                = inner.KeySize
	V3KeySize              = inner.V3KeySize
	ChunkSize              = inner.ChunkSize
	MaxEncChunk            = inner.MaxEncChunk
	MaxScryptN             = inner.MaxScryptN
	MaxScryptR             = inner.MaxScryptR
	MaxScryptP             = inner.MaxScryptP
	LegacyKeySize          = inner.LegacyKeySize
	LegacyPBKDF2Iterations = inner.LegacyPBKDF2Iterations

	// StreamV5Magic identifies the authenticated V5 stream framing.
	StreamV5Magic = "PV5STRM1"
)

var ErrCrypto = inner.ErrCrypto

// Profile describes one supported Scrypt cost profile.
type Profile = inner.Profile

// Profiles contains the built-in Scrypt profiles. Callers should treat this
// map as read-only; changing it does not change the validation limits.
var Profiles = map[string]Profile{
	"fast":     inner.Profiles["fast"],
	"standard": inner.Profiles["standard"],
	"hardened": inner.Profiles["hardened"],
}

func DeriveKeyScrypt(password string, salt []byte, n, r, p int) ([]byte, error) {
	return inner.DeriveKeyScrypt(password, salt, n, r, p)
}

func DeriveKeyLegacy(password string, salt []byte) ([]byte, error) {
	return inner.DeriveKeyLegacy(password, salt)
}

func DecryptDataLegacy(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	return inner.DecryptDataLegacy(key, nonce, ciphertext, aad)
}

func SplitV3Key(key64 []byte) (chachaKey, aesKey []byte, err error) {
	return inner.SplitV3Key(key64)
}

func EncryptDataV3(key64, plaintext []byte) (chachaNonce, aesNonce, outer []byte, err error) {
	return inner.EncryptDataV3(key64, plaintext)
}

// EncryptDataV3WithNonces is intended for deterministic protocol vectors and
// tests. Production callers should use EncryptDataV3 so nonces are random.
func EncryptDataV3WithNonces(key64, plaintext, chachaNonce, aesNonce []byte) ([]byte, error) {
	return inner.EncryptDataV3WithNonces(key64, plaintext, chachaNonce, aesNonce)
}

func DecryptDataV3(key64, chachaNonce, aesNonce, ciphertext []byte) ([]byte, error) {
	return inner.DecryptDataV3(key64, chachaNonce, aesNonce, ciphertext)
}

func EncryptStreamV5(key64 []byte, src io.Reader, dst io.Writer, compress bool) error {
	return inner.EncryptStreamV5(key64, src, dst, compress)
}

func DecryptStreamV5(key64 []byte, src io.Reader, dst io.Writer) error {
	return inner.DecryptStreamV5(key64, src, dst)
}

func DecryptStreamV4(key64 []byte, src io.Reader, dst io.Writer) error {
	return inner.DecryptStreamV4(key64, src, dst)
}

func SHA256Hex(data []byte) string {
	return inner.SHA256Hex(data)
}

func SHA256Reader(r io.Reader) (string, error) {
	return inner.SHA256Reader(r)
}
