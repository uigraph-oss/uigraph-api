package catalog

// schemaToExample generates an example payload from a resolved JSON Schema.
func schemaToExample(schema interface{}, depth int) interface{} {
	if schema == nil || depth > 6 {
		return nil
	}

	m, ok := schema.(map[string]interface{})
	if !ok {
		return schema
	}

	if example, ok := m["example"]; ok {
		return example
	}

	if enum, ok := m["enum"].([]interface{}); ok && len(enum) > 0 {
		return enum[0]
	}

	for _, combiner := range []string{"allOf", "anyOf", "oneOf"} {
		list, ok := m[combiner].([]interface{})
		if !ok || len(list) == 0 {
			continue
		}
		if combiner == "allOf" {
			merged := make(map[string]interface{})
			for _, item := range list {
				sub, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				for k, v := range sub {
					merged[k] = v
				}
			}
			return schemaToExample(merged, depth+1)
		}
		return schemaToExample(list[0], depth+1)
	}

	typ, _ := m["type"].(string)

	if typ == "object" || m["properties"] != nil {
		props, _ := m["properties"].(map[string]interface{})
		result := make(map[string]interface{})
		for key, propSchema := range props {
			result[key] = schemaToExample(propSchema, depth+1)
		}
		return result
	}

	if typ == "array" || m["items"] != nil {
		items := m["items"]
		if items == nil {
			items = map[string]interface{}{}
		}
		return []interface{}{schemaToExample(items, depth+1)}
	}

	format, _ := m["format"].(string)
	switch typ {
	case "string":
		switch format {
		case "email":
			return "user@example.com"
		case "date-time":
			return "2024-01-01T00:00:00Z"
		case "date":
			return "2024-01-01"
		case "password":
			return "********"
		case "uuid":
			return "00000000-0000-0000-0000-000000000000"
		case "uri":
			return "https://example.com"
		default:
			return "string"
		}
	case "integer", "number":
		return 0
	case "boolean":
		return true
	default:
		return nil
	}
}
