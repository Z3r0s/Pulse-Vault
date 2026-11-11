// Package updatecheck checks the public GitHub release feed only when called
// and saves release assets only after validating their published SHA-256.
package updatecheck

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	APIEndpoint       = "https://api.github.com/repos/Z3r0s/Pulse-Vault/releases/latest"
	releasePagePrefix = "https://github.com/Z3r0s/Pulse-Vault/releases/tag/"
	releaseAssetRoot  = "/Z3r0s/Pulse-Vault/releases/download/"
	userAgent         = "Pulse-Vault-update-check"
	apiVersion        = "2022-11-28"
	maxReleaseJSON    = 1 << 20
	maxChecksumBytes  = 1 << 20
	maxUpdateBytes    = 256 << 20
	copyBufferSize    = 64 << 10
)

// Client talks to the public release API. Constructing a client performs no
// network activity; Check and Download are explicit, caller-initiated actions.
type Client struct {
	httpClient *http.Client
	apiURL     string
}

// Asset describes one published release asset.
type Asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

// Release contains the latest stable release and a comparison with the
// version supplied by the running application.
type Release struct {
	TagName         string  `json:"tag_name"`
	Name            string  `json:"name"`
	HTMLURL         string  `json:"html_url"`
	Body            string  `json:"body"`
	Draft           bool    `json:"draft"`
	Prerelease      bool    `json:"prerelease"`
	Assets          []Asset `json:"assets"`
	CurrentVersion  string  `json:"-"`
	Comparable      bool    `json:"-"`
	UpdateAvailable bool    `json:"-"`
}

// New returns a client with bounded timeouts and redirects limited to GitHub's
// API, release pages, and release-asset host. It does not make a request.
func New() *Client {
	return newClient(APIEndpoint, &http.Client{
		Timeout: 45 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects while contacting GitHub")
			}
			if !trustedGitHubHost(req.URL) {
				return fmt.Errorf("refusing update redirect to %s", req.URL.Hostname())
			}
			return nil
		},
	})
}

func newClient(apiURL string, client *http.Client) *Client {
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	return &Client{httpClient: client, apiURL: apiURL}
}

// Check fetches the latest stable GitHub release. This method is the only
// operation that requests release metadata, so applications can keep checks
// fully opt-in.
func (c *Client) Check(ctx context.Context, currentVersion string) (Release, error) {
	var release Release
	if err := ctx.Err(); err != nil {
		return release, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiURL, nil)
	if err != nil {
		return release, fmt.Errorf("prepare GitHub release check: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", apiVersion)
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return release, fmt.Errorf("contact GitHub for release information: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusNotFound:
			return release, errors.New("GitHub has no published stable Pulse-Vault release to check yet")
		case http.StatusForbidden, http.StatusTooManyRequests:
			return release, errors.New("GitHub declined the update check, possibly because of rate limits or network policy; try again later or open the Releases page")
		default:
			return release, fmt.Errorf("GitHub release check returned %s", resp.Status)
		}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReleaseJSON+1))
	if err != nil {
		return release, fmt.Errorf("read GitHub release information: %w", err)
	}
	if len(body) > maxReleaseJSON {
		return release, errors.New("GitHub release response is larger than expected")
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return release, fmt.Errorf("parse GitHub release information: %w", err)
	}
	if release.Draft || release.Prerelease {
		return release, errors.New("GitHub did not return a stable published release")
	}
	if _, err := parseVersion(release.TagName); err != nil {
		return release, fmt.Errorf("GitHub returned an invalid release version: %w", err)
	}
	if !validReleasePageURL(release.HTMLURL, release.TagName) {
		return release, errors.New("GitHub returned an unexpected release page URL")
	}
	release.CurrentVersion = strings.TrimSpace(currentVersion)
	current, currentErr := parseVersion(currentVersion)
	latest, _ := parseVersion(release.TagName)
	if currentErr == nil {
		release.Comparable = true
		release.UpdateAvailable = compareVersion(latest, current) > 0
	} else {
		// A development build has no comparable release number. The UI can
		// still offer the published build without claiming it is newer.
		release.Comparable = false
		release.UpdateAvailable = false
	}
	return release, nil
}

