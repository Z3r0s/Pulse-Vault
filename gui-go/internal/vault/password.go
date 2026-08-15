package vault

import (
	"strings"
	"unicode"
)

// PasswordPolicyCode returns a stable weak-password reason, or "" if acceptable.
// Codes never echo password content.
func PasswordPolicyCode(password string) string {
	if len(password) < 14 {
		return "too_short"
	}
	lowered := strings.ToLower(password)
	if strings.Contains(lowered, "password") || strings.Contains(lowered, "qwerty") ||
		strings.Contains(lowered, "letmein") || strings.Contains(lowered, "123456") {
		return "common"
	}
	uniq := map[rune]struct{}{}
	for _, r := range password {
		uniq[r] = struct{}{}
	}
	if len(uniq) < 5 {
		return "repetitive"
	}
	var lower, upper, digit, other bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		default:
			other = true
		}
	}
	cats := 0
	for _, ok := range []bool{lower, upper, digit, other} {
		if ok {
			cats++
		}
	}
	if cats < 3 && len(password) < 20 {
		return "low_variety"
	}
	return ""
}

// PasswordPolicyError is a fixed catalog message for PasswordPolicyCode.
func PasswordPolicyError(password string) string {
	switch PasswordPolicyCode(password) {
	case "too_short":
		return "Use at least 14 characters."
	case "common":
		return "Avoid common words, keyboard patterns, and obvious number runs."
	case "repetitive":
		return "Use a less repetitive password."
	case "low_variety":
		return "Use more variety, or make it at least 20 characters."
	default:
		return ""
	}
}
