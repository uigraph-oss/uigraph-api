package authz

import "strings"

// claimMatches reports whether the claim at key equals value.
// It handles three formats:
//   - plain string:  "uigraph_role": "admin"
//   - string array:  "groups": ["uigraph-admin", "eng"]
//   - dot-notation:  "user.role": "editor"
func claimMatches(claims map[string]any, key, value string) bool {
	raw := resolveDotNotation(claims, key)
	switch v := raw.(type) {
	case string:
		return v == value
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s == value {
				return true
			}
		}
	}
	return false
}

// resolveDotNotation traverses nested maps using dot-separated key segments.
func resolveDotNotation(claims map[string]any, key string) any {
	parts := strings.SplitN(key, ".", 2)
	val, ok := claims[parts[0]]
	if !ok {
		return nil
	}
	if len(parts) == 1 {
		return val
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return nil
	}
	return resolveDotNotation(nested, parts[1])
}
