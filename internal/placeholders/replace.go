package placeholders

import (
	"regexp"
	"sort"
	"strings"
)

// ApplySentinelToSchema replaces template sentinels with ${schema...} refs.
func ApplySentinelToSchema(in string) string {
	// Longest-key-first to avoid partial overlaps.
	keys := make([]string, 0, len(SentinelToSchema))
	for k := range SentinelToSchema {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	out := in
	for _, k := range keys {
		out = strings.ReplaceAll(out, k, SentinelToSchema[k])
	}
	return out
}

// ApplySchemaDefaults replaces ${schema...} refs with concrete defaults.
// Only applied when emitting schema defaults or when materializing examples.
func ApplySchemaDefaults(in string) string {
	// Use regex to catch ${...} tokens even if new ones are added later.
	re := regexp.MustCompile(`\$\{schema\.spec[^}]*\}`)
	return re.ReplaceAllStringFunc(in, func(tok string) string {
		if v, ok := SchemaDefaults[tok]; ok {
			return v
		}
		// Unknown token passes through unchanged.
		return tok
	})
}

// ReplaceAll does both passes in correct order.
func ReplaceAll(in string, withDefaults bool) string {
	out := ApplySentinelToSchema(in)
	if withDefaults {
		out = ApplySchemaDefaults(out)
	}
	return out
}
