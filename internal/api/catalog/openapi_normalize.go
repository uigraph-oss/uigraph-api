package catalog

import "unicode"

// normalizeRequestBodyForStorage converts an OpenAPI requestBody (or plain JSON
// Schema) into an example payload for storage.
func normalizeRequestBodyForStorage(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	m, ok := v.(map[string]interface{})
	if !ok {
		return schemaToExample(v, 0)
	}

	if content, ok := m["content"].(map[string]interface{}); ok {
		if example := pickContentExample(content); example != nil {
			return example
		}
		if schema := pickContentSchema(content); schema != nil {
			return schemaToExample(schema, 0)
		}
		return nil
	}

	return schemaToExample(v, 0)
}

// normalizeResponsesForStorage converts OpenAPI responses into example payload(s).
// A single status code yields the example object directly; multiple statuses
// are stored as a map keyed by status code.
func normalizeResponsesForStorage(v interface{}) interface{} {
	m, ok := v.(map[string]interface{})
	if !ok {
		return map[string]interface{}{}
	}

	examplesByStatus := make(map[string]interface{})
	for status, entry := range m {
		if !isHTTPStatusKey(status) {
			continue
		}
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}

		var example interface{}
		if content, ok := entryMap["content"].(map[string]interface{}); ok {
			example = pickContentExample(content)
			if example == nil {
				if schema := pickContentSchema(content); schema != nil {
					example = schemaToExample(schema, 0)
				}
			}
		} else if ex, ok := entryMap["example"]; ok {
			example = ex
		} else if schema, ok := entryMap["schema"]; ok {
			example = schemaToExample(schema, 0)
		}

		if example != nil {
			examplesByStatus[status] = example
		}
	}

	if len(examplesByStatus) == 0 {
		return map[string]interface{}{}
	}
	if len(examplesByStatus) == 1 {
		for _, ex := range examplesByStatus {
			return ex
		}
	}
	return examplesByStatus
}

func pickContentSchema(content map[string]interface{}) interface{} {
	if jsonContent, ok := content["application/json"].(map[string]interface{}); ok {
		if schema, ok := jsonContent["schema"]; ok {
			return schema
		}
	}
	for _, ct := range content {
		ctMap, ok := ct.(map[string]interface{})
		if !ok {
			continue
		}
		if schema, ok := ctMap["schema"]; ok {
			return schema
		}
	}
	return nil
}

func pickContentExample(content map[string]interface{}) interface{} {
	if jsonContent, ok := content["application/json"].(map[string]interface{}); ok {
		if example, ok := jsonContent["example"]; ok {
			return example
		}
		if examples, ok := jsonContent["examples"].(map[string]interface{}); ok {
			for _, ex := range examples {
				exMap, ok := ex.(map[string]interface{})
				if !ok {
					continue
				}
				if value, ok := exMap["value"]; ok {
					return value
				}
			}
		}
	}
	return nil
}

func isHTTPStatusKey(s string) bool {
	if s == "default" {
		return true
	}
	if len(s) != 3 {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
