package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	MaxDocumentBytes = 10 << 20
	MaxDocumentDepth = 64
	MaxDocumentNodes = 10_000
	MaxPathBytes     = 512
)

// Parse decodes JSON or YAML config into Go values with map[string]any objects.
func Parse(data []byte, sourceName string) (any, error) {
	return ParseContext(context.Background(), data, sourceName)
}

// ParseContext bounds document size and structure and observes cancellation.
func ParseContext(ctx context.Context, data []byte, sourceName string) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(data) > MaxDocumentBytes {
		return nil, fmt.Errorf("input exceeds 10 MiB")
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("empty config")
	}

	ext := strings.ToLower(filepath.Ext(sourceName))
	if ext == ".json" {
		return parseJSON(ctx, trimmed)
	}

	if ext == ".yaml" || ext == ".yml" {
		return parseYAML(ctx, trimmed)
	}

	if looksLikeJSON(trimmed) {
		return parseJSON(ctx, trimmed)
	}

	value, err := parseYAML(ctx, trimmed)
	if err == nil {
		return value, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	jsonValue, jsonErr := parseJSON(ctx, trimmed)
	if jsonErr == nil {
		return jsonValue, nil
	}

	return nil, fmt.Errorf("parse config: yaml: %v; json: %v", err, jsonErr)
}

func parseJSON(ctx context.Context, data []byte) (any, error) {
	if err := checkJSONComplexity(ctx, data); err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(contextReader{ctx: ctx, reader: bytes.NewReader(data)})
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
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return normalize(ctx, value, 0)
}

func parseYAML(ctx context.Context, data []byte) (any, error) {
	decoder := yaml.NewDecoder(contextReader{ctx: ctx, reader: bytes.NewReader(data)})

	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("multiple YAML documents are not supported")
		}
		return nil, err
	}
	if err := checkYAMLComplexity(ctx, &document); err != nil {
		return nil, err
	}
	var value any
	if err := document.Decode(&value); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return normalize(ctx, value, 0)
}

func checkJSONComplexity(ctx context.Context, data []byte) error {
	decoder := json.NewDecoder(contextReader{ctx: ctx, reader: bytes.NewReader(data)})
	decoder.UseNumber()
	depth, nodes := 0, 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		nodes++
		if nodes > MaxDocumentNodes {
			return fmt.Errorf("document exceeds %d nodes", MaxDocumentNodes)
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{', '[':
				depth++
				if depth > MaxDocumentDepth {
					return fmt.Errorf("document exceeds depth %d", MaxDocumentDepth)
				}
			case '}', ']':
				depth--
			}
		}
	}
}

func checkYAMLComplexity(ctx context.Context, document *yaml.Node) error {
	type item struct {
		node  *yaml.Node
		depth int
	}
	stack := []item{{document, 0}}
	nodes := 0
	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		nodes++
		if nodes > MaxDocumentNodes {
			return fmt.Errorf("document exceeds %d nodes", MaxDocumentNodes)
		}
		if current.node.Kind == yaml.AliasNode {
			return errors.New("YAML aliases are not supported")
		}
		depth := current.depth
		if current.node.Kind == yaml.MappingNode || current.node.Kind == yaml.SequenceNode {
			depth++
			if depth > MaxDocumentDepth {
				return fmt.Errorf("document exceeds depth %d", MaxDocumentDepth)
			}
		}
		for _, child := range current.node.Content {
			stack = append(stack, item{child, depth})
		}
	}
	return nil
}

func looksLikeJSON(data []byte) bool {
	return len(data) > 0 && (data[0] == '{' || data[0] == '[')
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (reader contextReader) Read(data []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.reader.Read(data)
}

func normalize(ctx context.Context, value any, pathBytes int) (any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			childPathBytes := pathBytes + len(key)
			if pathBytes > 0 {
				childPathBytes++
			}
			if childPathBytes > MaxPathBytes {
				return nil, fmt.Errorf("document path exceeds %d bytes", MaxPathBytes)
			}
			value, err := normalize(ctx, child, childPathBytes)
			if err != nil {
				return nil, err
			}
			result[key] = value
		}
		return result, nil
	case map[any]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			name := fmt.Sprint(key)
			childPathBytes := pathBytes + len(name)
			if pathBytes > 0 {
				childPathBytes++
			}
			if childPathBytes > MaxPathBytes {
				return nil, fmt.Errorf("document path exceeds %d bytes", MaxPathBytes)
			}
			value, err := normalize(ctx, child, childPathBytes)
			if err != nil {
				return nil, err
			}
			result[name] = value
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for i, child := range typed {
			childPathBytes := pathBytes + len(fmt.Sprint(i)) + 2
			if childPathBytes > MaxPathBytes {
				return nil, fmt.Errorf("document path exceeds %d bytes", MaxPathBytes)
			}
			value, err := normalize(ctx, child, childPathBytes)
			if err != nil {
				return nil, err
			}
			result[i] = value
		}
		return result, nil
	default:
		return value, nil
	}
}
