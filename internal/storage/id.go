package storage

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type ID string

func (id ID) String() string {
	return string(id)
}

type IDGenerator struct {
	Prefixes  map[string]string
	Sequences map[string]uint64
}

func NewIDGenerator(prefixes map[string]string) IDGenerator {
	return IDGenerator{
		Prefixes:  copyStringMap(prefixes),
		Sequences: make(map[string]uint64),
	}
}

func (g *IDGenerator) Next(objectName string) (ID, error) {
	prefix := g.Prefixes[objectName]
	if prefix == "" {
		return "", fmt.Errorf("storage: missing key prefix for %s", objectName)
	}
	if len(prefix) != 3 {
		return "", fmt.Errorf("storage: key prefix for %s must be 3 characters", objectName)
	}
	next := g.Sequences[objectName] + 1
	g.Sequences[objectName] = next
	return ID(prefix + leftPadBase36(next, 12)), nil
}

func AssignDeterministicPrefixes(objectNames []string, explicit map[string]string) map[string]string {
	out := copyStringMap(StandardKeyPrefixes())
	for object, prefix := range explicit {
		if prefix != "" {
			out[object] = prefix
		}
	}

	names := append([]string(nil), objectNames...)
	sort.Strings(names)
	customIndex := 0
	for _, name := range names {
		if out[name] != "" {
			continue
		}
		prefix := customPrefix(customIndex)
		customIndex++
		for prefixInUse(out, prefix) {
			prefix = customPrefix(customIndex)
			customIndex++
		}
		out[name] = prefix
	}
	return out
}

func StandardKeyPrefixes() map[string]string {
	prefixes := map[string]string{
		"Account":                 "001",
		"Contact":                 "003",
		"User":                    "005",
		"Opportunity":             "006",
		"RecordType":              "012",
		"Attachment":              "00P",
		"Document":                "015",
		"Organization":            "00D",
		"UserRole":                "00E",
		"Profile":                 "00e",
		"ContentVersion":          "068",
		"ContentDocument":         "069",
		"ContentDocumentLink":     "06A",
		"EmailTemplate":           "00X",
		"PermissionSet":           "0PS",
		"PermissionSetAssignment": "0Pa",
	}
	for object, prefix := range standardObjectKeyPrefixes() {
		if prefix != "" {
			prefixes[object] = prefix
		}
	}
	return prefixes
}

func ValidateID(id ID) error {
	length := len(id)
	if length != 15 && length != 18 {
		return fmt.Errorf("storage: id %q must be 15 or 18 characters", id)
	}
	for _, r := range id {
		if !isBase62(r) {
			return fmt.Errorf("storage: id %q contains non-base62 character %q", id, r)
		}
	}
	return nil
}

func leftPadBase36(v uint64, width int) string {
	text := strings.ToUpper(strconv.FormatUint(v, 36))
	if len(text) >= width {
		return text
	}
	return strings.Repeat("0", width-len(text)) + text
}

func customPrefix(index int) string {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	if index < 0 {
		index = 0
	}
	first := (index / len(alphabet)) % len(alphabet)
	second := index % len(alphabet)
	return "a" + string(alphabet[first]) + string(alphabet[second])
}

func prefixInUse(prefixes map[string]string, prefix string) bool {
	for _, existing := range prefixes {
		if existing == prefix {
			return true
		}
	}
	return false
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func isBase62(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
}