// DesktopAssetName returns the release asset naming convention used by the
// GUI release workflow. The release itself determines whether that asset exists.
func DesktopAssetName(goos, goarch string) (string, bool) {
	if goos != "windows" && goos != "linux" && goos != "darwin" {
		return "", false
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", false
	}
	name := "pulse-vault-gui-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name, true
}

// FindAsset returns the exact named asset, without interpreting its path.
func (r Release) FindAsset(name string) (Asset, bool) {
	for _, asset := range r.Assets {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

// Download saves a release asset after verifying it against SHA256SUMS and,
// when GitHub supplies one, the asset digest from the release API. It never
// overwrites an existing destination and removes partial files on failure.
func (c *Client) Download(ctx context.Context, release Release, assetName, destination string, progress func(done, total int64)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateAssetName(assetName); err != nil {
		return err
	}
	if destination == "" {
		return errors.New("choose a destination for the update")
	}
	if err := destinationAbsent(destination); err != nil {
		return err
	}
	asset, ok := release.FindAsset(assetName)
	if !ok {
		return fmt.Errorf("this release does not include %s", assetName)
	}
	if err := validateAssetURL(asset.BrowserDownloadURL, release.TagName, assetName); err != nil {
		return err
	}
	if asset.Size <= 0 || asset.Size > maxUpdateBytes {
		return fmt.Errorf("release asset size %d is outside the allowed update size", asset.Size)
	}
	checksumAsset, ok := release.FindAsset("SHA256SUMS")
	if !ok {
		return errors.New("this release has no SHA256SUMS verification file; download from the release page only after reviewing its notes")
	}
	manifest, err := c.readSmallAsset(ctx, release.TagName, checksumAsset, maxChecksumBytes)
	if err != nil {
		return fmt.Errorf("download release checksum list: %w", err)
	}
	expected, err := checksumFor(manifest, assetName)
	if err != nil {
		return err
	}
	if apiDigest := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(asset.Digest)), "sha256:"); apiDigest != "" {
		if !isSHA256(apiDigest) {
			return errors.New("GitHub returned an invalid SHA-256 asset digest")
		}
		if apiDigest != expected {
			return errors.New("the release checksum list does not match GitHub's asset digest")
		}
	}

	req, err := c.newAssetRequest(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download update from GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub update download returned %s", resp.Status)
	}
	if resp.ContentLength > maxUpdateBytes || (resp.ContentLength >= 0 && resp.ContentLength != asset.Size) {
		return errors.New("download size does not match the published release asset")
	}

	temp, err := os.CreateTemp(filepath.Dir(destination), ".pulse-vault-update-*.part")
	if err != nil {
		return fmt.Errorf("prepare a temporary update file: %w", err)
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("protect temporary update file: %w", err)
	}
	hasher := sha256.New()
	written, err := copyWithProgress(ctx, io.MultiWriter(temp, hasher), resp.Body, asset.Size, progress)
	if err != nil {
		_ = temp.Close()
		return fmt.Errorf("save update download: %w", err)
	}
	if written != asset.Size {
		_ = temp.Close()
		return fmt.Errorf("download ended at %d bytes; expected %d", written, asset.Size)
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return fmt.Errorf("flush temporary update: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary update: %w", err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expected {
		return errors.New("downloaded update failed SHA-256 verification; the file was discarded")
	}
	if err := publishVerifiedFile(tempName, destination); err != nil {
		return err
	}
	if progress != nil {
		progress(written, asset.Size)
	}
	return nil
}

func (c *Client) readSmallAsset(ctx context.Context, tag string, asset Asset, limit int64) ([]byte, error) {
	if err := validateAssetURL(asset.BrowserDownloadURL, tag, asset.Name); err != nil {
		return nil, err
	}
	if asset.Size <= 0 || asset.Size > limit {
		return nil, fmt.Errorf("checksum file size %d is outside the allowed limit", asset.Size)
	}
	req, err := c.newAssetRequest(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request checksum file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksum download returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit || int64(len(body)) != asset.Size {
		return nil, errors.New("checksum file size does not match the release metadata")
	}
	return body, nil
}

func (c *Client) newAssetRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	if err := validateAssetURL(rawURL, "", path.Base(rawURL)); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("prepare GitHub asset request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

func copyWithProgress(ctx context.Context, dst io.Writer, src io.Reader, total int64, progress func(int64, int64)) (int64, error) {
	buffer := make([]byte, copyBufferSize)
	var written int64
	lastReport := time.Time{}
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		n, readErr := src.Read(buffer)
		if n > 0 {
			count, writeErr := dst.Write(buffer[:n])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
			if count != n {
				return written, io.ErrShortWrite
			}
			if progress != nil && time.Since(lastReport) >= 100*time.Millisecond {
				progress(written, total)
				lastReport = time.Now()
			}
			if written > maxUpdateBytes {
				return written, errors.New("download exceeded the update size limit")
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return written, nil
			}
			return written, readErr
		}
	}
}

func publishVerifiedFile(tempPath, destination string) error {
	input, err := os.Open(tempPath)
	if err != nil {
		return fmt.Errorf("open verified update: %w", err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o700)
	if err != nil {
		return fmt.Errorf("create update file without replacing another file: %w", err)
	}
	cleanup := true
	defer func() {
		_ = output.Close()
		if cleanup {
			_ = os.Remove(destination)
		}
	}()
	if _, err := io.Copy(output, input); err != nil {
		return fmt.Errorf("copy verified update to destination: %w", err)
	}
	if err := output.Sync(); err != nil {
		return fmt.Errorf("flush verified update: %w", err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close verified update: %w", err)
	}
	cleanup = false
	return nil
}

func destinationAbsent(filename string) error {
	_, err := os.Lstat(filename)
	if err == nil {
		return fmt.Errorf("a file already exists at %q; choose another name", filename)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check update destination: %w", err)
	}
	info, err := os.Stat(filepath.Dir(filename))
	if err != nil {
		return fmt.Errorf("access update destination folder: %w", err)
	}
	if !info.IsDir() {
		return errors.New("update destination folder is not a directory")
	}
	return nil
}

func validateAssetName(name string) error {
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) || name == "." || name == ".." {
		return errors.New("invalid release asset name")
	}
	return nil
}

func validateAssetURL(rawURL, tag, assetName string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid GitHub asset URL: %w", err)
	}
	if u.Scheme != "https" || u.Hostname() != "github.com" || u.User != nil || (u.Port() != "" && u.Port() != "443") || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("refusing a release asset URL outside the expected GitHub host")
	}
	if tag != "" {
		if path.Base(u.Path) != assetName || path.Base(path.Dir(u.Path)) != tag || path.Dir(path.Dir(u.Path)) != strings.TrimSuffix(releaseAssetRoot, "/") {
			return errors.New("release asset URL does not match this repository, version, and file")
		}
	} else if path.Base(u.Path) != assetName || !strings.HasPrefix(u.Path, releaseAssetRoot) {
		return errors.New("release asset URL does not match the expected GitHub release path")
	}
	return nil
}

func validReleasePageURL(rawURL, tag string) bool {
	u, err := url.Parse(rawURL)
	return err == nil && u.Scheme == "https" && u.Hostname() == "github.com" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.Path == "/Z3r0s/Pulse-Vault/releases/tag/"+tag
}

func trustedGitHubHost(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "api.github.com", "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
		return true
	default:
		return false
	}
}

func checksumFor(manifest []byte, name string) (string, error) {
	if err := validateAssetName(name); err != nil {
		return "", err
	}
	var match string
	scanner := bufio.NewScanner(strings.NewReader(string(manifest)))
	scanner.Buffer(make([]byte, 1024), maxChecksumBytes)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			continue
		}
		fileName := strings.TrimPrefix(fields[1], "*")
		if fileName != name {
			continue
		}
		digest := strings.ToLower(fields[0])
		if !isSHA256(digest) {
			return "", fmt.Errorf("SHA256SUMS contains an invalid digest for %s", name)
		}
		if match != "" {
			return "", fmt.Errorf("SHA256SUMS contains duplicate entries for %s", name)
		}
		match = digest
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read SHA256SUMS: %w", err)
	}
	if match == "" {
		return "", fmt.Errorf("SHA256SUMS has no entry for %s", name)
	}
	return match, nil
}

func isSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

type semanticVersion struct {
	major, minor, patch uint64
	pre                 []string
}

// CompareVersions compares two semantic versions after accepting an optional
// leading v. Build metadata does not affect precedence.
func CompareVersions(a, b string) (int, error) {
	left, err := parseVersion(a)
	if err != nil {
		return 0, fmt.Errorf("parse version %q: %w", a, err)
	}
	right, err := parseVersion(b)
	if err != nil {
		return 0, fmt.Errorf("parse version %q: %w", b, err)
	}
	return compareVersion(left, right), nil
}

func parseVersion(raw string) (semanticVersion, error) {
	var version semanticVersion
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(strings.TrimPrefix(value, "v"), "V")
	if value == "" {
		return version, errors.New("version is empty")
	}
	if strings.Count(value, "+") > 1 {
		return version, errors.New("invalid build metadata")
	}
	if coreAndPre, metadata, hasMetadata := strings.Cut(value, "+"); hasMetadata {
		if !validIdentifiers(metadata, false) {
			return version, errors.New("invalid build metadata")
		}
		value = coreAndPre
	}
	core, pre, hasPre := strings.Cut(value, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return version, errors.New("expected major.minor.patch")
	}
	components := []*uint64{&version.major, &version.minor, &version.patch}
	for i, part := range parts {
		if part == "" || (len(part) > 1 && part[0] == '0') {
			return version, errors.New("invalid numeric version component")
		}
		parsed, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return version, errors.New("invalid numeric version component")
		}
		*components[i] = parsed
	}
	if hasPre {
		if pre == "" {
			return version, errors.New("empty prerelease identifier")
		}
		version.pre = strings.Split(pre, ".")
		for _, part := range version.pre {
			if !validIdentifier(part) {
				return version, errors.New("invalid prerelease identifier")
			}
			if numericIdentifier(part) && len(part) > 1 && part[0] == '0' {
				return version, errors.New("numeric prerelease identifiers cannot have leading zeroes")
			}
		}
	}
	return version, nil
}

