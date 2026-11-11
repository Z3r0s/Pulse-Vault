package vault

import (
	"strings"
	"unicode"
)

// caseFoldKey returns one stable key for strings.EqualFold-equivalent paths.
// The ASCII fast path avoids constructing a rune slice or builder for the
// common case. Unicode keys use the minimum rune in each SimpleFold cycle.
func caseFoldKey(value string) string {
	ascii := true
	for i := 0; i < len(value); i++ {
		if value[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return strings.ToUpper(value)
	}

	var folded strings.Builder
	folded.Grow(len(value))
	for _, r := range value {
		minimum := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < minimum {
				minimum = next
			}
		}
		folded.WriteRune(minimum)
	}
	return folded.String()
}

func indexExistingPaths(files map[string]FileMeta) (map[string]string, map[string][]string) {
	index := make(map[string]string, len(files))
	var ambiguous map[string][]string
	for name := range files {
		key := caseFoldKey(name)
		if prior, exists := index[key]; exists && prior != name {
			if ambiguous == nil {
				ambiguous = make(map[string][]string)
			}
			ambiguous[key] = append(ambiguous[key], name)
		} else {
			index[key] = name
		}
	}
	return index, ambiguous
}

func existingPathConflict(folded, inputName string, index map[string]string, ambiguous map[string][]string) (string, bool) {
	if existing, ok := index[folded]; ok && existing != inputName {
		return existing, true
	}
	for _, existing := range ambiguous[folded] {
		if existing != inputName {
			return existing, true
		}
	}
	return "", false
}
