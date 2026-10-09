package processor

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
)

// OTLP/JSON encodes trace and span IDs as hex strings (per the OTLP spec),
// but protojson expects base64 for bytes fields. Without this conversion every
// ID decodes to garbage, and every JSON-speaking SDK would look broken.
var idFields = map[string]bool{"traceId": true, "spanId": true, "parentSpanId": true}

func normalizeOTLPJSON(body []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	fixIDs(v)
	return json.Marshal(v)
}

func fixIDs(v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if s, ok := child.(string); ok && idFields[k] {
				if b, err := hex.DecodeString(s); err == nil {
					x[k] = base64.StdEncoding.EncodeToString(b)
				}
				continue
			}
			fixIDs(child)
		}
	case []any:
		for _, child := range x {
			fixIDs(child)
		}
	}
}
