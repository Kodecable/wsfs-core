package harness

import (
	"context"
	"os"
	"strings"
	"time"
)

func WaitLogContains(ctx context.Context, path string, needle string) error {
	for {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), needle) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
