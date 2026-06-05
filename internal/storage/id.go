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

func IDsEqual(left, right ID) bool {
	leftText := string(left)
	rightText := string(right)
	if len(leftText) >= 15 && len(rightText) >= 15 {
		return leftText[:15] == rightText[:15]
	}
	return leftText == rightText
}

func LookupRecordByID(records map[ID]Record, id ID) (ID, Record, bool) {
	if records == nil || id == "" {
		return "", Record{}, false
	}
	if record, ok := records[id]; ok {
		return id, record, true
	}
	for storedID, record := range records {
		if IDsEqual(storedID, id) {
			return storedID, record, true
		}
	}
	return "", Record{}, false
}

type IDGenerator struct {
	Prefixes  map[string]string
	Sequences map[string]uint64
	Offset    uint64
}

func NewIDGenerator(prefixes map[string]string) IDGenerator {
	return IDGenerator{
		Prefixes:  copyStringMap(prefixes),
		Sequences: make(map[string]uint64),
	}
}

func NewRuntimeIDGenerator(prefixes map[string]string) IDGenerator {
	g := NewIDGenerator(prefixes)
	g.Offset = 1000000
	return g
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
	return ID(prefix + leftPadBase36(next+g.Offset, 12)), nil
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

func EnsureUniqueKeyPrefixes(org *OrgState) {
	if org == nil {
		return
	}
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	standard := StandardKeyPrefixes()
	reserved := make(map[string]string, len(standard))
	for object, prefix := range standard {
		if prefix != "" {
			reserved[prefix] = object
		}
	}
	used := make(map[string]string, len(names))
	nextCustom := 0
	for _, name := range names {
		state := org.Objects[name]
		prefix := strings.TrimSpace(state.Definition.KeyPrefix)
		if standardPrefix := standard[name]; standardPrefix != "" {
			prefix = standardPrefix
		}
		if !keyPrefixAvailableForObject(prefix, name, used, reserved) {
			for {
				candidate := customPrefix(nextCustom)
				nextCustom++
				if keyPrefixAvailableForObject(candidate, name, used, reserved) {
					prefix = candidate
					break
				}
			}
		}
		state.Definition.KeyPrefix = prefix
		org.Objects[name] = state
		used[prefix] = name
	}
}

func keyPrefixAvailableForObject(prefix, objectName string, used, reserved map[string]string) bool {
	if len(prefix) != 3 {
		return false
	}
	if owner := used[prefix]; owner != "" && !strings.EqualFold(owner, objectName) {
		return false
	}
	if reservedObject := reserved[prefix]; reservedObject != "" && !strings.EqualFold(reservedObject, objectName) {
		return false
	}
	return true
}

var standardKeyPrefixBaseData = map[string]string{
	"Account":                 "001",
	"Contact":                 "003",
	"User":                    "005",
	"Opportunity":             "006",
	"OpportunityLineItem":     "00k",
	"RecordType":              "012",
	"Attachment":              "00P",
	"Document":                "015",
	"Organization":            "00D",
	"Group":                   "00G",
	"UserRole":                "00E",
	"Profile":                 "00e",
	"UserLicense":             "100",
	"BatchApexErrorEvent":     "1Be",
	"ContentVersion":          "068",
	"ContentDocument":         "069",
	"ContentDocumentLink":     "06A",
	"EmailTemplate":           "00X",
	"PermissionSet":           "0PS",
	"PermissionSetAssignment": "0Pa",
	"PricebookEntry":          "01u",
	"FieldPermissions":        "0FP",
	"ObjectPermissions":       "110",
	"SetupEntityAccess":       "0J0",
}

func StandardKeyPrefixes() map[string]string {
	prefixes := copyStringMap(standardKeyPrefixBaseData)
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
	base := len(alphabet)
	leadingARange := base * base
	if index < leadingARange {
		first := index / base
		second := index % base
		return "a" + string(alphabet[first]) + string(alphabet[second])
	}
	index -= leadingARange
	first := (index / (base * base)) % (base - 1)
	if first >= 36 {
		first++
	}
	second := (index / base) % base
	third := index % base
	return string(alphabet[first]) + string(alphabet[second]) + string(alphabet[third])
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