func compareVersion(a, b semanticVersion) int {
	for _, pair := range [][2]uint64{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] < pair[1] {
			return -1
		}
		if pair[0] > pair[1] {
			return 1
		}
	}
	if len(a.pre) == 0 && len(b.pre) != 0 {
		return 1
	}
	if len(a.pre) != 0 && len(b.pre) == 0 {
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		left, right := a.pre[i], b.pre[i]
		leftNum, rightNum := numericIdentifier(left), numericIdentifier(right)
		if leftNum && rightNum {
			if len(left) < len(right) {
				return -1
			}
			if len(left) > len(right) {
				return 1
			}
			if left < right {
				return -1
			}
			if left > right {
				return 1
			}
			continue
		}
		if leftNum != rightNum {
			if leftNum {
				return -1
			}
			return 1
		}
		if left < right {
			return -1
		}
		if left > right {
			return 1
		}
	}
	if len(a.pre) < len(b.pre) {
		return -1
	}
	if len(a.pre) > len(b.pre) {
		return 1
	}
	return 0
}

func numericIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func validIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if !(ch >= '0' && ch <= '9') && !(ch >= 'A' && ch <= 'Z') && !(ch >= 'a' && ch <= 'z') && ch != '-' {
			return false
		}
	}
	return true
}

func validIdentifiers(value string, rejectNumericLeadingZero bool) bool {
	if value == "" {
		return false
	}
	for _, part := range strings.Split(value, ".") {
		if !validIdentifier(part) || (rejectNumericLeadingZero && numericIdentifier(part) && len(part) > 1 && part[0] == '0') {
			return false
		}
	}
	return true
}
