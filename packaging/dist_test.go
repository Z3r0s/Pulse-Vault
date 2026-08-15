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
		"packaging/pypi/pyproject.toml",
		"snap/snapcraft.yaml",
		"scripts/install.sh",
		"packaging/linux/pulse-vault.desktop",
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
	if !strings.Contains(s, "Z3r0s/Pulse-Vault") {
		t.Fatal("launcher must fetch from this GitHub repo")
	}
	if strings.Contains(s, "customtkinter") {
		t.Fatal("launcher must not import the retired GUI")
	}
}
