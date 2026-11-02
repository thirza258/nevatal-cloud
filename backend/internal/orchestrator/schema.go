package orchestrator

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

// ValidateInput checks value against the JSON Schema subset the registry
// uses: type (object, string, integer, number, boolean, array), properties,
// required, additionalProperties (false), enum, pattern, minLength,
// maxLength, minimum, maximum, items, maxItems and format "uuid". It returns
// the first problem as "field: message".
func ValidateInput(schema map[string]any, value any) error {
	return validateValue(schema, value, "")
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func fieldName(path string) string {
	if path == "" {
		return "input"
	}
	return path
}

func validateValue(schema map[string]any, value any, path string) error {
	if schema == nil {
		return nil
	}
	typ, _ := schema["type"].(string)
	switch typ {
	case "object":
		obj, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: must be an object", fieldName(path))
		}
		props, _ := schema["properties"].(map[string]any)
		for _, req := range asStrings(schema["required"]) {
			if v, present := obj[req]; !present || v == nil {
				return fmt.Errorf("%s: is required", joinPath(path, req))
			}
		}
		if additional, ok := schema["additionalProperties"].(bool); ok && !additional {
			keys := make([]string, 0, len(obj))
			for k := range obj {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if _, known := props[k]; !known {
					return fmt.Errorf("%s: is not allowed", joinPath(path, k))
				}
			}
		}
		keys := make([]string, 0, len(props))
		for k := range props {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v, present := obj[k]
			if !present || v == nil {
				continue
			}
			sub, _ := props[k].(map[string]any)
			if err := validateValue(sub, v, joinPath(path, k)); err != nil {
				return err
			}
		}
	case "string":
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s: must be a string", fieldName(path))
		}
		if n, ok := number(schema["minLength"]); ok && float64(len(s)) < n {
			return fmt.Errorf("%s: must be at least %d characters", fieldName(path), int(n))
		}
		if n, ok := number(schema["maxLength"]); ok && float64(len(s)) > n {
			return fmt.Errorf("%s: must be at most %d characters", fieldName(path), int(n))
		}
		if p, ok := schema["pattern"].(string); ok && p != "" {
			if re, err := regexp.Compile(p); err == nil && !re.MatchString(s) {
				return fmt.Errorf("%s: has an invalid format", fieldName(path))
			}
		}
		if f, _ := schema["format"].(string); f == "uuid" && !uuidPattern.MatchString(s) {
			return fmt.Errorf("%s: must be a UUID", fieldName(path))
		}
		if err := checkEnum(schema, s, path); err != nil {
			return err
		}
	case "integer", "number":
		n, ok := number(value)
		if !ok {
			return fmt.Errorf("%s: must be a number", fieldName(path))
		}
		if typ == "integer" && n != math.Trunc(n) {
			return fmt.Errorf("%s: must be an integer", fieldName(path))
		}
		if min, ok := number(schema["minimum"]); ok && n < min {
			return fmt.Errorf("%s: must be at least %v", fieldName(path), min)
		}
		if max, ok := number(schema["maximum"]); ok && n > max {
			return fmt.Errorf("%s: must be at most %v", fieldName(path), max)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: must be a boolean", fieldName(path))
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: must be an array", fieldName(path))
		}
		if n, ok := number(schema["maxItems"]); ok && float64(len(items)) > n {
			return fmt.Errorf("%s: must have at most %d items", fieldName(path), int(n))
		}
		sub, _ := schema["items"].(map[string]any)
		for i, item := range items {
			if err := validateValue(sub, item, fmt.Sprintf("%s[%d]", fieldName(path), i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkEnum(schema map[string]any, s, path string) error {
	enum := asStrings(schema["enum"])
	if len(enum) == 0 {
		return nil
	}
	for _, e := range enum {
		if e == s {
			return nil
		}
	}
	return fmt.Errorf("%s: must be one of %s", fieldName(path), strings.Join(enum, ", "))
}

func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func asStrings(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
