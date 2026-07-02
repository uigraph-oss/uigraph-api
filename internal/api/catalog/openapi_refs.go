package catalog

import "strings"

// resolveRefsInDoc walks v and inlines JSON Pointer $ref values against doc.
func resolveRefsInDoc(doc map[string]interface{}, v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		if ref, ok := val["$ref"].(string); ok {
			if resolved := resolveJSONPointer(doc, ref); resolved != nil {
				return resolveRefsInDoc(doc, resolved)
			}
			return val
		}
		out := make(map[string]interface{}, len(val))
		for k, child := range val {
			out[k] = resolveRefsInDoc(doc, child)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, child := range val {
			out[i] = resolveRefsInDoc(doc, child)
		}
		return out
	default:
		return v
	}
}

func resolveJSONPointer(doc map[string]interface{}, ref string) interface{} {
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	parts := strings.Split(ref[2:], "/")
	var current interface{} = doc
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}
