package orchestrator

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Canonical encodes v as canonical JSON: object keys sorted, no insignificant
// whitespace, no HTML escaping, numbers kept as written. Encoding the same
// value always yields the same bytes, also after a database round trip.
func Canonical(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var generic any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&generic); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(generic); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// SpecHash is "sha256:" + hex(sha256(canonical JSON of {specification, tasks})).
func SpecHash(spec Specification, tasks []TaskSpec) (string, error) {
	b, err := Canonical(map[string]any{"specification": spec, "tasks": tasks})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// DecodeJSON decodes data into v keeping numbers as json.Number.
func DecodeJSON(data string, v any) error {
	dec := json.NewDecoder(bytes.NewReader([]byte(data)))
	dec.UseNumber()
	return dec.Decode(v)
}

// MustJSON encodes v (canonical) or returns "null".
func MustJSON(v any) string {
	b, err := Canonical(v)
	if err != nil {
		return "null"
	}
	return string(b)
}
