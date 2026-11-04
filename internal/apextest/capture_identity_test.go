package apextest

import (
	"fmt"
	"sort"
	"strings"
)

// normalizeCaptureIdentities preserves same-invocation identity and ordering.
// Standard prefixes remain distinct; custom object prefixes are assigned
// distinct tokens per invocation, following sorted fields and array order.
func normalizeCaptureIdentities(value any) any {
	ids := map[string]string{}
	counts := map[string]int{}
	customPrefixes := map[string]string{}
	var visit func(any) any
	visit = func(v any) any {
		switch x := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(x))
			for key := range x {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			out := make(map[string]any, len(x))
			for _, key := range keys {
				out[key] = visit(x[key])
			}
			return out
		case []any:
			out := make([]any, len(x))
			for i, item := range x {
				out[i] = visit(item)
			}
			return out
		case string:
			if strings.HasPrefix(x, "/services/data/") && strings.Contains(x, "/sobjects/") {
				prefix, suffix, ok := strings.Cut(x, "/sobjects/")
				if ok {
					object, id, found := strings.Cut(suffix, "/")
					if found {
						if normalized, ok := visit(id).(string); ok && normalized != id {
							return prefix + "/sobjects/" + object + "/" + normalized
						}
					}
				}
			}
			if len(x) != 15 && len(x) != 18 {
				return x
			}
			custom := x[0] == 'a' && strings.IndexAny(x[3:15], "0123456789") >= 0
			if !(custom || strings.HasPrefix(x, "001") || strings.HasPrefix(x, "003") || strings.HasPrefix(x, "005") || strings.HasPrefix(x, "00T") || strings.HasPrefix(x, "00U") || strings.HasPrefix(x, "707") || strings.HasPrefix(x, "08e")) {
				return x
			}
			for _, c := range x {
				if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
					return x
				}
			}
			canonical := x[:15]
			if id, ok := ids[canonical]; ok {
				return id
			}
			prefix := x[:3]
			if custom {
				if _, ok := customPrefixes[prefix]; !ok {
					customPrefixes[prefix] = fmt.Sprintf("custom%d", len(customPrefixes)+1)
				}
				prefix = customPrefixes[prefix]
			}
			counts[prefix]++
			id := fmt.Sprintf("<%s:%d>", prefix, counts[prefix])
			ids[canonical] = id
			return id
		default:
			return v
		}
	}
	return visit(value)
}
