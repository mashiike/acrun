package acrun

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

type marshalJSONOptions struct {
	hooks                 []func(string, string, any) (string, any, error)
	ignoreLowerCamelPaths []string
}

func marshalJSON(v interface{}, optFns ...func(*marshalJSONOptions)) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	var opts marshalJSONOptions
	for _, fn := range optFns {
		fn(&opts)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	fn := func(path, key string, value any) (string, any, error) {
		ignored := false
		for _, p := range opts.ignoreLowerCamelPaths {
			if matchJSONKey(path, p) {
				ignored = true
				break
			}
		}
		var err error
		newKey, newValue := key, value
		if !ignored {
			newKey, newValue, err = toLowerCamelCase(path, key, value)
			if err != nil {
				return "", nil, err
			}
		}
		for _, hook := range opts.hooks {
			newKey, newValue, err = hook(path, newKey, newValue)
			if err != nil {
				return "", nil, err
			}
		}
		return newKey, newValue, nil
	}
	switch b[0] {
	case '{':
		m := map[string]any{}
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		if err := walkMap(m, "$", fn); err != nil {
			return nil, err
		}
		return json.Marshal(m)
	case '[':
		a := []any{}
		if err := json.Unmarshal(b, &a); err != nil {
			return nil, err
		}
		if err := walkArray(a, "$", fn); err != nil {
			return nil, err
		}
		return json.Marshal(a)
	default:
		return b, nil
	}
}

type unmarshalJSONOptions struct {
	hooks                 []func(string, string, any) (string, any, error)
	ignoreUpperCamelPaths []string
	strict                bool
}

func unmarshalJSON(data []byte, v interface{}, optFns ...func(*unmarshalJSONOptions)) error {
	if v == nil {
		return nil
	}
	var opts unmarshalJSONOptions
	for _, fn := range optFns {
		fn(&opts)
	}
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	fn := func(path, key string, value any) (string, any, error) {
		ignored := false
		for _, p := range opts.ignoreUpperCamelPaths {
			if matchJSONKey(path, p) {
				ignored = true
				break
			}
		}
		var err error
		newKey, newValue := key, value
		if !ignored {
			newKey, newValue, err = toUpperCamelCase(path, key, value)
			if err != nil {
				return "", nil, err
			}
		}
		for _, hook := range opts.hooks {
			newKey, newValue, err = hook(path, newKey, newValue)
			if err != nil {
				return "", nil, err
			}
		}
		return newKey, newValue, nil
	}
	switch raw := raw.(type) {
	case map[string]interface{}:
		if err := walkMap(raw, "$", fn); err != nil {
			return err
		}
	case []interface{}:
		if err := walkArray(raw, "$", fn); err != nil {
			return err
		}
	default:
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if opts.strict {
		dec.DisallowUnknownFields()
	}
	return dec.Decode(v)
}

func toLowerCamelCase(_, s string, v any) (string, any, error) {
	if len(s) == 0 {
		return s, v, nil
	}
	return strings.ToLower(s[:1]) + s[1:], v, nil
}

func toUpperCamelCase(_, s string, v any) (string, any, error) {
	if len(s) == 0 {
		return s, v, nil
	}
	return strings.ToUpper(s[:1]) + s[1:], v, nil
}

func walkMap(m map[string]interface{}, path string, fn func(string, string, any) (string, any, error)) error {
	for key, value := range m {
		delete(m, key)
		newKey := key
		newValue := value
		currentPath := path + "." + key
		if fn != nil {
			var err error
			newKey, newValue, err = fn(currentPath, newKey, newValue)
			if err != nil {
				return err
			}
		}
		if newValue != nil {
			m[newKey] = newValue
		} else {
			continue
		}
		switch value := value.(type) {
		case map[string]any:
			if err := walkMap(value, currentPath, fn); err != nil {
				return err
			}
		case []interface{}:
			if err := walkArray(value, currentPath, fn); err != nil {
				return err
			}
		default:
		}
	}
	return nil
}

func walkArray(a []interface{}, path string, fn func(string, string, any) (string, any, error)) error {
	for i, value := range a {
		currentPath := path + "." + strconv.Itoa(i)
		switch value := value.(type) {
		case map[string]interface{}:
			if err := walkMap(value, currentPath, fn); err != nil {
				return err
			}
		case []interface{}:
			if err := walkArray(value, currentPath, fn); err != nil {
				return err
			}
		default:
		}
	}
	return nil
}

func matchJSONKey(path, pattern string) bool {
	pathParts := strings.Split(strings.ToLower(path), ".")
	patternParts := strings.Split(strings.ToLower(pattern), ".")
	if len(pathParts) != len(patternParts) {
		return false
	}
	for i := range pathParts {
		if patternParts[i] == "*" {
			continue
		}
		if pathParts[i] != patternParts[i] {
			return false
		}
	}
	return true
}

const unknownFieldPrefix = "json: unknown field "

func extractUnknownFieldKey(err error) string {
	if err == nil {
		return ""
	}
	unwraped := errors.Unwrap(err)
	if unwraped == nil {
		unwraped = err
	}
	if strings.HasPrefix(unwraped.Error(), unknownFieldPrefix) {
		return strings.Trim(
			strings.TrimPrefix(unwraped.Error(), unknownFieldPrefix),
			`"`,
		)
	}
	return ""
}
