package vault

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Vault tests use the explicitly test-only low-cost profile to avoid running
	// the production KDF for every fixture. Production binaries never set this.
	_ = os.Setenv("PULSEVAULT_TEST_FAST_KDF", "1")
	_ = os.Unsetenv("PULSEVAULT_SCRYPT_PROFILE")
	os.Exit(m.Run())
}
