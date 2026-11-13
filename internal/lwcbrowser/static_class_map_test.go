package lwcbrowser

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The LWC engine applies vnode classMap as a static class list: every key is
// added and the values are ignored. A conditional class written as
// classMap: { "slds-has-error": Boolean(x) } is therefore always present.
// Conditional classes must go through className instead.
func TestBaseComponentClassMapsAreStatic(t *testing.T) {
	entry := regexp.MustCompile(`^\s*"[^"]+"\s*:\s*true\s*$`)
	names := make([]string, 0)
	for name := range SupportedLightningBaseComponentSpecifiers() {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, specifier := range names {
		name := strings.TrimPrefix(specifier, "lightning/")
		source := LightningBaseComponentModuleJS(name)
		for _, body := range classMapBodies(source) {
			for _, part := range strings.Split(body, ",") {
				if strings.TrimSpace(part) == "" {
					continue
				}
				if !entry.MatchString(part) {
					t.Errorf("%s: classMap entry %q is not static; use className for conditional classes", specifier, strings.TrimSpace(part))
				}
			}
		}
	}
}

func classMapBodies(source string) []string {
	const marker = "classMap: {"
	out := []string{}
	for {
		start := strings.Index(source, marker)
		if start < 0 {
			return out
		}
		source = source[start+len(marker):]
		end := strings.Index(source, "}")
		if end < 0 {
			return out
		}
		out = append(out, source[:end])
		source = source[end+1:]
	}
}
