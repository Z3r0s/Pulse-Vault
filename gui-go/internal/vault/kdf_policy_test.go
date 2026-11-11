package vault

import (
	"testing"

	"github.com/Z3r0s/Pulse-Vault/gui-go/internal/crypto"
)

func TestValidateKDFRecordOnlyAcceptsSupportedProfiles(t *testing.T) {
	for _, test := range []struct {
		name    string
		record  KDFRecord
		want    string
		wantErr bool
	}{
		{name: "standard", record: KDFRecord{Algorithm: "scrypt", Profile: "standard", N: 1 << 15, R: 8, P: 1}, want: "standard"},
		{name: "standard without label", record: KDFRecord{Algorithm: "scrypt", N: 1 << 15, R: 8, P: 1}, want: "standard"},
		{name: "hardened", record: KDFRecord{Algorithm: "scrypt", Profile: "hardened", N: 1 << 20, R: 8, P: 1}, want: "hardened"},
		{name: "custom low cost", record: KDFRecord{Algorithm: "scrypt", Profile: "standard", N: 1 << 10, R: 8, P: 1}, wantErr: true},
		{name: "mislabeled cost", record: KDFRecord{Algorithm: "scrypt", Profile: "hardened", N: 1 << 15, R: 8, P: 1}, wantErr: true},
		{name: "unsupported algorithm", record: KDFRecord{Algorithm: "pbkdf2", Profile: "standard", N: 1 << 15, R: 8, P: 1}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := validateKDFRecord(test.record)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateKDFRecord error = %v, wantErr %v", err, test.wantErr)
			}
			if err == nil && got != test.want {
				t.Fatalf("profile = %q, want %q", got, test.want)
			}
		})
	}
}

func TestFastKDFRecordRequiresExplicitTestGate(t *testing.T) {
	record := KDFRecord{Algorithm: "scrypt", Profile: "fast", N: crypto.Profiles["fast"].N, R: 8, P: 1}
	t.Setenv("PULSEVAULT_TEST_FAST_KDF", "")
	if _, err := validateKDFRecord(record); err == nil {
		t.Fatal("test-only fast profile was accepted without its explicit test gate")
	}
	t.Setenv("PULSEVAULT_TEST_FAST_KDF", "1")
	if profile, err := validateKDFRecord(record); err != nil || profile != "fast" {
		t.Fatalf("test-only profile validation = %q, %v", profile, err)
	}
}
