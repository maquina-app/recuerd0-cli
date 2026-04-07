package commands

// ValidCategories is the locked enum of memory categories supported by the
// recuerd0 server. Keep in sync with Memory::CATEGORIES on the Rails side.
var ValidCategories = []string{"decision", "discovery", "preference", "general"}

// IsValidCategory reports whether s is one of the supported category values.
// Empty string returns true (means "let the server default it").
func IsValidCategory(s string) bool {
	if s == "" {
		return true
	}
	for _, c := range ValidCategories {
		if c == s {
			return true
		}
	}
	return false
}
