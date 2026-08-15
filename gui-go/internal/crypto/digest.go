package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
)

// File-content integrity fingerprints (collision-resistant checksums).
//
// These are not password hashes. Vault passwords are stretched with
// scrypt (DeriveKeyScrypt) or, for V1/V2 unlock only, PBKDF2
// (DeriveKeyLegacy). SHA-256 here is the same digest stored in vault
// metadata so extract/verify can detect bit flips.

// NewFileDigest returns a SHA-256 hasher for stored file bytes.
func NewFileDigest() hash.Hash {
	// codeql[go/weak-sensitive-data-hashing]
	return sha256.New()
}

// FileDigestHex returns lowercase hex SHA-256 of file contents.
func FileDigestHex(fileBytes []byte) string {
	h := NewFileDigest()
	_, _ = h.Write(fileBytes)
	return hex.EncodeToString(h.Sum(nil))
}

// FileDigestReader streams file contents and returns lowercase hex SHA-256.
func FileDigestReader(r io.Reader) (string, error) {
	h := NewFileDigest()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SHA256Hex is FileDigestHex. Kept so existing callers keep compiling.
func SHA256Hex(data []byte) string {
	return FileDigestHex(data)
}

// SHA256Reader is FileDigestReader. Kept so existing callers keep compiling.
func SHA256Reader(r io.Reader) (string, error) {
	return FileDigestReader(r)
}
