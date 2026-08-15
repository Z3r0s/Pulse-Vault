// Package vault implements Pulse-Vault V5 ZIP containers.
package vault

import (
	"archive/zip"
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	crypto "github.com/Z3r0s/Pulse-Vault/gui-go/crypto"
)

const (
	FormatV1 = "Z3R0VAULT1"
	FormatV2 = "PULSEVAULT2"
	FormatV3 = "PULSEVAULT3_CASCADE"
	FormatV4 = "PULSEVAULT4_CASCADE"
	FormatV5 = "PULSEVAULT5_COMPRESSED_CASCADE"

	MaxZipEntries   = 20_000
	MaxFormatSize   = 128
	MaxKDFJSONSize  = 512
	MaxMetadataSize = 16 * 1024 * 1024
	MaxDataBlobSize = 512 * 1024 * 1024 * 1024
)

// don't stamp the host clock on zip members (that's sitting there in the clear)
var zipNeutralTime = time.Unix(0, 0).UTC()

func storedZipHeader(name string) *zip.FileHeader {
	return &zip.FileHeader{
		Name:     name,
		Method:   zip.Store,
		Modified: zipNeutralTime,
	}
}

var ErrVault = errors.New("vault error")

// FileMeta describes one entry in vault metadata.
type FileMeta struct {
	Name       string `json:"name"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	AddedAt    int64  `json:"added_at"`
	UpdatedAt  int64  `json:"updated_at"`
	Type       string `json:"type"`
	InternalID string `json:"internal_id"`
	Content    string `json:"content,omitempty"`
}

// Metadata is the encrypted JSON document.
type Metadata struct {
	Version   int                 `json:"version"`
	CreatedAt int64               `json:"created_at"`
	UpdatedAt int64               `json:"updated_at"`
	Files     map[string]FileMeta `json:"files"`
}

// KDFRecord is stored in kdf.json.
type KDFRecord struct {
	Algorithm string `json:"algorithm"`
	Profile   string `json:"profile"`
	N         int    `json:"n"`
	R         int    `json:"r"`
	P         int    `json:"p"`
}

// Vault is an unlocked or locked Pulse-Vault V5 container.
type Vault struct {
	Path    string
	Format  string
	Profile string
	KdfN    int
	KdfR    int
	KdfP    int

	salt []byte
	key  []byte
	meta Metadata
	// legacySource is set only while rewriting a non-ZIP V1 container during
	// migration; its raw bytes must not be parsed as an existing ZIP.
	legacySource bool
	// carrierSource is a media file prepended on the next write (hide-in-picture).
	// Cleared after a successful write.
	carrierSource string
	// carrierPrefix is the number of leading non-ZIP bytes after unlock/write.
	carrierPrefix int64
}

func New(path string) *Vault {
	return &Vault{Path: path, Format: FormatV5, Profile: "standard"}
}

func (v *Vault) IsUnlocked() bool {
	return v.key != nil && v.salt != nil
}

// CreatedAt is the vault metadata timestamp (unix seconds).
func (v *Vault) CreatedAt() int64 { return v.meta.CreatedAt }

// UpdatedAt is the last metadata write (unix seconds).
func (v *Vault) UpdatedAt() int64 { return v.meta.UpdatedAt }

// FileCount is the number of sealed entries.
func (v *Vault) FileCount() int { return len(v.meta.Files) }

func (v *Vault) setKDF(profile string) error {
	p, ok := crypto.Profiles[profile]
	if !ok {
		return fmt.Errorf("%w: unknown scrypt profile %q", ErrVault, profile)
	}
	v.Profile = profile
	v.KdfN, v.KdfR, v.KdfP = p.N, p.R, p.P
	return nil
}

func nowUnix() int64 { return time.Now().Unix() }

func defaultMeta() Metadata {
	t := nowUnix()
	return Metadata{
		Version:   5,
		CreatedAt: t,
		UpdatedAt: t,
		Files:     map[string]FileMeta{},
	}
}

// Create writes a new empty V5 vault.
func (v *Vault) Create(password, profile string) error {
	return v.create(password, profile, "")
}

// CreateWithCarrier writes a new V5 vault after a carrier image/video so the
// result still opens as that picture while holding an encrypted vault.
func (v *Vault) CreateWithCarrier(password, profile, carrierPath string) error {
	carrierPath = strings.TrimSpace(carrierPath)
	if carrierPath == "" {
		return fmt.Errorf("%w: carrier path is required", ErrVault)
	}
	return v.create(password, profile, carrierPath)
}

func (v *Vault) create(password, profile, carrierPath string) error {
	if password == "" {
		return fmt.Errorf("%w: password cannot be empty", ErrVault)
	}
	if err := refuseSymlinkDest(v.Path); err != nil {
		return err
	}
	if carrierPath != "" {
		if err := validateCarrier(carrierPath); err != nil {
			return err
		}
	}
	if st, err := os.Stat(v.Path); err == nil {
		if st.IsDir() {
			return fmt.Errorf("%w: destination path is a directory", ErrVault)
		}
		// File-save dialogs often create a zero-byte placeholder; treat that as free.
		// Embedding into the carrier itself (hide-in-picture) is also allowed.
		embedInPlace := carrierPath != "" && samePath(v.Path, carrierPath)
		if st.Size() == 0 && !st.IsDir() {
			if remErr := os.Remove(v.Path); remErr != nil {
				return fmt.Errorf("%w: could not clear empty placeholder at path: %v", ErrVault, remErr)
			}
		} else if !embedInPlace {
			return fmt.Errorf("%w: a vault already exists at that location", ErrVault)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if profile == "" {
		profile = "standard"
	}
	if err := v.setKDF(profile); err != nil {
		return err
	}
	salt := make([]byte, crypto.SaltSize)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	key, err := crypto.DeriveKeyScrypt(password, salt, v.KdfN, v.KdfR, v.KdfP)
	if err != nil {
		return err
	}
	v.salt = salt
	v.key = key
	v.meta = defaultMeta()
	v.Format = FormatV5
	v.carrierSource = carrierPath
	v.carrierPrefix = 0
	if err := v.writeVault(nil); err != nil {
		v.carrierSource = ""
		return err
	}
	return nil
}

// HasCarrier reports whether the unlocked vault has a leading image/video prefix.
func (v *Vault) HasCarrier() bool {
	return v.carrierPrefix > 0
}

// CarrierPrefix returns the number of leading non-ZIP bytes (0 if none).
func (v *Vault) CarrierPrefix() int64 {
	return v.carrierPrefix
}

// Unlock opens an existing vault. V1/V2 use the legacy PBKDF2/AES format;
// V3/V4/V5 use the Scrypt cascade formats shared with the Python reference.
func (v *Vault) Unlock(password string) error {
	if password == "" {
		return fmt.Errorf("%w: password cannot be empty", ErrVault)
	}
	if err := refuseSymlinkDest(v.Path); err != nil {
		return err
	}
	_, err := os.Stat(v.Path)
	if err != nil {
		return err
	}
	probe, err := os.Open(v.Path)
	if err != nil {
		return err
	}
	var header [len(FormatV1)]byte
	_, _ = io.ReadFull(probe, header[:])
	_ = probe.Close()
	if string(header[:]) == FormatV1 {
		raw, err := os.ReadFile(v.Path)
		if err != nil {
			return err
		}
		return v.unlockV1(password, raw)
	}
	zr, err := zip.OpenReader(v.Path)
	if err != nil {
		return fmt.Errorf("%w: invalid vault format (not a zip). Open a .pulsevault or a picture/video that contains a hidden vault: %v", ErrVault, err)
	}
	defer zr.Close()
	if err := validateZip(zr.File); err != nil {
		return err
	}

	names := map[string]*zip.File{}
	for _, f := range zr.File {
		names[f.Name] = f
	}
	formatEntry, ok := names["format.txt"]
	format := FormatV2
	if ok {
		formatBytes, err := readZipFileLimit(formatEntry, MaxFormatSize)
		if err != nil {
			return err
		}
		format = string(formatBytes)
		if format != FormatV3 && format != FormatV4 && format != FormatV5 {
			return fmt.Errorf("%w: unsupported vault format", ErrVault)
		}
	}
	kdfEntry, hasKDF := names["kdf.json"]
	if format == FormatV5 && !hasKDF {
		return fmt.Errorf("%w: invalid vault KDF record", ErrVault)
	}
	var rec KDFRecord
	if format != FormatV2 {
		if hasKDF {
			kdfBytes, err := readZipFileLimit(kdfEntry, MaxKDFJSONSize)
			if err != nil {
				return err
			}
			if err := json.Unmarshal(kdfBytes, &rec); err != nil {
				return fmt.Errorf("%w: invalid vault KDF record", ErrVault)
			}
			if rec.Algorithm != "scrypt" || rec.N <= 1 || rec.N > crypto.MaxScryptN || rec.N&(rec.N-1) != 0 || rec.R <= 0 || rec.R > crypto.MaxScryptR || rec.P <= 0 || rec.P > crypto.MaxScryptP {
				return fmt.Errorf("%w: invalid vault KDF record", ErrVault)
			}
			v.Profile = rec.Profile
			if v.Profile == "" {
				v.Profile = "standard"
			}
			v.KdfN, v.KdfR, v.KdfP = rec.N, rec.R, rec.P
		} else if err := v.setKDF(legacyScryptProfile()); err != nil {
			return err
		}
	}

	saltEntry, ok := names["salt.bin"]
	if !ok {
		return fmt.Errorf("%w: missing salt.bin", ErrVault)
	}
	salt, err := readZipFileLimit(saltEntry, crypto.SaltSize)
	if err != nil {
		return err
	}
	if len(salt) != crypto.SaltSize {
		return fmt.Errorf("%w: invalid salt", ErrVault)
	}
	metaEntry, ok := names["metadata.enc"]
	if !ok {
		return fmt.Errorf("%w: missing metadata.enc", ErrVault)
	}
	encMeta, err := readZipFileLimit(metaEntry, MaxMetadataSize)
	if err != nil {
		return err
	}
	minMeta := crypto.NonceSize
	if format != FormatV2 {
		minMeta = crypto.NonceSize * 2
	}
	if len(encMeta) < minMeta {
		return fmt.Errorf("%w: corrupted vault metadata", ErrVault)
	}

	var key []byte
	if format == FormatV2 {
		key, err = crypto.DeriveKeyLegacy(password, salt)
	} else {
		key, err = crypto.DeriveKeyScrypt(password, salt, v.KdfN, v.KdfR, v.KdfP)
	}
	if err != nil {
		return err
	}
	cNonce := encMeta[:crypto.NonceSize]
	var plain []byte
	if format == FormatV2 {
		plain, err = crypto.DecryptDataLegacy(key, cNonce, encMeta[crypto.NonceSize:], nil)
	} else {
		aNonce := encMeta[crypto.NonceSize : crypto.NonceSize*2]
		ct := encMeta[crypto.NonceSize*2:]
		plain, err = crypto.DecryptDataV3(key, cNonce, aNonce, ct)
	}
	if err != nil {
		return fmt.Errorf("%w: invalid password or corrupted vault", ErrVault)
	}
	var meta Metadata
	if err := json.Unmarshal(plain, &meta); err != nil {
		return fmt.Errorf("%w: vault decrypted, but internal data is invalid", ErrVault)
	}
	if meta.Files == nil {
		meta.Files = map[string]FileMeta{}
	}
	v.salt = salt
	v.key = key
	v.meta = meta
	v.Format = format
	if off, offErr := zipLocalHeaderOffset(v.Path); offErr == nil {
		v.carrierPrefix = off
	}
	return nil
}

// Legacy V3/V4 containers did not persist their Scrypt parameters. Match the
// Python implementation's historical fallback so test/profile environments
// remain interoperable; production defaults to standard.
func legacyScryptProfile() string {
	profile := strings.ToLower(strings.TrimSpace(os.Getenv("PULSEVAULT_SCRYPT_PROFILE")))
	if _, ok := crypto.Profiles[profile]; ok {
		return profile
	}
	if os.Getenv("PULSEVAULT_TEST_FAST_KDF") == "1" {
		return "fast"
	}
	return "standard"
}

func (v *Vault) unlockV1(password string, raw []byte) error {
	const headerSize = len(FormatV1) + crypto.SaltSize + crypto.NonceSize
	if len(raw) < headerSize || len(raw) > MaxMetadataSize+headerSize {
		return fmt.Errorf("%w: legacy vault is truncated or too large", ErrVault)
	}
	salt := raw[len(FormatV1) : len(FormatV1)+crypto.SaltSize]
	nonce := raw[len(FormatV1)+crypto.SaltSize : headerSize]
	key, err := crypto.DeriveKeyLegacy(password, salt)
	if err != nil {
		return err
	}
	plain, err := crypto.DecryptDataLegacy(key, nonce, raw[headerSize:], []byte(FormatV1))
	if err != nil {
		return fmt.Errorf("%w: invalid password or corrupted legacy vault", ErrVault)
	}
	var meta Metadata
	if err := json.Unmarshal(plain, &meta); err != nil {
		return fmt.Errorf("%w: legacy metadata is invalid", ErrVault)
	}
	if meta.Files == nil {
		meta.Files = map[string]FileMeta{}
	}
	v.salt = append([]byte(nil), salt...)
	v.key = key
	v.meta = meta
	v.Format = FormatV1
	if err := v.setKDF("standard"); err != nil {
		return err
	}
	return nil
}

// Lock clears secrets from memory.
func (v *Vault) Lock() {
	if v.key != nil {
		for i := range v.key {
			v.key[i] = 0
		}
	}
	if v.salt != nil {
		for i := range v.salt {
			v.salt[i] = 0
		}
	}
	v.key = nil
	v.salt = nil
	v.meta = defaultMeta()
	v.Format = FormatV5
	v.carrierSource = ""
	v.carrierPrefix = 0
}

// ListFiles returns sorted vault entry names.
func (v *Vault) ListFiles() ([]string, error) {
	if !v.IsUnlocked() {
		return nil, fmt.Errorf("%w: vault is locked", ErrVault)
	}
	names := make([]string, 0, len(v.meta.Files))
	for n := range v.meta.Files {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		return strings.ToLower(names[i]) < strings.ToLower(names[j])
	})
	return names, nil
}

// AddFile encrypts path into the vault under its base name.
func (v *Vault) AddFile(path string, overwrite bool) error {
	if !v.IsUnlocked() {
		return fmt.Errorf("%w: vault is locked", ErrVault)
	}
	if v.Format == FormatV1 || v.Format == FormatV2 {
		return fmt.Errorf("%w: migrate this legacy vault with change-password before adding files", ErrVault)
	}
	if lst, err := os.Lstat(path); err != nil {
		return fmt.Errorf("%w: selected path is not a file", ErrVault)
	} else if lst.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: refusing to add a symbolic link", ErrVault)
	} else if !lst.Mode().IsRegular() {
		return fmt.Errorf("%w: selected path is not a file", ErrVault)
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return fmt.Errorf("%w: selected path is not a file", ErrVault)
	}
	if samePath(path, v.Path) {
		return fmt.Errorf("%w: cannot add the vault file into itself", ErrVault)
	}
	name, err := safeFilename(filepath.Base(path))
	if err != nil {
		return err
	}
	if _, exists := v.meta.Files[name]; exists && !overwrite {
		return fmt.Errorf("%w: %q already exists in the vault", ErrVault, name)
	}

	src, err := os.Open(path)
	if err != nil {
		return err
	}
	defer src.Close()
	internalID := newID()
	var encBuf bytes.Buffer
	if info.Size() > 0 {
		// Ciphertext is roughly plaintext-sized plus a small header/tag overhead.
		encBuf.Grow(int(info.Size() + info.Size()/8))
	}
	h := sha256.New()
	counting := &hashingReader{r: src, h: h}
	if err := crypto.EncryptStreamV5(v.key, counting, &encBuf, true); err != nil {
		return err
	}
	if counting.size != info.Size() {
		return fmt.Errorf("%w: file changed during encryption", ErrVault)
	}
	oldMeta := cloneMetadata(v.meta)
	oldFormat := v.Format
	now := nowUnix()
	v.meta.Files[name] = FileMeta{
		Name:       name,
		Size:       counting.size,
		SHA256:     hex.EncodeToString(h.Sum(nil)),
		AddedAt:    now,
		UpdatedAt:  now,
		Type:       "file",
		InternalID: internalID,
	}
	v.meta.UpdatedAt = now
	v.meta.Version = 5

	blobs := map[string][]byte{fmt.Sprintf("data/%s.enc", internalID): encBuf.Bytes()}
	// Preserve existing blobs except replaced name's old id.
	if err := v.writeVault(blobs); err != nil {
		v.meta = oldMeta
		v.Format = oldFormat
		return err
	}
	v.Format = FormatV5
	return nil
}

// ExtractFile decrypts filename into outputDir and verifies SHA-256 when recorded.
func (v *Vault) ExtractFile(filename, outputDir string, overwrite bool) (string, error) {
	if !v.IsUnlocked() {
		return "", fmt.Errorf("%w: vault is locked", ErrVault)
	}
	meta, ok := v.meta.Files[filename]
	if !ok {
		return "", fmt.Errorf("%w: file not found in vault", ErrVault)
	}
	safe, err := safeFilename(filename)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	outPath := filepath.Join(outputDir, safe)
	if info, err := os.Lstat(outPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
			return "", fmt.Errorf("%w: refusing to overwrite non-regular output path", ErrVault)
		}
		if !overwrite {
			return "", fmt.Errorf("%w: %q already exists in the output folder", ErrVault, safe)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	tmp := outPath + ".part"
	if info, err := os.Lstat(tmp); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
			return "", fmt.Errorf("%w: refusing to use unsafe temporary output path", ErrVault)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	file, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if err := v.decryptFileTo(meta, io.MultiWriter(file, h)); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		return "", err
	}
	if meta.SHA256 != "" && meta.SHA256 != "pending" && meta.SHA256 != "skipped_large_file" {
		if hex.EncodeToString(h.Sum(nil)) != meta.SHA256 {
			_ = file.Close()
			_ = os.Remove(tmp)
			return "", fmt.Errorf("%w: extracted file hash mismatch", ErrVault)
		}
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, outPath); err != nil {
		if overwrite {
			if removeErr := os.Remove(outPath); removeErr != nil {
				_ = os.Remove(tmp)
				return "", err
			}
			if retryErr := os.Rename(tmp, outPath); retryErr == nil {
				return outPath, nil
			}
		}
		_ = os.Remove(tmp)
		return "", err
	}
	return outPath, nil
}

// VerifyResult is the outcome of integrity-checking one vault entry.
type VerifyResult struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	HashChecked bool   `json:"hash_checked"`
}

// VerifyFile decrypts an entry and checks size / SHA-256 against metadata.
func (v *Vault) VerifyFile(filename string) (VerifyResult, error) {
	if !v.IsUnlocked() {
		return VerifyResult{}, fmt.Errorf("%w: vault is locked", ErrVault)
	}
	meta, ok := v.meta.Files[filename]
	if !ok {
		return VerifyResult{}, fmt.Errorf("%w: file not found in vault", ErrVault)
	}
	plain, err := v.decryptFileBytes(meta)
	if err != nil {
		return VerifyResult{}, err
	}
	actual := crypto.SHA256Hex(plain)
	hashChecked := meta.SHA256 != "" && meta.SHA256 != "pending" && meta.SHA256 != "skipped_large_file"
	if hashChecked && actual != meta.SHA256 {
		return VerifyResult{}, fmt.Errorf("%w: hash mismatch for %q", ErrVault, filename)
	}
	if int64(len(plain)) != meta.Size {
		return VerifyResult{}, fmt.Errorf("%w: size mismatch for %q", ErrVault, filename)
	}
	return VerifyResult{
		Name:        filename,
		Size:        int64(len(plain)),
		SHA256:      actual,
		HashChecked: hashChecked,
	}, nil
}

// VerifyAll integrity-checks every file; returns counts for CLI/status.
func (v *Vault) VerifyAll() (fileCount int, bytesChecked int64, hashCheckedCount int, err error) {
	names, err := v.ListFiles()
	if err != nil {
		return 0, 0, 0, err
	}
	for _, name := range names {
		res, verr := v.VerifyFile(name)
		if verr != nil {
			return fileCount, bytesChecked, hashCheckedCount, verr
		}
		fileCount++
		bytesChecked += res.Size
		if res.HashChecked {
			hashCheckedCount++
		}
	}
	return fileCount, bytesChecked, hashCheckedCount, nil
}

// DeleteFile removes an entry and rewrites the vault without its blob.
func (v *Vault) DeleteFile(filename string) error {
	if !v.IsUnlocked() {
		return fmt.Errorf("%w: vault is locked", ErrVault)
	}
	if v.Format == FormatV1 || v.Format == FormatV2 {
		return fmt.Errorf("%w: migrate this legacy vault with change-password before modifying it", ErrVault)
	}
	if _, ok := v.meta.Files[filename]; !ok {
		return fmt.Errorf("%w: file not found in vault", ErrVault)
	}
	oldMeta := cloneMetadata(v.meta)
	delete(v.meta.Files, filename)
	v.meta.UpdatedAt = nowUnix()
	if err := v.writeVault(nil); err != nil {
		v.meta = oldMeta
		return err
	}
	return nil
}

// ChangePassword re-encrypts every blob and metadata under a new salt+key.
// KDF profile (n,r,p) is preserved.
func (v *Vault) ChangePassword(oldPassword, newPassword string) error {
	if !v.IsUnlocked() {
		return fmt.Errorf("%w: vault is locked", ErrVault)
	}
	if newPassword == "" {
		return fmt.Errorf("%w: password cannot be empty", ErrVault)
	}
	var candidate []byte
	var err error
	if v.Format == FormatV1 || v.Format == FormatV2 {
		candidate, err = crypto.DeriveKeyLegacy(oldPassword, v.salt)
	} else {
		candidate, err = crypto.DeriveKeyScrypt(oldPassword, v.salt, v.KdfN, v.KdfR, v.KdfP)
	}
	if err != nil {
		return err
	}
	if !hmac.Equal(candidate, v.key) {
		return fmt.Errorf("%w: current password is incorrect", ErrVault)
	}

	oldFormat, oldProfile, oldN, oldR, oldP := v.Format, v.Profile, v.KdfN, v.KdfR, v.KdfP
	wasLegacy := oldFormat == FormatV1 || oldFormat == FormatV2
	oldMeta := cloneMetadata(v.meta)
	committed := false
	defer func() {
		if !committed {
			v.meta = oldMeta
			v.Format, v.Profile, v.KdfN, v.KdfR, v.KdfP = oldFormat, oldProfile, oldN, oldR, oldP
		}
	}()
	if v.Format == FormatV1 || v.Format == FormatV2 {
		if err := v.setKDF("standard"); err != nil {
			return err
		}
	}
	newSalt := make([]byte, crypto.SaltSize)
	if _, err := rand.Read(newSalt); err != nil {
		return err
	}
	newKey, err := crypto.DeriveKeyScrypt(newPassword, newSalt, v.KdfN, v.KdfR, v.KdfP)
	if err != nil {
		return err
	}

	// Decrypt each file with the old key, encrypt with the new key (same internal ids).
	newBlobs := map[string][]byte{}
	for _, meta := range v.meta.Files {
		if meta.InternalID == "" {
			continue
		}
		plain, err := v.decryptFileBytes(meta)
		if err != nil {
			return err
		}
		var encBuf bytes.Buffer
		if err := crypto.EncryptStreamV5(newKey, bytes.NewReader(plain), &encBuf, true); err != nil {
			return err
		}
		newBlobs[fmt.Sprintf("data/%s.enc", meta.InternalID)] = encBuf.Bytes()
	}

	oldKey, oldSalt := v.key, v.salt
	v.key = newKey
	v.salt = newSalt
	v.Format = FormatV5
	v.legacySource = wasLegacy
	if err := v.writeVault(newBlobs); err != nil {
		v.key = oldKey
		v.salt = oldSalt
		v.legacySource = false
		return err
	}
	v.legacySource = false
	// Zero old key material best-effort
	for i := range oldKey {
		oldKey[i] = 0
	}
	for i := range oldSalt {
		oldSalt[i] = 0
	}
	committed = true
	return nil
}

// GetFileMeta returns metadata for one entry.
func (v *Vault) GetFileMeta(filename string) (FileMeta, error) {
	if !v.IsUnlocked() {
		return FileMeta{}, fmt.Errorf("%w: vault is locked", ErrVault)
	}
	meta, ok := v.meta.Files[filename]
	if !ok {
		return FileMeta{}, fmt.Errorf("%w: file not found in vault", ErrVault)
	}
	return meta, nil
}

// PeekKDFProfile reads kdf.json without unlocking (for CLI info / tests).
func PeekKDFProfile(path string) (KDFRecord, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return KDFRecord{}, fmt.Errorf("%w: invalid vault zip: %v", ErrVault, err)
	}
	defer zr.Close()
	if err := validateZip(zr.File); err != nil {
		return KDFRecord{}, err
	}
	for _, f := range zr.File {
		if f.Name != "kdf.json" {
			continue
		}
		b, err := readZipFileLimit(f, MaxKDFJSONSize)
		if err != nil {
			return KDFRecord{}, err
		}
		var rec KDFRecord
		if err := json.Unmarshal(b, &rec); err != nil {
			return KDFRecord{}, fmt.Errorf("%w: invalid vault KDF record", ErrVault)
		}
		return rec, nil
	}
	return KDFRecord{}, fmt.Errorf("%w: invalid vault KDF record", ErrVault)
}

func (v *Vault) decryptFileBytes(meta FileMeta) ([]byte, error) {
	var plain bytes.Buffer
	if err := v.decryptFileTo(meta, &plain); err != nil {
		return nil, err
	}
	return plain.Bytes(), nil
}

func (v *Vault) decryptFileTo(meta FileMeta, dst io.Writer) error {
	if meta.Content != "" {
		plain, err := base64.StdEncoding.DecodeString(meta.Content)
		if err != nil {
			return fmt.Errorf("%w: invalid legacy inline content", ErrVault)
		}
		_, err = dst.Write(plain)
		return err
	}
	if meta.InternalID == "" {
		return fmt.Errorf("%w: file metadata missing content reference", ErrVault)
	}
	zr, err := zip.OpenReader(v.Path)
	if err != nil {
		return err
	}
	defer zr.Close()
	want := fmt.Sprintf("data/%s.enc", meta.InternalID)
	var src *zip.File
	for _, f := range zr.File {
		if f.Name == want {
			src = f
			break
		}
	}
	if src == nil {
		return fmt.Errorf("%w: internal data file missing from vault", ErrVault)
	}
	rc, err := src.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	limited := io.LimitReader(rc, int64(MaxDataBlobSize)+1)
	var decryptErr error
	switch v.Format {
	case FormatV2:
		data, readErr := io.ReadAll(limited)
		if readErr != nil {
			return readErr
		}
		if int64(len(data)) > MaxDataBlobSize || len(data) < crypto.NonceSize {
			return fmt.Errorf("%w: corrupted legacy data", ErrVault)
		}
		plainData, err := crypto.DecryptDataLegacy(v.key, data[:crypto.NonceSize], data[crypto.NonceSize:], nil)
		if err == nil {
			_, err = dst.Write(plainData)
		}
		decryptErr = err
	case FormatV3:
		data, readErr := io.ReadAll(limited)
		if readErr != nil {
			return readErr
		}
		if int64(len(data)) > MaxDataBlobSize || len(data) < crypto.NonceSize*2 {
			return fmt.Errorf("%w: corrupted legacy data", ErrVault)
		}
		plainData, err := crypto.DecryptDataV3(v.key, data[:crypto.NonceSize], data[crypto.NonceSize:crypto.NonceSize*2], data[crypto.NonceSize*2:])
		if err == nil {
			_, err = dst.Write(plainData)
		}
		decryptErr = err
	case FormatV4:
		decryptErr = crypto.DecryptStreamV4(v.key, limited, dst)
	default:
		decryptErr = crypto.DecryptStreamV5(v.key, limited, dst)
	}
	if decryptErr != nil {
		return fmt.Errorf("%w: failed to decrypt file", ErrVault)
	}
	return nil
}

func (v *Vault) writeVault(newBlobs map[string][]byte) error {
	if v.key == nil || v.salt == nil {
		return fmt.Errorf("%w: vault is locked", ErrVault)
	}
	if len(v.key) != crypto.V3KeySize {
		return fmt.Errorf("%w: legacy vault must be migrated before writing", ErrVault)
	}
	if info, err := os.Lstat(v.Path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: refusing to replace a symbolic link", ErrVault)
	}
	// Kept data/*.enc blobs are copied from the old ZIP; only new/legacy blobs stay in memory.
	memBlobs := map[string][]byte{}
	var oldZip *zip.ReadCloser
	defer func() {
		if oldZip != nil {
			_ = oldZip.Close()
		}
	}()
	var carrierPrefixSize int64
	prefixSrc := ""
	if v.carrierSource != "" {
		if err := validateCarrier(v.carrierSource); err != nil {
			return err
		}
		st, err := os.Stat(v.carrierSource)
		if err != nil {
			return err
		}
		carrierPrefixSize = st.Size()
		prefixSrc = v.carrierSource
	} else if _, statErr := os.Stat(v.Path); statErr == nil && v.Format != FormatV1 && !v.legacySource {
		zr, err := zip.OpenReader(v.Path)
		if err != nil {
			return fmt.Errorf("%w: existing vault is not a readable ZIP", ErrVault)
		}
		if err := validateZip(zr.File); err != nil {
			_ = zr.Close()
			return err
		}
		carrierPrefixSize, err = zipLocalHeaderOffset(v.Path)
		if err != nil {
			_ = zr.Close()
			return err
		}
		if carrierPrefixSize > 0 {
			prefixSrc = v.Path
		}
		oldZip = zr
	} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}
	if err := v.materializeLegacyContent(memBlobs); err != nil {
		return err
	}
	for k, val := range newBlobs {
		memBlobs[k] = val
	}

	referenced := map[string]struct{}{}
	for _, m := range v.meta.Files {
		if m.InternalID != "" {
			referenced[fmt.Sprintf("data/%s.enc", m.InternalID)] = struct{}{}
		}
	}
	for name := range memBlobs {
		if _, ok := referenced[name]; !ok {
			delete(memBlobs, name)
		}
	}
	oldByName := map[string]*zip.File{}
	if oldZip != nil {
		for _, f := range oldZip.File {
			if _, ok := referenced[f.Name]; !ok {
				continue
			}
			if _, inMem := memBlobs[f.Name]; inMem {
				continue
			}
			oldByName[f.Name] = f
		}
	}

	v.meta.UpdatedAt = nowUnix()
	v.meta.Version = 5
	metaPlain, err := json.Marshal(v.meta)
	if err != nil {
		return err
	}
	// Python uses separators=(',', ':') compact JSON — Go's Marshal is also compact.
	cNonce, aNonce, ct, err := crypto.EncryptDataV3(v.key, metaPlain)
	if err != nil {
		return err
	}
	encMeta := append(append(append([]byte{}, cNonce...), aNonce...), ct...)

	kdf := KDFRecord{
		Algorithm: "scrypt",
		Profile:   v.Profile,
		N:         v.KdfN,
		R:         v.KdfR,
		P:         v.KdfP,
	}
	kdfBytes, err := json.Marshal(kdf)
	if err != nil {
		return err
	}

	dir := filepath.Dir(v.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(v.Path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if carrierPrefixSize > 0 && prefixSrc != "" {
		carrier, err := os.Open(prefixSrc)
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return err
		}
		copied, copyErr := io.CopyN(tmp, carrier, carrierPrefixSize)
		closeErr := carrier.Close()
		if copyErr != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return copyErr
		}
		if copied != carrierPrefixSize {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return fmt.Errorf("%w: carrier prefix was truncated while writing", ErrVault)
		}
		if closeErr != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return closeErr
		}
	}
	zw := zip.NewWriter(tmp)
	// Pulse-Vault V5 requires ZIP_STORED (no zip compression) for all members.
	write := func(name string, data []byte) error {
		w, err := zw.CreateHeader(storedZipHeader(name))
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	copyStored := func(name string, f *zip.File) error {
		if f.UncompressedSize64 > MaxDataBlobSize {
			return fmt.Errorf("%w: ZIP entry %q is too large", ErrVault, name)
		}
		w, err := zw.CreateHeader(storedZipHeader(name))
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		n, copyErr := io.Copy(w, io.LimitReader(rc, int64(MaxDataBlobSize)+1))
		closeErr := rc.Close()
		if copyErr != nil {
			return copyErr
		}
		if uint64(n) > MaxDataBlobSize {
			return fmt.Errorf("%w: ZIP entry %q is too large", ErrVault, name)
		}
		return closeErr
	}
	failWrite := func(err error) error {
		_ = zw.Close()
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := write("salt.bin", v.salt); err != nil {
		return failWrite(err)
	}
	if err := write("format.txt", []byte(FormatV5)); err != nil {
		return failWrite(err)
	}
	if err := write("kdf.json", kdfBytes); err != nil {
		return failWrite(err)
	}
	// Stable order for data blobs: memory (new/legacy) first-choice, else stream from old ZIP.
	var blobNames []string
	for n := range referenced {
		if _, ok := memBlobs[n]; ok {
			blobNames = append(blobNames, n)
			continue
		}
		if _, ok := oldByName[n]; ok {
			blobNames = append(blobNames, n)
		}
	}
	sort.Strings(blobNames)
	for _, n := range blobNames {
		if data, ok := memBlobs[n]; ok {
			if err := write(n, data); err != nil {
				return failWrite(err)
			}
			continue
		}
		if err := copyStored(n, oldByName[n]); err != nil {
			return failWrite(err)
		}
	}
	if err := write("metadata.enc", encMeta); err != nil {
		return failWrite(err)
	}
	if err := zw.Close(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	// Close the old ZIP before rename so Windows can replace the file.
	if oldZip != nil {
		err := oldZip.Close()
		oldZip = nil
		if err != nil {
			_ = os.Remove(tmpName)
			return err
		}
	}
	if err := os.Rename(tmpName, v.Path); err != nil {
		// Windows may need remove-first if target exists
		_ = os.Remove(v.Path)
		if err2 := os.Rename(tmpName, v.Path); err2 != nil {
			_ = os.Remove(tmpName)
			return err2
		}
	}
	v.carrierPrefix = carrierPrefixSize
	v.carrierSource = ""
	return nil
}

func (v *Vault) materializeLegacyContent(existing map[string][]byte) error {
	for name, meta := range v.meta.Files {
		if meta.Content == "" || meta.InternalID != "" {
			continue
		}
		plain, err := base64.StdEncoding.DecodeString(meta.Content)
		if err != nil {
			return fmt.Errorf("%w: invalid legacy inline content for %q", ErrVault, name)
		}
		id := newID()
		var enc bytes.Buffer
		if err := crypto.EncryptStreamV5(v.key, bytes.NewReader(plain), &enc, true); err != nil {
			return err
		}
		existing[fmt.Sprintf("data/%s.enc", id)] = enc.Bytes()
		meta.InternalID = id
		meta.Content = ""
		v.meta.Files[name] = meta
	}
	return nil
}

func validateZip(files []*zip.File) error {
	if len(files) > MaxZipEntries {
		return fmt.Errorf("%w: vault contains too many ZIP entries", ErrVault)
	}
	seen := make(map[string]struct{}, len(files))
	for _, f := range files {
		if _, exists := seen[f.Name]; exists {
			return fmt.Errorf("%w: duplicate ZIP entry %q", ErrVault, f.Name)
		}
		seen[f.Name] = struct{}{}
		if f.Method != zip.Store {
			return fmt.Errorf("%w: ZIP entry %q is compressed", ErrVault, f.Name)
		}
		if f.Name == "" || strings.HasPrefix(f.Name, "/") || strings.HasPrefix(f.Name, "\\") || strings.Contains(f.Name, "\\") {
			return fmt.Errorf("%w: invalid ZIP entry path", ErrVault)
		}
		for _, part := range strings.Split(f.Name, "/") {
			if part == ".." {
				return fmt.Errorf("%w: invalid ZIP entry path", ErrVault)
			}
		}
		max := uint64(MaxDataBlobSize)
		switch f.Name {
		case "salt.bin":
			if f.UncompressedSize64 != crypto.SaltSize {
				return fmt.Errorf("%w: invalid salt size", ErrVault)
			}
		case "format.txt":
			max = MaxFormatSize
		case "kdf.json":
			max = MaxKDFJSONSize
		case "metadata.enc":
			max = MaxMetadataSize
		default:
			if !strings.HasPrefix(f.Name, "data/") {
				return fmt.Errorf("%w: unexpected ZIP entry %q", ErrVault, f.Name)
			}
		}
		if f.UncompressedSize64 > max {
			return fmt.Errorf("%w: ZIP entry %q is too large", ErrVault, f.Name)
		}
	}
	return nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	return readZipFileLimit(f, MaxDataBlobSize)
}

func readZipFileLimit(f *zip.File, max uint64) ([]byte, error) {
	if f.UncompressedSize64 > max {
		return nil, fmt.Errorf("%w: ZIP entry is too large", ErrVault)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, int64(max)+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(data)) > max {
		return nil, fmt.Errorf("%w: ZIP entry is too large", ErrVault)
	}
	return data, nil
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	// UUID v4-ish hex without dashes for internal blob ids (Python uses uuid4 with dashes).
	// Match Python uuid4 string form for interop friendliness.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type hashingReader struct {
	r    io.Reader
	h    io.Writer
	size int64
}

func (r *hashingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		_, _ = r.h.Write(p[:n])
		r.size += int64(n)
	}
	return n, err
}

func cloneMetadata(meta Metadata) Metadata {
	copyMeta := meta
	copyMeta.Files = make(map[string]FileMeta, len(meta.Files))
	for name, item := range meta.Files {
		copyMeta.Files[name] = item
	}
	return copyMeta
}

func safeFilename(name string) (string, error) {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	var b strings.Builder
	for _, ch := range name {
		if ch < 32 || strings.ContainsRune(`<>:"|?*`, ch) {
			b.WriteByte('_')
		} else {
			b.WriteRune(ch)
		}
	}
	name = strings.TrimRight(b.String(), " .")
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("%w: invalid filename", ErrVault)
	}
	return name, nil
}
