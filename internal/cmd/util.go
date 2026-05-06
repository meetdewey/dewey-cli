package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func intToString(n int) string { return strconv.Itoa(n) }

// parseKVPairs parses key=value flags into a map[string]any. JSON values are
// parsed if they parse cleanly; otherwise the raw string is used.
func parseKVPairs(pairs []string) (map[string]any, error) {
	if len(pairs) == 0 {
		return nil, nil
	}
	out := make(map[string]any, len(pairs))
	for _, kv := range pairs {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			return nil, fmt.Errorf("invalid metadata pair %q (expected key=value)", kv)
		}
		k := kv[:idx]
		v := kv[idx+1:]
		var asJSON any
		if err := json.Unmarshal([]byte(v), &asJSON); err == nil {
			out[k] = asJSON
		} else {
			out[k] = v
		}
	}
	return out, nil
}

func truncate(s string, n int) string {
	if n <= 1 {
		return s
	}
	if len([]rune(s)) <= n {
		return s
	}
	rs := []rune(s)
	return string(rs[:n-1]) + "…"
}
