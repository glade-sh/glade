package namespaceremap

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

type Rule struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func ApplyNamespace(rules []Rule, namespace string) string {
	for _, rule := range rules {
		if strings.EqualFold(namespace, rule.From) {
			return rule.To
		}
	}
	return namespace
}

func ApplyMetadataName(rules []Rule, name string) string {
	return rewriteNamespaceTokens(rules, name)
}

func ApplySource(rules []Rule, source string) string {
	if len(rules) == 0 || source == "" {
		return source
	}
	var out strings.Builder
	out.Grow(len(source))
	for i := 0; i < len(source); {
		switch {
		case strings.HasPrefix(source[i:], "//"):
			end := strings.IndexByte(source[i:], '\n')
			if end < 0 {
				out.WriteString(source[i:])
				return out.String()
			}
			out.WriteString(source[i : i+end+1])
			i += end + 1
		case strings.HasPrefix(source[i:], "/*"):
			end := strings.Index(source[i+2:], "*/")
			if end < 0 {
				out.WriteString(source[i:])
				return out.String()
			}
			end += i + 4
			out.WriteString(source[i:end])
			i = end
		case source[i] == '\'':
			end := stringLiteralEnd(source, i)
			out.WriteByte('\'')
			out.WriteString(rewriteNamespaceTokens(rules, source[i+1:end]))
			if end < len(source) {
				out.WriteByte('\'')
				i = end + 1
			} else {
				i = end
			}
		default:
			replacement, width := namespaceTokenReplacement(rules, source, i)
			if width > 0 {
				out.WriteString(replacement)
				i += width
				continue
			}
			out.WriteByte(source[i])
			i++
		}
	}
	return out.String()
}

func Fingerprint(rules []Rule) string {
	if len(rules) == 0 {
		return ""
	}
	parts := make([]string, 0, len(rules))
	for _, rule := range rules {
		parts = append(parts, strings.ToLower(rule.From)+"\x00"+rule.To)
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:8])
}

func rewriteNamespaceTokens(rules []Rule, value string) string {
	if len(rules) == 0 || value == "" {
		return value
	}
	var out strings.Builder
	out.Grow(len(value))
	for i := 0; i < len(value); {
		replacement, width := namespaceTokenReplacement(rules, value, i)
		if width > 0 {
			out.WriteString(replacement)
			i += width
			continue
		}
		out.WriteByte(value[i])
		i++
	}
	return out.String()
}

func namespaceTokenReplacement(rules []Rule, value string, offset int) (string, int) {
	if offset > 0 && isIdentifierByte(value[offset-1]) {
		return "", 0
	}
	for _, rule := range rules {
		if rule.From == "" || rule.To == "" || len(value)-offset < len(rule.From) {
			continue
		}
		if !strings.EqualFold(value[offset:offset+len(rule.From)], rule.From) {
			continue
		}
		next := offset + len(rule.From)
		if strings.HasPrefix(value[next:], "__") || strings.HasPrefix(value[next:], ".") {
			return rule.To, len(rule.From)
		}
	}
	return "", 0
}

func stringLiteralEnd(source string, start int) int {
	for i := start + 1; i < len(source); i++ {
		switch source[i] {
		case '\\':
			if i+1 < len(source) {
				i++
			}
		case '\'':
			if i+1 < len(source) && source[i+1] == '\'' {
				i++
				continue
			}
			return i
		}
	}
	return len(source)
}

func isIdentifierByte(b byte) bool {
	return b == '_' || ('0' <= b && b <= '9') || ('A' <= b && b <= 'Z') || ('a' <= b && b <= 'z')
}
