package vault

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	crypto "github.com/Z3r0s/Pulse-Vault/gui-go/crypto"
)

// compareOps is the shared work list for Go vs archived-Python timings.
// Values are best-of-N wall times in milliseconds.
type compareOps struct {
	StreamEncrypt4MiBms             float64 `json:"stream_encrypt_4mib_ms"`
	StreamEncrypt4MiBIncompressible float64 `json:"stream_encrypt_4mib_incompressible_ms"`
	StreamDecrypt4MiBms             float64 `json:"stream_decrypt_4mib_ms"`
	VaultAdd2MiBms                  float64 `json:"vault_add_2mib_ms"`
	VaultExtract2MiBms              float64 `json:"vault_extract_2mib_ms"`
	Parallel4x1MiBEncryptms         float64 `json:"parallel_4x_1mib_encrypt_ms"`
}

type compareReport struct {
	Impl string     `json:"impl"`
	Ops  compareOps `json:"ops"`
}

func sha256Expand(seed string, n int) []byte {
	block := []byte(seed)
	out := make([]byte, 0, n+32)
	for len(out) < n {
		sum := sha256.Sum256(block)
		block = sum[:]
		out = append(out, block...)
	}
	return out[:n]
}

func bestOf(n int, fn func()) float64 {
	var best float64
	for i := 0; i < n; i++ {
		t0 := time.Now()
		fn()
		ms := float64(time.Since(t0).Microseconds()) / 1000.0
		if i == 0 || ms < best {
			best = ms
		}
	}
	return best
}

func timeGoOps(t *testing.T) compareOps {
	t.Helper()
	chunk := []byte("pulse-vault-benchmark-data-")
	payload4 := bytes.Repeat(chunk, (4<<20)/len(chunk)+1)[:4<<20]
	payload2 := payload4[:2<<20]
	payload1 := payload4[:1<<20]
	incomp4 := sha256Expand("seed-pulse-vault-compare-v1", 4<<20)
	salt := bytes.Repeat([]byte{0x11}, crypto.SaltSize)
	prof := crypto.Profiles["fast"]
	key, err := crypto.DeriveKeyScrypt("GoVsPythonCompare!!", salt, prof.N, prof.R, prof.P)
	if err != nil {
		t.Fatal(err)
	}

	var enc bytes.Buffer
	if err := crypto.EncryptStreamV5(key, bytes.NewReader(payload4), &enc, true); err != nil {
		t.Fatal(err)
	}
	encBytes := append([]byte(nil), enc.Bytes()...)

	ops := compareOps{
		StreamEncrypt4MiBms: bestOf(3, func() {
			var out bytes.Buffer
			if err := crypto.EncryptStreamV5(key, bytes.NewReader(payload4), &out, true); err != nil {
				t.Fatal(err)
			}
		}),
		StreamEncrypt4MiBIncompressible: bestOf(3, func() {
			var out bytes.Buffer
			if err := crypto.EncryptStreamV5(key, bytes.NewReader(incomp4), &out, true); err != nil {
				t.Fatal(err)
			}
		}),
		StreamDecrypt4MiBms: bestOf(3, func() {
			var out bytes.Buffer
			if err := crypto.DecryptStreamV5(key, bytes.NewReader(encBytes), &out); err != nil {
				t.Fatal(err)
			}
		}),
		Parallel4x1MiBEncryptms: bestOf(3, func() {
			var wg sync.WaitGroup
			errCh := make(chan error, 4)
			for i := 0; i < 4; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					var out bytes.Buffer
					if err := crypto.EncryptStreamV5(key, bytes.NewReader(payload1), &out, true); err != nil {
						errCh <- err
					}
				}()
			}
			wg.Wait()
			select {
			case err := <-errCh:
				t.Fatal(err)
			default:
			}
		}),
	}

	dir := t.TempDir()
	vp := filepath.Join(dir, "bench.pulsevault")
	src := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(src, payload2, 0o600); err != nil {
		t.Fatal(err)
	}
	v := New(vp)
	if err := v.Create("GoVsPythonCompare!!", "fast"); err != nil {
		t.Fatal(err)
	}
	ops.VaultAdd2MiBms = bestOf(3, func() {
		if err := v.AddFile(src, true); err != nil {
			t.Fatal(err)
		}
	})
	outDir := filepath.Join(dir, "out")
	ops.VaultExtract2MiBms = bestOf(3, func() {
		if _, err := v.ExtractFile("blob.bin", outDir, true); err != nil {
			t.Fatal(err)
		}
	})
	return ops
}

