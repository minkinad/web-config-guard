package guard

import (
	"context"
	"errors"
	"testing"
)

func TestWalkContextStopsAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	visited := 0
	err := WalkContext(ctx, []any{1, 2, 3}, func(Node) {
		visited++
		cancel()
	})
	if !errors.Is(err, context.Canceled) || visited != 1 {
		t.Fatalf("WalkContext() error = %v, visited = %d", err, visited)
	}
}
