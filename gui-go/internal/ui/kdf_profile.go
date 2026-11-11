package ui

import (
	"fmt"
	"strings"

	pulsecrypto "github.com/Z3r0s/Pulse-Vault/gui-go/crypto"
)

const kdfMiB = uint64(1 << 20)
const kdfGiB = uint64(1 << 30)

func kdfProfileOption(profile string) string {
	definition, ok := pulsecrypto.Profiles[profile]
	if !ok || (profile != "standard" && profile != "hardened") {
		return ""
	}
	bytes := uint64(128) * uint64(definition.N) * uint64(definition.R) * uint64(definition.P)
	memory := ""
	switch {
	case bytes%kdfGiB == 0:
		memory = fmt.Sprintf("%d GiB", bytes/kdfGiB)
	case bytes%kdfMiB == 0:
		memory = fmt.Sprintf("%d MiB", bytes/kdfMiB)
	default:
		memory = fmt.Sprintf("%.1f MiB", float64(bytes)/float64(kdfMiB))
	}
	return strings.ToUpper(profile[:1]) + profile[1:] + " · " + memory
}

func kdfProfileFromOption(option string) string {
	for _, profile := range []string{"standard", "hardened"} {
		if option == kdfProfileOption(profile) {
			return profile
		}
	}
	return ""
}

func keepCurrentKDFOption(profile string) string {
	if profile == "" {
		profile = "custom"
	}
	return "Keep current (" + profile + ")"
}

func kdfProfileExplanation(profile string) string {
	switch profile {
	case "standard":
		return "Standard uses 32 MiB per key derivation. It opens faster and suits everyday use."
	case "hardened":
		return "Hardened uses 1 GiB per key derivation. It raises the cost of offline password guesses, but takes more memory and time to create, unlock, and rotate."
	default:
		return "Keep the current KDF settings. Anyone holding a copy of the vault can test password guesses offline, so use a long, unique passphrase."
	}
}
