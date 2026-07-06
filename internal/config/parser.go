package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parse decodes JSON or YAML config into Go values with map[string]any objects.
func Parse(data []byte, sourceName string) (any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("empty config")
	}

	ext := strings.ToLower(filepath.Ext(sourceName))
	if ext == ".json" {
		return parseJSON(trimmed)
	}

	if ext == ".yaml" || ext == ".yml" {
		return parseYAML(trimmed)
	}

	if looksLikeJSON(trimmed) {
		return parseJSON(trimmed)
	}

	value, err := parseYAML(trimmed)
	if err == nil {
		return value, nil
	}

	jsonValue, jsonErr := parseJSON(trimmed)
	if jsonErr == nil {
		return jsonValue, nil
	}

	return nil, fmt.Errorf("parse config: yaml: %v; json: %v", err, jsonErr)
}

func parseJSON(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("multiple JSON documents are not supported")
		}
		return nil, err
	}
	return normalize(value), nil
}

func parseYAML(data []byte) (any, error) {
	var value any
	if err := yaml.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return normalize(value), nil
}

func looksLikeJSON(data []byte) bool {
	return len(data) > 0 && (data[0] == '{' || data[0] == '[')
}

func normalize(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[key] = normalize(child)
		}
		return result
	case map[any]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			result[fmt.Sprint(key)] = normalize(child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for i, child := range typed {
			result[i] = normalize(child)
		}
		return result
	default:
		return value
	}
}
