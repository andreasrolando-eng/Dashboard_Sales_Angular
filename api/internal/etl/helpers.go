package etl

import (
	"encoding/json"
	"strconv"
)

func itoa(n int) string { return strconv.Itoa(n) }

// rawJSONArray marshals a slice of raw JSON values back to a jsonb-ready
// byte slice, defaulting to an empty array (matching the original's
// `?? []` fallback) rather than SQL NULL.
func rawJSONArray(items []json.RawMessage) []byte {
	if items == nil {
		return []byte("[]")
	}
	b, err := json.Marshal(items)
	if err != nil {
		return []byte("[]")
	}
	return b
}

// mustMarshal re-serializes an already-decoded ESB record for the `raw`
// jsonb column -- this can only fail if the record contains something
// json.Marshal can't handle, which esbSaleRecord never does.
func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