func timePythonOps(t *testing.T) (compareOps, bool) {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	// gui-go/internal/vault → repo root → legacy/python/bench_lifecycle.py
	script := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "legacy", "python", "bench_lifecycle.py")
	if _, err := os.Stat(script); err != nil {
		t.Log("python bench script missing:", err)
		return compareOps{}, false
	}
	cmd := exec.Command("python", script)
	cmd.Env = append(os.Environ(), "PULSEVAULT_SCRYPT_PROFILE=fast", "PULSEVAULT_TEST_FAST_KDF=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("python bench skipped (%v):\n%s", err, out)
		return compareOps{}, false
	}
	var rep compareReport
	if err := json.Unmarshal(out, &rep); err != nil {
		t.Logf("python bench JSON parse failed (%v):\n%s", err, out)
		return compareOps{}, false
	}
	return rep.Ops, true
}

func pctFaster(pythonMs, goMs float64) float64 {
	if pythonMs <= 0 || goMs <= 0 {
		return 0
	}
	return (pythonMs - goMs) / pythonMs * 100
}

func speedup(pythonMs, goMs float64) float64 {
	if goMs <= 0 {
		return 0
	}
	return pythonMs / goMs
}

// times Go vs leftover Python on the same work. skips if python isn't there.
func TestCompareGoVsArchivedPython(t *testing.T) {
	goOps := timeGoOps(t)
	pyOps, pyOK := timePythonOps(t)

	t.Logf("Go  encrypt 4MiB:     %7.2f ms", goOps.StreamEncrypt4MiBms)
	t.Logf("Go  encrypt 4MiB (incompressible): %7.2f ms", goOps.StreamEncrypt4MiBIncompressible)
	t.Logf("Go  decrypt 4MiB:     %7.2f ms", goOps.StreamDecrypt4MiBms)
	t.Logf("Go  add 2MiB:         %7.2f ms", goOps.VaultAdd2MiBms)
	t.Logf("Go  extract 2MiB:     %7.2f ms", goOps.VaultExtract2MiBms)
	t.Logf("Go  4×1MiB parallel:  %7.2f ms", goOps.Parallel4x1MiBEncryptms)

	if goOps.StreamEncrypt4MiBms <= 0 || goOps.VaultAdd2MiBms <= 0 {
		t.Fatal("Go timings must be positive (real vault/crypto path did not run)")
	}

	if !pyOK {
		t.Skip("archived Python bench not runnable here (python + cryptography required)")
	}

	t.Logf("Py  encrypt 4MiB:     %7.2f ms", pyOps.StreamEncrypt4MiBms)
	t.Logf("Py  encrypt 4MiB (incompressible): %7.2f ms", pyOps.StreamEncrypt4MiBIncompressible)
	t.Logf("Py  decrypt 4MiB:     %7.2f ms", pyOps.StreamDecrypt4MiBms)
	t.Logf("Py  add 2MiB:         %7.2f ms", pyOps.VaultAdd2MiBms)
	t.Logf("Py  extract 2MiB:     %7.2f ms", pyOps.VaultExtract2MiBms)
	t.Logf("Py  4×1MiB sequential:%7.2f ms", pyOps.Parallel4x1MiBEncryptms)

	rows := []struct {
		name string
		py   float64
		go_  float64
	}{
		{"stream_encrypt_4mib", pyOps.StreamEncrypt4MiBms, goOps.StreamEncrypt4MiBms},
		{"stream_encrypt_4mib_incompressible", pyOps.StreamEncrypt4MiBIncompressible, goOps.StreamEncrypt4MiBIncompressible},
		{"stream_decrypt_4mib", pyOps.StreamDecrypt4MiBms, goOps.StreamDecrypt4MiBms},
		{"vault_add_2mib", pyOps.VaultAdd2MiBms, goOps.VaultAdd2MiBms},
		{"vault_extract_2mib", pyOps.VaultExtract2MiBms, goOps.VaultExtract2MiBms},
		{"parallel_4x_1mib_encrypt", pyOps.Parallel4x1MiBEncryptms, goOps.Parallel4x1MiBEncryptms},
	}
	for _, row := range rows {
		if row.py <= 0 || row.go_ <= 0 {
			t.Fatalf("%s produced non-positive timing py=%.3f go=%.3f", row.name, row.py, row.go_)
		}
		t.Logf("%s: python %.2f ms  go %.2f ms  →  %.0f%% faster (%.2fx)",
			row.name, row.py, row.go_, pctFaster(row.py, row.go_), speedup(row.py, row.go_))
	}

	// numbers are whatever the machine did. don't hardcode a speedup.
}
