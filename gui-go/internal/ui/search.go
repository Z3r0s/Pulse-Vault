package ui

import "strings"

// filterFileNames caches folded names only while a query is active and reuses
// the result slice across keystrokes, clearing stale paths as the result shrinks.
// Clearing search drops the cache so large vaults do not retain a second copy
// of their paths when it is not needed.
func filterFileNames(names, foldedNames []string, rawQuery string, filtered []string) ([]string, []string) {
	query := strings.ToLower(strings.TrimSpace(rawQuery))
	previousLength := len(filtered)
	if cap(filtered) < len(names) {
		filtered = make([]string, 0, len(names))
		previousLength = 0
	} else {
		filtered = filtered[:0]
	}

	if query == "" {
		filtered = append(filtered, names...)
		clearUnusedPaths(filtered, previousLength)
		return nil, filtered
	}

	if len(foldedNames) != len(names) {
		foldedNames = make([]string, len(names))
		for i, name := range names {
			foldedNames[i] = strings.ToLower(name)
		}
	}
	for i, folded := range foldedNames {
		if strings.Contains(folded, query) {
			filtered = append(filtered, names[i])
		}
	}
	clearUnusedPaths(filtered, previousLength)
	return foldedNames, filtered
}

func clearUnusedPaths(filtered []string, previousLength int) {
	if previousLength > len(filtered) {
		clear(filtered[len(filtered):previousLength])
	}
}
