package guard

import (
	"fmt"
	"sort"
	"strings"
)

type Node struct {
	Path  []string
	Key   string
	Value any
}

func Walk(value any, visit func(Node)) {
	walk(value, nil, "", visit)
}

func walk(value any, path []string, key string, visit func(Node)) {
	visit(Node{Path: append([]string(nil), path...), Key: key, Value: value})

	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for childKey := range typed {
			keys = append(keys, childKey)
		}
		sort.Strings(keys)
		for _, childKey := range keys {
			walk(typed[childKey], append(path, childKey), childKey, visit)
		}
	case []any:
		for i, child := range typed {
			part := fmt.Sprintf("[%d]", i)
			walk(child, append(path, part), part, visit)
		}
	}
}

func FormatPath(parts []string) string {
	if len(parts) == 0 {
		return "$"
	}

	var builder strings.Builder
	for i, part := range parts {
		if strings.HasPrefix(part, "[") {
			builder.WriteString(part)
			continue
		}
		if i > 0 {
			builder.WriteByte('.')
		}
		builder.WriteString(part)
	}
	return builder.String()
}
