# Pulse-Vault Crypto

`github.com/Z3r0s/Pulse-Vault/gui-go/crypto` is the reusable Go package for
Pulse-Vault's authenticated encryption formats.

It provides:

- Scrypt key derivation for V3–V5 vaults (passwords — not SHA-256).
- PBKDF2-SHA256 and AES-GCM compatibility for V1/V2 vaults.
- SHA-256 file-content fingerprints (`FileDigestHex`) for extract/verify only.
- ChaCha20-Poly1305 + AES-GCM cascade encryption for metadata.
- V5 stream encrypt/decrypt (zstd now, old XZ still reads)
- V4 stream decrypt (old vaults)
- Constants, limits, profiles, errors

Normal ciphers. Not a homemade algorithm.

## Install

```bash
go get github.com/Z3r0s/Pulse-Vault/gui-go/crypto
```

## Example

```go
package main

import (
	"bytes"
	"log"

	pulsecrypto "github.com/Z3r0s/Pulse-Vault/gui-go/crypto"
)

func main() {
	salt := bytes.Repeat([]byte{0x42}, pulsecrypto.SaltSize)
	profile := pulsecrypto.Profiles["standard"]
	key, err := pulsecrypto.DeriveKeyScrypt("correct horse battery staple", salt, profile.N, profile.R, profile.P)
	if err != nil {
		log.Fatal(err)
	}

	var encrypted bytes.Buffer
	if err := pulsecrypto.EncryptStreamV5(key, bytes.NewReader([]byte("hello")), &encrypted, true); err != nil {
		log.Fatal(err)
	}

	var plaintext bytes.Buffer
	if err := pulsecrypto.DecryptStreamV5(key, &encrypted, &plaintext); err != nil {
		log.Fatal(err)
	}
}
```

## Compatibility

The package is tested against the Python Pulse-Vault implementation and the
repository's V1–V5 fixtures. V1/V2 are read-only compatibility formats and
should be migrated to V5 before modification.

The stream and metadata layouts are protocol formats. Changes require new
format identifiers and cross-language vectors; do not silently change the
existing wire format.

## Operational safety

`DecryptStreamV5` and `DecryptStreamV4` write plaintext as authenticated chunks
become available. If a later chunk is truncated or fails authentication, the
destination may already contain earlier plaintext. Applications that require
all-or-nothing extraction should decrypt into a private temporary file and
rename it only after the function returns successfully.

The exported `Profiles` map is a convenience for selecting the built-in KDF
parameters. Treat it as read-only and validate any user-selected parameters
with `DeriveKeyScrypt`; do not lower the limits or reuse a salt across vaults.
