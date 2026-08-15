package vault

import "testing"

func TestPasswordPolicyCode(t *testing.T) {
	if PasswordPolicyCode("short") != "too_short" {
		t.Fatalf("short: %q", PasswordPolicyCode("short"))
	}
	if PasswordPolicyCode("qwerty123456789") != "common" {
		t.Fatalf("common: %q", PasswordPolicyCode("qwerty123456789"))
	}
	if PasswordPolicyCode("aaaaaaaaaaaaaa") != "repetitive" {
		t.Fatalf("repetitive: %q", PasswordPolicyCode("aaaaaaaaaaaaaa"))
	}
	if PasswordPolicyCode("abcdefghijklmn") != "low_variety" {
		t.Fatalf("low variety: %q", PasswordPolicyCode("abcdefghijklmn"))
	}
	if PasswordPolicyCode("GoVaultTestPhrase123!") != "" {
		t.Fatalf("strong password rejected: %q", PasswordPolicyCode("GoVaultTestPhrase123!"))
	}
	if msg := PasswordPolicyError("short"); msg == "" {
		t.Fatal("expected catalog message")
	}
	if PasswordPolicyError("GoVaultTestPhrase123!") != "" {
		t.Fatal("strong password should have empty error")
	}
}
