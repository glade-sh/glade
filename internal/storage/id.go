package storage

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	Prefixes     map[string]string
	Sequences    map[string]uint64
	Offset       uint64
	PrefixLookup func(objectName string) string
}

func NewIDGenerator(prefixes map[string]string) IDGenerator {
	return IDGenerator{
		Prefixes:  copyStringMap(prefixes),
		Sequences: make(map[string]uint64),
	}
}

func NewStandardIDGenerator() IDGenerator {
	return NewIDGenerator(standardKeyPrefixes())
}

func NewRuntimeIDGenerator(prefixes map[string]string) IDGenerator {
	g := NewIDGenerator(prefixes)
	g.Offset = 1000000
	return g
}

// NewRuntimeIDGeneratorWithOwnedPrefixes builds a runtime generator from a map
// the caller will not mutate or share.
func NewRuntimeIDGeneratorWithOwnedPrefixes(prefixes map[string]string) IDGenerator {
	return IDGenerator{
		Prefixes:  prefixes,
		Sequences: make(map[string]uint64),
		Offset:    1000000,
	}
}

func NewRuntimeIDGeneratorForOrg(org *OrgState) IDGenerator {
	return IDGenerator{
		Prefixes:  make(map[string]string),
		Sequences: make(map[string]uint64),
		Offset:    1000000,
		PrefixLookup: func(objectName string) string {
			if org == nil {
				return ""
			}
			state, ok := org.Objects[objectName]
			if !ok {
				return ""
			}
			return state.Definition.KeyPrefix
		},
	}
}

func (g *IDGenerator) Next(objectName string) (ID, error) {
	prefix := g.Prefixes[objectName]
	if prefix == "" && g.PrefixLookup != nil {
		prefix = strings.TrimSpace(g.PrefixLookup(objectName))
		if prefix != "" {
			if g.Prefixes == nil {
				g.Prefixes = make(map[string]string)
			}
			g.Prefixes[objectName] = prefix
		}
	}
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
	metadataIndex := 0
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
		if strings.HasSuffix(strings.ToLower(name), "__mdt") {
			prefix = customMetadataPrefix(metadataIndex)
			metadataIndex++
			for prefixInUse(out, prefix) {
				prefix = customMetadataPrefix(metadataIndex)
				metadataIndex++
			}
		}
		out[name] = prefix
	}
	return out
}

func EnsureUniqueKeyPrefixes(org *OrgState) {
	if org == nil {
		return
	}
	if org.keyPrefixesValidated && org.keyPrefixesValidatedObjectCount == len(org.Objects) && keyPrefixValidationSnapshotMatches(org) {
		return
	}
	if uniqueKeyPrefixesAlreadyValid(org) {
		markKeyPrefixesValidated(org)
		return
	}
	names := make([]string, 0, len(org.Objects))
	for name := range org.Objects {
		names = append(names, name)
	}
	sort.Strings(names)
	used := make(map[string]string, len(names))
	nextCustom := 0
	nextMetadata := 0
	nextCustomSetting := 0
	reserved := make(map[string]bool, len(names))
	for _, name := range names {
		reserved[strings.TrimSpace(org.Objects[name].Definition.KeyPrefix)] = true
	}
	for _, name := range names {
		state := org.Objects[name]
		prefix := strings.TrimSpace(state.Definition.KeyPrefix)
		if standardPrefix := standardKeyPrefixForObject(name); standardPrefix != "" {
			prefix = standardPrefix
		}
		if !keyPrefixAvailableForObject(prefix, name, used) {
			for {
				candidate := ""
				switch {
				case IsCustomSettingDefinition(state.Definition):
					candidate = CustomSettingPrefix(nextCustomSetting)
					nextCustomSetting++
				case strings.HasSuffix(strings.ToLower(name), "__mdt"):
					candidate = customMetadataPrefix(nextMetadata)
					nextMetadata++
				default:
					candidate = customPrefix(nextCustom)
					nextCustom++
				}
				if !reserved[candidate] && keyPrefixAvailableForObject(candidate, name, used) {
					prefix = candidate
					break
				}
			}
		}
		state.Definition.KeyPrefix = prefix
		org.Objects[name] = state
		used[prefix] = name
	}
	markKeyPrefixesValidated(org)
}

