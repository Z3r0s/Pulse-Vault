package packaging_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Dir(filepath.Dir(file))
}

func TestDistributionFilesPresent(t *testing.T) {
	root := repoRoot(t)
	need := []string{
		"docs/WHY_GO.md",
		"docs/DISTRIBUTE.md",
		"docs/ROADMAP.md",
		"docs/assets/why-go-walltime.svg",
		"docs/assets/why-go-speedup.svg",
		"docs/assets/why-go-tcb.svg",
		"docs/assets/why-go-supply-chain.svg",
		"packaging/pypi/pyproject.toml",
		"packaging/pypi/src/pulse_vault/launcher.py",
		"packaging/pypi/tests/test_launcher.py",
		"snap/snapcraft.yaml",
		"scripts/install.sh",
		"scripts/install.ps1",
		"scripts/install.cmd",
		"cli.ps1",
		"cli.sh",
		"docs/TRUST.md",
		"INSTALL.md",
		"packaging/linux/pulse-vault.desktop",
		"packaging/linux/io.github.z3r0s.PulseVault.metainfo.xml",
	}
	for _, rel := range need {
		p := filepath.Join(root, filepath.FromSlash(rel))
		st, err := os.Stat(p)
		if err != nil || st.Size() == 0 {
			t.Errorf("missing %s: %v", rel, err)
		}
	}
}

func TestBrandingMentionsDNSPulse(t *testing.T) {
	root := repoRoot(t)
	files := []string{
		"docs/WHY_GO.md",
		"docs/DISTRIBUTE.md",
		"docs/ROADMAP.md",
		"docs/TRUST.md",
		"gui-go/app.manifest",
		"gui-go/scripts/sign-windows.ps1",
		"packaging/pypi/pyproject.toml",
		"snap/snapcraft.yaml",
		"scripts/install.sh",
		"cli.ps1",
		"cli.sh",
	}
	for _, rel := range files {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		body := string(b)
		if !strings.Contains(body, "dnspulse.org") {
			t.Errorf("%s missing dnspulse.org", rel)
		}
	}
}

func TestTrustDocDoesNotPromiseZeroDetections(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "docs", "TRUST.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, n := range []string{
		"dnspulse.org",
		"Authenticode",
		"wdsi/filesubmission",
		"sign-windows.ps1",
		"nobody can make Windows promise",
	} {
		if !strings.Contains(s, n) {
			t.Errorf("TRUST.md missing %q", n)
		}
	}
}

func TestSnapIsGoCLINotPython(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "snap", "snapcraft.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "./gui-go/cmd/pulse-vault") {
		t.Fatal("snap must build the Go CLI")
	}
	if strings.Contains(s, "legacy/python") {
		t.Fatal("snap must not package the retired Python tree")
	}
}

func TestInstallDocsLeadWithDownloadNotBuild(t *testing.T) {
	root := repoRoot(t)
	install, err := os.ReadFile(filepath.Join(root, "INSTALL.md"))
	if err != nil {
		t.Fatal(err)
	}
	body := string(install)
	need := []string{
		"pulse-vault-gui-windows-amd64.exe",
		"scripts/install.cmd",
		"install.ps1",
		"install.sh",
		".\\cli.ps1",
		"./cli.sh",
		"go install github.com/Z3r0s/Pulse-Vault/gui-go/cmd/pulse-vault@main",
		"--from-source",
		"-FromSource",
		"-WithGui",
		"pip install -U pulse-vault",
		"Uninstall",
		"dnspulse.org",
	}
	for _, n := range need {
		if !strings.Contains(body, n) {
			t.Errorf("INSTALL.md missing %q", n)
		}
	}
	if !strings.Contains(body, "GitHub Releases") {
		t.Fatal("INSTALL.md should mention GitHub Releases")
	}
	if strings.Contains(body, "CGO_ENABLED=0") {
		t.Fatal("INSTALL.md must not tell people to set CGO_ENABLED=0 for the CLI")
	}
}

func TestInstallScriptsCoverGuiAndSourceFallback(t *testing.T) {
	root := repoRoot(t)
	ps1, err := os.ReadFile(filepath.Join(root, "scripts", "install.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	sh, err := os.ReadFile(filepath.Join(root, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	ps := string(ps1)
	unix := string(sh)
	for _, n := range []string{"WithGui", "FromSource", "Start Menu", "SHA256", "dnspulse.org"} {
		if !strings.Contains(ps, n) {
			t.Errorf("install.ps1 missing %q", n)
		}
	}
	for _, n := range []string{"--gui", "--from-source", "shasum", "sha256sum", "dnspulse.org"} {
		if !strings.Contains(unix, n) {
			t.Errorf("install.sh missing %q", n)
		}
	}
}

func TestPyPIIsLauncherNotLegacyVault(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, "packaging", "pypi", "src", "pulse_vault", "launcher.py"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "SHA256") && !strings.Contains(s, "sha256") {
		t.Fatal("launcher must verify SHA-256")
	}
	if !strings.Contains(s, "api.github.com/repos") && !strings.Contains(s, "Z3r0s/Pulse-Vault") {
		t.Fatal("launcher must fetch from this GitHub repo")
	}
	if strings.Contains(s, "customtkinter") {
		t.Fatal("launcher must not import the retired GUI")
	}
}
