package storage

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"reflect"
	"sort"
	"testing"
)

func TestStandardDescribeCatalogV2DigestOracle(t *testing.T) {
	freshCache := newStandardDescribeCatalogV2Cache(standardDescribeCatalogV2EntryForResolvedIndexEntry)
	previousCache := standardDescribeCatalogV2ProductionCache
	standardDescribeCatalogV2ProductionCache = freshCache
	defer func() { standardDescribeCatalogV2ProductionCache = previousCache }()

	file, err := standardDescribeCatalogFS.Open("standard_describe_catalog.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	decompressor, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	broadJSON, err := io.ReadAll(decompressor)
	if err != nil {
		t.Fatal(err)
	}
	if err := decompressor.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	var rawBundle struct {
		Describes map[string]json.RawMessage `json:"describes"`
	}
	if err := json.Unmarshal(broadJSON, &rawBundle); err != nil {
		t.Fatal(err)
	}
	var typedBundle standardDescribeCatalogBundle
	if err := json.Unmarshal(broadJSON, &typedBundle); err != nil {
		t.Fatal(err)
	}

	var fieldCount, childRowCount, recordTypeCount int
	for _, describe := range typedBundle.Describes {
		fieldCount += len(describe.Fields)
		childRowCount += len(describe.ChildRelationships)
		recordTypeCount += len(describe.RecordTypeInfos)
	}
	if got := []int{len(typedBundle.Describes), fieldCount, childRowCount, recordTypeCount}; !reflect.DeepEqual(got, []int{5323, 81918, 43473, 820}) {
		t.Fatalf("raw counts objects/fields/childRows/recordTypes = %v", got)
	}

	rawDigest := newStandardDescribeDigest("RAW-BROAD-v1")
	projectedDigest := newStandardDescribeDigest("PROJECTED-BROAD-v1")
	legacy := loadEmbeddedStandardDescribeCatalog()
	for _, indexEntry := range standardDescribeCatalogV2Index {
		member, err := decodeStandardDescribeCatalogV2MemberBytes(standardDescribeCatalogV2Pack, indexEntry, standardDescribeCatalogV2Magic, len(standardDescribeCatalogV2Index))
		if err != nil {
			t.Fatal(err)
		}
		original, ok := rawBundle.Describes[indexEntry.Name]
		if !ok {
			t.Fatalf("raw broad object missing at describes.%s", indexEntry.Name)
		}
		var originalValue, memberValue any
		if err := json.Unmarshal(original, &originalValue); err != nil {
			t.Fatalf("raw describes.%s: %v", indexEntry.Name, err)
		}
		originalValue = canonicalizeStandardDescribeJSON(originalValue, "", 0)
		if err := json.Unmarshal(member, &memberValue); err != nil {
			t.Fatalf("V2 describes.%s: %v", indexEntry.Name, err)
		}
		if path, equal := firstStandardDescribeJSONMismatch("describes."+indexEntry.Name, originalValue, memberValue); !equal {
			t.Fatalf("raw V2 mismatch at %s", path)
		}
		rawDigest.add([]byte(indexEntry.Name))
		rawDigest.add(member)

		v2Entry, err := standardDescribeCatalogV2EntryForResolvedIndexEntry(indexEntry)
		if err != nil {
			t.Fatalf("project V2 %s: %v", indexEntry.Name, err)
		}
		legacyEntry, ok := legacy[indexEntry.Name]
		if !ok {
			t.Fatalf("legacy projected object missing at %s", indexEntry.Name)
		}
		legacyEntry = canonicalizeStandardDescribeProjectedEntry(legacyEntry)
		v2Entry = canonicalizeStandardDescribeProjectedEntry(v2Entry)
		legacyJSON, err := json.Marshal(legacyEntry)
		if err != nil {
			t.Fatal(err)
		}
		v2JSON, err := json.Marshal(v2Entry)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(legacyJSON, v2JSON) {
			var legacyValue, v2Value any
			_ = json.Unmarshal(legacyJSON, &legacyValue)
			_ = json.Unmarshal(v2JSON, &v2Value)
			path, _ := firstStandardDescribeJSONMismatch("projected."+indexEntry.Name, legacyValue, v2Value)
			t.Fatalf("projected V2 mismatch at %s", path)
		}
		projectedDigest.add([]byte(indexEntry.Name))
		projectedDigest.add(legacyJSON)
	}

	reverseDigest := newStandardDescribeDigest("REVERSE-v1")
	type reverseDigestMember struct {
		name string
		data []byte
	}
	reverseMembers := make([]reverseDigestMember, 0, len(standardDescribeChildRelationshipsV2Index))
	v2Reverse := make(map[string]standardDescribeChildRelationshipInfo, 3705)
	for _, indexEntry := range standardDescribeChildRelationshipsV2Index {
		member, err := decodeStandardDescribeCatalogV2MemberBytes(standardDescribeChildRelationshipsV2Pack, indexEntry, standardDescribeChildRelationshipsV2Magic, len(standardDescribeChildRelationshipsV2Index))
		if err != nil {
			t.Fatal(err)
		}
		var reverse standardDescribeChildRelationshipsV2Member
		if err := json.Unmarshal(member, &reverse); err != nil {
			t.Fatalf("reverse.%s: %v", indexEntry.Name, err)
		}
		if reverse.ChildSObject != indexEntry.Name {
			t.Fatalf("reverse.%s childSObject = %q", indexEntry.Name, reverse.ChildSObject)
		}
		for key, relationship := range standardDescribeChildRelationshipMapV2(reverse) {
			if _, exists := v2Reverse[key]; exists {
				t.Fatalf("reverse V2 duplicate projected key at %s", key)
			}
			v2Reverse[key] = relationship
		}
		reverseMembers = append(reverseMembers, reverseDigestMember{name: indexEntry.Name, data: member})
	}
	conflictCount := 0
	for _, relationship := range v2Reverse {
		if relationship.conflict {
			conflictCount++
		}
	}
	if len(v2Reverse) != 3705 || conflictCount != 52 {
		t.Fatalf("reverse key/conflict counts = %d/%d, want 3705/52", len(v2Reverse), conflictCount)
	}
	legacyReverse := legacyStandardDescribeReverseOracle(typedBundle.Describes)
	if len(legacyReverse) != len(v2Reverse) {
		t.Fatalf("reverse key count legacy/V2 = %d/%d", len(legacyReverse), len(v2Reverse))
	}
	allReverseKeys := make([]string, 0, len(legacyReverse)+len(v2Reverse))
	seenReverseKey := map[string]bool{}
	for key := range legacyReverse {
		seenReverseKey[key] = true
		allReverseKeys = append(allReverseKeys, key)
	}
	for key := range v2Reverse {
		if !seenReverseKey[key] {
			allReverseKeys = append(allReverseKeys, key)
		}
	}
	sort.Strings(allReverseKeys)
	for _, key := range allReverseKeys {
		legacyRelationship, legacyOK := legacyReverse[key]
		v2Relationship, v2OK := v2Reverse[key]
		if !legacyOK || !v2OK {
			t.Fatalf("reverse key mismatch at %s: legacy=%v V2=%v", key, legacyOK, v2OK)
		}
		if legacyRelationship.relationshipName != v2Relationship.relationshipName {
			t.Fatalf("reverse mismatch at %s.relationshipName: legacy=%q V2=%q", key, legacyRelationship.relationshipName, v2Relationship.relationshipName)
		}
		if legacyRelationship.conflict != v2Relationship.conflict {
			t.Fatalf("reverse mismatch at %s.conflict: legacy=%v V2=%v", key, legacyRelationship.conflict, v2Relationship.conflict)
		}
		if legacyRelationship.cascadeDelete != v2Relationship.cascadeDelete {
			t.Fatalf("reverse mismatch at %s.cascadeDelete: legacy=%v V2=%v", key, legacyRelationship.cascadeDelete, v2Relationship.cascadeDelete)
		}
		if legacyRelationship.restrictedDelete != v2Relationship.restrictedDelete {
			t.Fatalf("reverse mismatch at %s.restrictedDelete: legacy=%v V2=%v", key, legacyRelationship.restrictedDelete, v2Relationship.restrictedDelete)
		}
	}
	for _, member := range reverseMembers {
		reverseDigest.add([]byte(member.name))
		reverseDigest.add(member.data)
	}

	richJSON, err := json.Marshal(standardObjectCatalogData)
	if err != nil {
		t.Fatal(err)
	}
	if len(standardObjectCatalogData) != 30 {
		t.Fatalf("rich object count = %d, want 30", len(standardObjectCatalogData))
	}
	for name, richEntry := range standardObjectCatalogData {
		got, ok := standardObjectCatalogEntryForName(name)
		if !ok || !reflect.DeepEqual(got, richEntry) {
			t.Fatalf("rich catalog priority mismatch at %s", name)
		}
	}
	richDigest := newStandardDescribeDigest("RICH-v1")
	richDigest.add(richJSON)

	gotDigests := map[string]string{
		"RAW-BROAD-v1":       rawDigest.sum(),
		"PROJECTED-BROAD-v1": projectedDigest.sum(),
		"RICH-v1":            richDigest.sum(),
		"REVERSE-v1":         reverseDigest.sum(),
	}
	wantDigests := map[string]string{
		"RAW-BROAD-v1":       "f94933f548ac5c35519a493f1c2cd49be847d4c06fd3373d0c3bee2189931dd6",
		"PROJECTED-BROAD-v1": "0bf585e9f5ce11f288f511d216c0b46d6088cafa8b74a7542e8e83a02f39485e",
		"RICH-v1":            "1e68e1c0dcb057a1c41b97aa13d74493bcd3a767ce567cf321621b698d17ecde",
		"REVERSE-v1":         "aa015ff86cdbf7a2ccb15e0c5e3bd1cbc3429ea1be2e08345f1e6332c42c049a",
	}
	for _, section := range []string{"RAW-BROAD-v1", "PROJECTED-BROAD-v1", "RICH-v1", "REVERSE-v1"} {
		if gotDigests[section] != wantDigests[section] {
			t.Errorf("%s digest = %s, want %s", section, gotDigests[section], wantDigests[section])
		}
	}
	if got := standardDescribeCatalogV2CacheEntryCount(freshCache); got != 0 {
		t.Fatalf("complete digest oracle retained %d V2 objects, want 0", got)
	}
}

func TestStandardDescribeCatalogV2ReverseOracleCoversNonCatalogChildren(t *testing.T) {
	catalogNames := make(map[string]bool, len(standardDescribeCatalogV2Index))
	for _, entry := range standardDescribeCatalogV2Index {
		catalogNames[entry.Name] = true
	}
	reverseOnly := 0
	for _, entry := range standardDescribeChildRelationshipsV2Index {
		if !catalogNames[entry.Name] {
			reverseOnly++
		}
	}
	if reverseOnly != 92 {
		t.Fatalf("reverse-only child objects = %d, want 92 covered by the complete reverse oracle", reverseOnly)
	}
}

func legacyStandardDescribeReverseOracle(describes map[string]standardDescribeObject) map[string]standardDescribeChildRelationshipInfo {
	out := make(map[string]standardDescribeChildRelationshipInfo, 3705)
	for _, describe := range describes {
		for _, relationship := range describe.ChildRelationships {
			if relationship.DeprecatedAndHidden || relationship.ChildSObject == "" || relationship.Field == "" || relationship.RelationshipName == "" {
				continue
			}
			key := describeChildRelationshipKey(relationship.ChildSObject, relationship.Field)
			existing := out[key]
			if existing.relationshipName == "" || relationship.RelationshipName < existing.relationshipName {
				if existing.relationshipName != "" && existing.relationshipName != relationship.RelationshipName {
					existing.conflict = true
				}
				existing.relationshipName = relationship.RelationshipName
			} else if existing.relationshipName != relationship.RelationshipName {
				existing.conflict = true
			}
			existing.cascadeDelete = existing.cascadeDelete || relationship.CascadeDelete
			existing.restrictedDelete = existing.restrictedDelete || relationship.RestrictedDelete
			out[key] = existing
		}
	}
	return out
}

func canonicalizeStandardDescribeProjectedEntry(entry standardObjectCatalogEntry) standardObjectCatalogEntry {
	entry.Definition.Relations = append([]Relationship(nil), entry.Definition.Relations...)
	sort.SliceStable(entry.Definition.Relations, func(left, right int) bool {
		leftJSON, _ := json.Marshal(entry.Definition.Relations[left])
		rightJSON, _ := json.Marshal(entry.Definition.Relations[right])
		return bytes.Compare(leftJSON, rightJSON) < 0
	})
	entry.Definition.RecordTypes = append([]RecordTypeInfo(nil), entry.Definition.RecordTypes...)
	sort.SliceStable(entry.Definition.RecordTypes, func(left, right int) bool {
		leftJSON, _ := json.Marshal(entry.Definition.RecordTypes[left])
		rightJSON, _ := json.Marshal(entry.Definition.RecordTypes[right])
		return bytes.Compare(leftJSON, rightJSON) < 0
	})
	return entry
}

func canonicalizeStandardDescribeJSON(value any, key string, depth int) any {
	switch typed := value.(type) {
	case map[string]any:
		for childKey, child := range typed {
			typed[childKey] = canonicalizeStandardDescribeJSON(child, childKey, depth+1)
		}
		return typed
	case []any:
		for index := range typed {
			typed[index] = canonicalizeStandardDescribeJSON(typed[index], "", depth+1)
		}
		lessJSON := func(left, right any) bool {
			leftJSON, _ := json.Marshal(left)
			rightJSON, _ := json.Marshal(right)
			return bytes.Compare(leftJSON, rightJSON) < 0
		}
		stringAt := func(value any, field string) string {
			mapped, _ := value.(map[string]any)
			text, _ := mapped[field].(string)
			return text
		}
		switch key {
		case "referenceTo", "junctionReferenceTo", "junctionIdListNames":
			sort.SliceStable(typed, func(left, right int) bool {
				return bytes.Compare([]byte(fmt.Sprint(typed[left])), []byte(fmt.Sprint(typed[right]))) < 0
			})
		case "fields":
			if depth != 1 {
				break
			}
			sort.SliceStable(typed, func(left, right int) bool {
				leftName, rightName := stringAt(typed[left], "name"), stringAt(typed[right], "name")
				if leftName != rightName {
					return bytes.Compare([]byte(leftName), []byte(rightName)) < 0
				}
				return lessJSON(typed[left], typed[right])
			})
		case "childRelationships":
			if depth != 1 {
				break
			}
			sort.SliceStable(typed, func(left, right int) bool {
				for _, field := range []string{"childSObject", "field", "relationshipName"} {
					leftValue, rightValue := stringAt(typed[left], field), stringAt(typed[right], field)
					if leftValue != rightValue {
						return bytes.Compare([]byte(leftValue), []byte(rightValue)) < 0
					}
				}
				return lessJSON(typed[left], typed[right])
			})
		case "recordTypeInfos":
			if depth != 1 {
				break
			}
			sort.SliceStable(typed, func(left, right int) bool {
				for _, field := range []string{"developerName", "recordTypeId"} {
					leftValue, rightValue := stringAt(typed[left], field), stringAt(typed[right], field)
					if leftValue != rightValue {
						return bytes.Compare([]byte(leftValue), []byte(rightValue)) < 0
					}
				}
				return lessJSON(typed[left], typed[right])
			})
		}
		return typed
	default:
		return value
	}
}

type standardDescribeDigest struct {
	hash hash.Hash
}

func newStandardDescribeDigest(section string) *standardDescribeDigest {
	digest := &standardDescribeDigest{hash: sha256.New()}
	digest.add([]byte(section))
	return digest
}

func (digest *standardDescribeDigest) add(value []byte) {
	var length [4]byte
	if uint64(len(value)) > uint64(^uint32(0)) {
		panic("standard describe digest value exceeds uint32")
	}
	binary.BigEndian.PutUint32(length[:], uint32(len(value)))
	_, _ = digest.hash.Write(length[:])
	_, _ = digest.hash.Write(value)
}

func (digest *standardDescribeDigest) sum() string {
	return hex.EncodeToString(digest.hash.Sum(nil))
}

func firstStandardDescribeJSONMismatch(path string, left, right any) (string, bool) {
	if reflect.TypeOf(left) != reflect.TypeOf(right) {
		return fmt.Sprintf("%s (types %T/%T)", path, left, right), false
	}
	switch typed := left.(type) {
	case map[string]any:
		other := right.(map[string]any)
		keys := make([]string, 0, len(typed)+len(other))
		seen := map[string]bool{}
		for key := range typed {
			seen[key] = true
			keys = append(keys, key)
		}
		for key := range other {
			if !seen[key] {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			leftValue, leftOK := typed[key]
			rightValue, rightOK := other[key]
			if !leftOK || !rightOK {
				return fmt.Sprintf("%s.%s (present %v/%v)", path, key, leftOK, rightOK), false
			}
			if mismatch, equal := firstStandardDescribeJSONMismatch(path+"."+key, leftValue, rightValue); !equal {
				return mismatch, false
			}
		}
		return "", true
	case []any:
		other := right.([]any)
		if len(typed) != len(other) {
			return fmt.Sprintf("%s (length %d/%d)", path, len(typed), len(other)), false
		}
		for index := range typed {
			if mismatch, equal := firstStandardDescribeJSONMismatch(fmt.Sprintf("%s[%d]", path, index), typed[index], other[index]); !equal {
				return mismatch, false
			}
		}
		return "", true
	default:
		if !reflect.DeepEqual(left, right) {
			return fmt.Sprintf("%s (%v/%v)", path, left, right), false
		}
		return "", true
	}
}