func uniqueKeyPrefixesAlreadyValid(org *OrgState) bool {
	if len(org.Objects) == 0 {
		return true
	}
	var seen [keyPrefixBitsetWords]uint64
	for name, state := range org.Objects {
		rawPrefix := state.Definition.KeyPrefix
		prefix := strings.TrimSpace(rawPrefix)
		if rawPrefix != prefix || len(prefix) != 3 {
			return false
		}
		if standardPrefix := standardKeyPrefixForObject(name); standardPrefix != "" && prefix != standardPrefix {
			return false
		}
		if reservedObject := standardKeyPrefixOwner(prefix); reservedObject != "" && !strings.EqualFold(reservedObject, name) {
			return false
		}
		index, ok := keyPrefixBitsetIndex(prefix)
		if !ok {
			return false
		}
		word := index / 64
		mask := uint64(1) << (index % 64)
		if seen[word]&mask != 0 {
			return false
		}
		seen[word] |= mask
	}
	return true
}

// keyPrefixSnapshotEntry records one object's key prefix at validation time.
type keyPrefixSnapshotEntry struct {
	name   string
	prefix string
}

// markKeyPrefixesValidated records every object's prefix. The snapshot is a
// fresh slice on every mark because org clones share it by value copy.
func markKeyPrefixesValidated(org *OrgState) {
	org.keyPrefixesValidated = true
	org.keyPrefixesValidatedObjectCount = len(org.Objects)
	snapshot := make([]keyPrefixSnapshotEntry, 0, len(org.Objects))
	for name, state := range org.Objects {
		snapshot = append(snapshot, keyPrefixSnapshotEntry{name: name, prefix: state.Definition.KeyPrefix})
	}
	org.keyPrefixesValidatedPrefixes = snapshot
}

// keyPrefixValidationSnapshotMatches reports whether every object's prefix
// equals its snapshot prefix, reading a name absent from the snapshot as "".
// The fast path walks the snapshot slice and reads only KeyPrefix, instead of
// iterating Objects and copying every ObjectState. Any difference it sees is
// re-decided by the full Objects walk, so the answer is unchanged when a
// same-count swap adds an object without a prefix.
func keyPrefixValidationSnapshotMatches(org *OrgState) bool {
	if org.keyPrefixesValidatedPrefixes == nil || len(org.keyPrefixesValidatedPrefixes) != len(org.Objects) {
		return false
	}
	for _, entry := range org.keyPrefixesValidatedPrefixes {
		if org.Objects[entry.name].Definition.KeyPrefix != entry.prefix {
			return keyPrefixValidationSnapshotMatchesByObject(org)
		}
	}
	return true
}

// keyPrefixSnapshotObjectWalks counts full Objects walks; tests use it to keep
// the unchanged-org check on the snapshot fast path.
var keyPrefixSnapshotObjectWalks atomic.Uint64

func keyPrefixValidationSnapshotMatchesByObject(org *OrgState) bool {
	keyPrefixSnapshotObjectWalks.Add(1)
	prefixes := make(map[string]string, len(org.keyPrefixesValidatedPrefixes))
	for _, entry := range org.keyPrefixesValidatedPrefixes {
		prefixes[entry.name] = entry.prefix
	}
	for name, state := range org.Objects {
		if prefixes[name] != state.Definition.KeyPrefix {
			return false
		}
	}
	return true
}

func keyPrefixAvailableForObject(prefix, objectName string, used map[string]string) bool {
	if len(prefix) != 3 {
		return false
	}
	if owner := used[prefix]; owner != "" && !strings.EqualFold(owner, objectName) {
		return false
	}
	if reservedObject := standardKeyPrefixOwner(prefix); reservedObject != "" && !strings.EqualFold(reservedObject, objectName) {
		return false
	}
	return true
}

