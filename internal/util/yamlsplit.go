package util

import "strings"

// SplitYAML splits a YAML multi-document string into individual docs, trimming empties.
func SplitYAML(s string) []string {
	parts := []string{}
	for _, p := range strings.Split(s, "\n---") {
		t := strings.TrimSpace(p)
		if t == "" { continue }
		// Ensure trailing newline for deterministic encoding later
		if !strings.HasSuffix(t, "\n") { t += "\n" }
		parts = append(parts, t)
	}
	return parts
}