package catalog

import "encoding/json"

// normalizeStoredJSON unwraps JSON payloads that were accidentally stored as a
// JSON string (double-encoded) instead of a raw object or array.
func normalizeStoredJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err != nil {
		return raw
	}
	if !json.Valid([]byte(asString)) {
		return raw
	}
	return json.RawMessage(asString)
}

// seedExampleSamplesJSON wraps a parser-produced payload as a one-element sample array.
func seedExampleSamplesJSON(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return json.RawMessage("[]")
	}
	switch string(payload) {
	case "{}", "null", "[]":
		return json.RawMessage("[]")
	}
	var v interface{}
	if err := json.Unmarshal(payload, &v); err != nil {
		return json.RawMessage("[]")
	}
	out, err := json.Marshal([]interface{}{v})
	if err != nil {
		return json.RawMessage("[]")
	}
	return out
}