func standardKeyPrefixForObject(objectName string) string {
	if prefix := standardKeyPrefixBaseData[objectName]; prefix != "" {
		return prefix
	}
	return standardObjectKeyPrefixData[objectName]
}

func standardKeyPrefixOwner(prefix string) string {
	return standardKeyPrefixOwnerData[prefix]
}

const keyPrefixAlphabetSize = 62
const keyPrefixBitsetWords = (keyPrefixAlphabetSize*keyPrefixAlphabetSize*keyPrefixAlphabetSize + 63) / 64

func keyPrefixBitsetIndex(prefix string) (int, bool) {
	if len(prefix) != 3 {
		return 0, false
	}
	first, ok := keyPrefixCharIndex(prefix[0])
	if !ok {
		return 0, false
	}
	second, ok := keyPrefixCharIndex(prefix[1])
	if !ok {
		return 0, false
	}
	third, ok := keyPrefixCharIndex(prefix[2])
	if !ok {
		return 0, false
	}
	return (first*keyPrefixAlphabetSize+second)*keyPrefixAlphabetSize + third, true
}

func keyPrefixCharIndex(ch byte) (int, bool) {
	switch {
	case ch >= '0' && ch <= '9':
		return int(ch - '0'), true
	case ch >= 'A' && ch <= 'Z':
		return int(ch-'A') + 10, true
	case ch >= 'a' && ch <= 'z':
		return int(ch-'a') + 36, true
	default:
		return 0, false
	}
}

var standardKeyPrefixOwnerData = buildStandardKeyPrefixOwnerData()

func buildStandardKeyPrefixOwnerData() map[string]string {
	owners := make(map[string]string, len(standardKeyPrefixBaseData)+len(standardObjectKeyPrefixData))
	for object, prefix := range standardKeyPrefixBaseData {
		if prefix != "" {
			owners[prefix] = object
		}
	}
	for object, prefix := range standardObjectKeyPrefixData {
		if prefix != "" {
			owners[prefix] = object
		}
	}
	return owners
}

var standardKeyPrefixBaseData = map[string]string{
	"Account":                 "001",
	"Contact":                 "003",
	"CollaborationGroup":      "0F9",
	"FeedComment":             "0D7",
	"FeedItem":                "0D5",
	"User":                    "005",
	"Opportunity":             "006",
	"OpportunityLineItem":     "00k",
	"RecordType":              "012",
	"Attachment":              "00P",
	"Document":                "015",
	"Organization":            "00D",
	"Group":                   "00G",
	"ListView":                "00B",
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

var standardKeyPrefixCache struct {
	once     sync.Once
	prefixes map[string]string
}

func standardKeyPrefixes() map[string]string {
	standardKeyPrefixCache.once.Do(func() {
		prefixes := copyStringMap(standardKeyPrefixBaseData)
		for object, prefix := range standardObjectKeyPrefixes() {
			if prefix != "" {
				prefixes[object] = prefix
			}
		}
		standardKeyPrefixCache.prefixes = prefixes
	})
	return standardKeyPrefixCache.prefixes
}

func StandardKeyPrefixes() map[string]string {
	return copyStringMap(standardKeyPrefixes())
}

func StandardKeyPrefix(objectName string) string {
	return standardKeyPrefixes()[objectName]
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

// Local custom metadata uses its own prefix pool. Sorting callers provide a
// deterministic local allocation order, not an org-independent Salesforce ID.
func customMetadataPrefix(index int) string {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	if index < len(alphabet)*len(alphabet) {
		return "m" + string(alphabet[index/len(alphabet)]) + string(alphabet[index%len(alphabet)])
	}
	// Preserve the allocator's general overflow capacity after the m pool.
	return customPrefix(index)
}

// Local custom settings use their own prefix pool. This keeps synthesized IDs
// stable when an unrelated object is added to the same fixture composition.
func CustomSettingPrefix(index int) string {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	if index < 0 {
		index = 0
	}
	base := len(alphabet)
	if index < base*base {
		return "s" + string(alphabet[index/base]) + string(alphabet[index%base])
	}
	return customPrefix(index)
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
