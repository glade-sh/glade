package aura

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

var lwcChildRE = regexp.MustCompile(`(?i)<([a-zA-Z0-9_]+):([a-zA-Z0-9_]+)\b`)

// LWCPassthrough describes an Aura component that delegates to a single LWC child.
type LWCPassthrough struct {
	AuraName string
	Target   string
}

func BuildLWCPassthroughIndex(p project.Project) ([]LWCPassthrough, error) {
	namespace := strings.TrimSpace(p.Namespace)
	if namespace == "" {
		namespace = "c"
	}
	var out []LWCPassthrough
	for _, group := range groupAuraApps(p.AuraFiles) {
		bundleName := filepath.Base(filepath.Dir(group[0]))
		for _, path := range group {
			if !hasSuffixFold(path, ".cmp") || hasSuffixFold(path, "-meta.xml") {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			target, ok := ParseLWCPassthrough(string(data), namespace)
			if !ok {
				continue
			}
			out = append(out, LWCPassthrough{
				AuraName: bundleName,
				Target:   target,
			})
		}
	}
	return out, nil
}

func lwcChildReferences(source string) [][2]string {
	matches := lwcChildRE.FindAllStringSubmatch(source, -1)
	out := make([][2]string, 0, len(matches))
	for _, m := range matches {
		prefix := strings.ToLower(strings.TrimSpace(m[1]))
		if prefix == "aura" || prefix == "lightning" {
			continue
		}
		out = append(out, [2]string{m[1], m[2]})
	}
	return out
}

func ParseLWCPassthrough(source, namespace string) (string, bool) {
	refs := lwcChildReferences(source)
	if len(refs) != 1 {
		return "", false
	}
	prefix := strings.TrimSpace(refs[0][0])
	lwcName := strings.TrimSpace(refs[0][1])
	if prefix == "" || lwcName == "" {
		return "", false
	}
	return resolveComponentQualified(prefix, lwcName, namespace), true
}

func resolveComponentQualified(prefix, name, namespace string) string {
	if prefix == "c" || strings.EqualFold(prefix, namespace) {
		return lookupKey(namespace + ":" + name)
	}
	return lookupKey(prefix + ":" + name)
}
