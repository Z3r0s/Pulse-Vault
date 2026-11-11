package updatecheck

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func response(request *http.Request, status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode:    status,
		Status:        fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:        make(http.Header),
		Body:          io.NopCloser(strings.NewReader(string(body))),
		ContentLength: int64(len(body)),
		Request:       request,
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		left, right string
		want        int
	}{
		{"v1.2.4", "1.2.3", 1},
		{"1.2.3", "v1.2.3", 0},
		{"1.3.0", "1.2.99", 1},
		{"1.2.3-alpha.2", "1.2.3-alpha.10", -1},
		{"1.2.3-alpha", "1.2.3", -1},
		{"1.2.3+build.4", "1.2.3+build.1", 0},
	}
	for _, test := range tests {
		got, err := CompareVersions(test.left, test.right)
		if err != nil {
			t.Fatalf("CompareVersions(%q, %q): %v", test.left, test.right, err)
		}
		if got != test.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
	if _, err := CompareVersions("dev", "1.2.3"); err == nil {
		t.Fatal("development build should not be treated as a semantic version")
	}
}

func TestDesktopAssetName(t *testing.T) {
	tests := []struct {
		goos, goarch, want string
		ok                 bool
	}{
		{"windows", "amd64", "pulse-vault-gui-windows-amd64.exe", true},
		{"linux", "arm64", "pulse-vault-gui-linux-arm64", true},
		{"darwin", "arm64", "pulse-vault-gui-darwin-arm64", true},
		{"freebsd", "amd64", "", false},
		{"windows", "386", "", false},
	}
	for _, test := range tests {
		got, ok := DesktopAssetName(test.goos, test.goarch)
		if got != test.want || ok != test.ok {
			t.Errorf("DesktopAssetName(%q, %q) = (%q, %v), want (%q, %v)", test.goos, test.goarch, got, ok, test.want, test.ok)
		}
	}
}

func TestCheckReportsAvailableEqualAndDevelopmentVersions(t *testing.T) {
	body := []byte(`{"tag_name":"v1.3.0","name":"Pulse-Vault 1.3.0","html_url":"https://github.com/Z3r0s/Pulse-Vault/releases/tag/v1.3.0","assets":[]}`)
	client := newClient(APIEndpoint, &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != APIEndpoint || request.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL)
		}
		if request.Header.Get("User-Agent") != userAgent || request.Header.Get("X-GitHub-Api-Version") != apiVersion {
			t.Fatal("release request is missing its documented client headers")
		}
		return response(request, http.StatusOK, body), nil
	})})

	available, err := client.Check(context.Background(), "1.2.9")
	if err != nil {
		t.Fatal(err)
	}
	if !available.Comparable || !available.UpdateAvailable || available.TagName != "v1.3.0" {
		t.Fatalf("unexpected update comparison: %+v", available)
	}
	current, err := client.Check(context.Background(), "1.3.0")
	if err != nil {
		t.Fatal(err)
	}
	if !current.Comparable || current.UpdateAvailable {
		t.Fatalf("equal version should be current: %+v", current)
	}
	development, err := client.Check(context.Background(), "dev")
	if err != nil {
		t.Fatal(err)
	}
	if development.Comparable || development.UpdateAvailable {
		t.Fatalf("development builds should remain incomparable: %+v", development)
	}
}

func TestCheckRejectsInvalidReleaseMetadata(t *testing.T) {
	cases := []string{
		`{"tag_name":"v1.2.3","draft":true,"html_url":"https://github.com/Z3r0s/Pulse-Vault/releases/tag/v1.2.3"}`,
		`{"tag_name":"v1.2.3-rc.1","prerelease":true,"html_url":"https://github.com/Z3r0s/Pulse-Vault/releases/tag/v1.2.3-rc.1"}`,
		`{"tag_name":"v1.2.3","html_url":"https://example.com/releases/tag/v1.2.3"}`,
		`{"tag_name":"latest","html_url":"https://github.com/Z3r0s/Pulse-Vault/releases/tag/latest"}`,
		`{"tag_name":"v1.2.3+","html_url":"https://github.com/Z3r0s/Pulse-Vault/releases/tag/v1.2.3+"}`,
		`{"tag_name":"v1.2.3+build!","html_url":"https://github.com/Z3r0s/Pulse-Vault/releases/tag/v1.2.3+build!"}`,
		`{"tag_name":"v1.2.3-alpha..1","html_url":"https://github.com/Z3r0s/Pulse-Vault/releases/tag/v1.2.3-alpha..1"}`,
	}
	for _, body := range cases {
		client := newClient(APIEndpoint, &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return response(request, http.StatusOK, []byte(body)), nil
		})})
		if _, err := client.Check(context.Background(), "1.0.0"); err == nil {
			t.Errorf("Check accepted invalid release metadata: %s", body)
		}
	}
}

func TestCancelledCheckDoesNotIssueRequest(t *testing.T) {
	requests := 0
	client := newClient(APIEndpoint, &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		return response(request, http.StatusOK, []byte(`{}`)), nil
	})})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.Check(ctx, "1.0.0"); err == nil {
		t.Fatal("cancelled release check unexpectedly succeeded")
	}
	if requests != 0 {
		t.Fatalf("cancelled check made %d requests", requests)
	}
}

func TestDownloadVerifiesManifestBeforePublishing(t *testing.T) {
	binary := []byte("sample GUI update")
	digest := sha256.Sum256(binary)
	hexDigest := hex.EncodeToString(digest[:])
	assetURL := "https://github.com/Z3r0s/Pulse-Vault/releases/download/v1.3.0/pulse-vault-gui-windows-amd64.exe"
	manifestURL := "https://github.com/Z3r0s/Pulse-Vault/releases/download/v1.3.0/SHA256SUMS"
	manifest := []byte(hexDigest + "  pulse-vault-gui-windows-amd64.exe\n")
	var downloaded int
	client := newClient(APIEndpoint, &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case manifestURL:
			return response(request, http.StatusOK, manifest), nil
		case assetURL:
			downloaded++
			return response(request, http.StatusOK, binary), nil
		default:
			t.Fatalf("unexpected download URL: %s", request.URL)
			return nil, nil
		}
	})})
	release := Release{
		TagName: "v1.3.0",
		HTMLURL: "https://github.com/Z3r0s/Pulse-Vault/releases/tag/v1.3.0",
		Assets: []Asset{
			{Name: "pulse-vault-gui-windows-amd64.exe", Size: int64(len(binary)), BrowserDownloadURL: assetURL, Digest: "sha256:" + hexDigest},
			{Name: "SHA256SUMS", Size: int64(len(manifest)), BrowserDownloadURL: manifestURL},
		},
	}
	destination := filepath.Join(t.TempDir(), "pulse-vault-gui-windows-amd64.exe")
	progressCalls := 0
	err := client.Download(context.Background(), release, "pulse-vault-gui-windows-amd64.exe", destination, func(done, total int64) {
		progressCalls++
		if done > total || total != int64(len(binary)) {
			t.Errorf("invalid progress: %d/%d", done, total)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if downloaded != 1 || progressCalls == 0 {
		t.Fatalf("download count=%d, progress calls=%d", downloaded, progressCalls)
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(binary) {
		t.Fatalf("downloaded bytes = %q, want %q", got, binary)
	}
}

func TestDownloadDiscardsCorruptAssetAndProtectsExistingFile(t *testing.T) {
	expectedData := []byte("original release bytes")
	corruptData := []byte("modified release bytes")
	digest := sha256.Sum256(expectedData)
	assetURL := "https://github.com/Z3r0s/Pulse-Vault/releases/download/v1.3.0/app.exe"
	manifestURL := "https://github.com/Z3r0s/Pulse-Vault/releases/download/v1.3.0/SHA256SUMS"
	manifest := []byte(hex.EncodeToString(digest[:]) + "  app.exe\n")
	requests := 0
	client := newClient(APIEndpoint, &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.String() == manifestURL {
			return response(request, http.StatusOK, manifest), nil
		}
		return response(request, http.StatusOK, corruptData), nil
	})})
	release := Release{
		TagName: "v1.3.0",
		HTMLURL: "https://github.com/Z3r0s/Pulse-Vault/releases/tag/v1.3.0",
		Assets: []Asset{
			{Name: "app.exe", Size: int64(len(corruptData)), BrowserDownloadURL: assetURL},
			{Name: "SHA256SUMS", Size: int64(len(manifest)), BrowserDownloadURL: manifestURL},
		},
	}
	dir := t.TempDir()
	destination := filepath.Join(dir, "app.exe")
	if err := client.Download(context.Background(), release, "app.exe", destination, nil); err == nil {
		t.Fatal("corrupt binary unexpectedly passed SHA-256 verification")
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("failed download left a destination file: %v", err)
	}
	parts, err := filepath.Glob(filepath.Join(dir, ".pulse-vault-update-*.part"))
	if err != nil || len(parts) != 0 {
		t.Fatalf("failed download left temporary files: %v (err=%v)", parts, err)
	}
	if err := os.WriteFile(destination, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := requests
	if err := client.Download(context.Background(), release, "app.exe", destination, nil); err == nil {
		t.Fatal("existing destination was overwritten")
	}
	if requests != before {
		t.Fatal("existing destination should be rejected before contacting GitHub")
	}
	got, err := os.ReadFile(destination)
	if err != nil || string(got) != "keep me" {
		t.Fatalf("existing destination changed: %q, err=%v", got, err)
	}
}

func TestAssetURLRequiresTheExpectedGitHubRelease(t *testing.T) {
	valid := "https://github.com/Z3r0s/Pulse-Vault/releases/download/v1.2.3/app.exe"
	if err := validateAssetURL(valid, "v1.2.3", "app.exe"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		"http://github.com/Z3r0s/Pulse-Vault/releases/download/v1.2.3/app.exe",
		"https://evil.example/Z3r0s/Pulse-Vault/releases/download/v1.2.3/app.exe",
		"https://github.com/other/repo/releases/download/v1.2.3/app.exe",
		"https://github.com/Z3r0s/Pulse-Vault/releases/download/v1.2.4/app.exe",
		"https://github.com/Z3r0s/Pulse-Vault/releases/download/v1.2.3/../other.exe",
	} {
		if err := validateAssetURL(raw, "v1.2.3", "app.exe"); err == nil {
			t.Errorf("accepted unsafe or mismatched URL %q", raw)
		}
	}
}

func TestChecksumForRequiresOneValidMatchingEntry(t *testing.T) {
	digest := strings.Repeat("a", 64)
	for _, test := range []struct {
		manifest string
		wantErr  bool
	}{
		{digest + "  app.exe\n", false},
		{digest + " *app.exe\n", false},
		{digest + "  other.exe\n", true},
		{"not-a-digest  app.exe\n", true},
		{digest + "  app.exe\n" + digest + "  app.exe\n", true},
	} {
		_, err := checksumFor([]byte(test.manifest), "app.exe")
		if (err != nil) != test.wantErr {
			t.Errorf("checksumFor(%q) err=%v, wantErr=%v", test.manifest, err, test.wantErr)
		}
	}
}

func TestTrustedRedirectHosts(t *testing.T) {
	for _, raw := range []string{
		"https://api.github.com/repos/Z3r0s/Pulse-Vault/releases/latest",
		"https://github.com/Z3r0s/Pulse-Vault/releases/download/v1.2.3/app.exe",
		"https://release-assets.githubusercontent.com/file",
		"https://objects.githubusercontent.com/github-production-release-asset/file",
	} {
		u, err := url.Parse(raw)
		if err != nil || !trustedGitHubHost(u) {
			t.Errorf("trustedGitHubHost(%q) = false", raw)
		}
	}
	for _, raw := range []string{
		"http://github.com/Z3r0s/Pulse-Vault/releases/latest",
		"https://api.github.com.evil.example/repos/Z3r0s/Pulse-Vault/releases/latest",
		"https://github.com@evil.example/Z3r0s/Pulse-Vault/releases/latest",
	} {
		u, err := url.Parse(raw)
		if err != nil || trustedGitHubHost(u) {
			t.Errorf("trustedGitHubHost(%q) = true", raw)
		}
	}
}
