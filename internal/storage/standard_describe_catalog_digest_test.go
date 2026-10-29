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
	"os"
	"reflect"
	"regexp"
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
	if got := []int{len(typedBundle.Describes), fieldCount, childRowCount, recordTypeCount}; !reflect.DeepEqual(got, []int{5323, 81897, 43445, 766}) {
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
	v2Reverse := make(map[string]standardDescribeChildRelationshipInfo, 3697)
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
	if len(v2Reverse) != 3697 || conflictCount != 52 {
		t.Fatalf("reverse key/conflict counts = %d/%d, want 3697/52", len(v2Reverse), conflictCount)
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
	// A23 native API 62/67 rows R113/R114 and R179-R182 qualify the
	// PROJECTED-BROAD-v1 Time mapping: 35 captured time fields in 14 objects
	// now project type TIME and displayType TIME, replacing ANY and omission.
	// The broad catalog carries no source-org residue: no custom fields, no
	// custom reference targets, no custom child rows, no non-master record types,
	// and only reviewed standard picklist values.
	// The standard_picklist_values native API 62/67 rows qualify the three
	// corrected default lists and their ordering in both catalogs.
	// Commerce API62/67 native webstore_fields rows add the captured Boolean
	// OptionsAutoFacetingEnabled field; the rich and reverse catalogs are unchanged.
	// The raw and projected digests below combine that field with the restricted
	// picklist projection (the rich digest is unchanged by the Commerce field).
	// standard_restricted_picklists/org.tsv:2-5 (API 62/67) qualifies retaining
	// restrictedPicklist in the rich and projected catalogs; raw input is unchanged.
	// Projected fields also retain catalog-origin provenance so configuration
	// exclusions preserve explicit and native validation rules. The raw, rich,
	// and reverse catalogs are unchanged by this projection-only addition.
	wantDigests := map[string]string{
		"RAW-BROAD-v1":       "7227213d3c382ce9d0a9be6118331e34e40bdf99e0004b9be932b22d287f2bb3",
		"PROJECTED-BROAD-v1": "abb6cd24e88fb1a7c72e25ee50ac515032f55ef80b23681c7865735ea90541af",
		"RICH-v1":            "92239c48aed07e0db93739fb128fa64e8ac26d3b22228e79b9cc5fc3d826130f",
		"REVERSE-v1":         "a7010fc13d96f894dfc7c3efd5e8fe9b23e1a40984f78f972cf161fa59a4cf6f",
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

func TestStandardDescribeCatalogCarriesNoSourceOrgResidue(t *testing.T) {
	file, err := standardDescribeCatalogFS.Open("standard_describe_catalog.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decompressor, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	var bundle struct {
		Describes map[string]struct {
			Fields []struct {
				Name        string   `json:"name"`
				Custom      *bool    `json:"custom"`
				ReferenceTo []string `json:"referenceTo"`
			} `json:"fields"`
			ChildRelationships []struct {
				ChildSObject     string  `json:"childSObject"`
				Field            string  `json:"field"`
				RelationshipName *string `json:"relationshipName"`
			} `json:"childRelationships"`
			RecordTypeInfos []struct {
				DeveloperName string `json:"developerName"`
				Master        bool   `json:"master"`
			} `json:"recordTypeInfos"`
		} `json:"describes"`
	}
	if err := json.NewDecoder(decompressor).Decode(&bundle); err != nil {
		t.Fatal(err)
	}
	custom := func(value string) bool { return bytes.Contains([]byte(value), []byte("__")) }
	for objectName, describe := range bundle.Describes {
		for _, field := range describe.Fields {
			if (field.Custom != nil && *field.Custom) || custom(field.Name) {
				t.Errorf("%s.%s is a custom field", objectName, field.Name)
			}
			for _, target := range field.ReferenceTo {
				if custom(target) {
					t.Errorf("%s.%s references custom object %s", objectName, field.Name, target)
				}
			}
		}
		for _, relationship := range describe.ChildRelationships {
			name := ""
			if relationship.RelationshipName != nil {
				name = *relationship.RelationshipName
			}
			if custom(relationship.ChildSObject) || custom(relationship.Field) || custom(name) {
				t.Errorf("%s child relationship %s.%s is custom", objectName, relationship.ChildSObject, relationship.Field)
			}
		}
		for _, recordType := range describe.RecordTypeInfos {
			if !recordType.Master {
				t.Errorf("%s carries non-master record type %s", objectName, recordType.DeveloperName)
			}
		}
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
	if reverseOnly != 90 {
		t.Fatalf("reverse-only child objects = %d, want 90 covered by the complete reverse oracle", reverseOnly)
	}
}

func legacyStandardDescribeReverseOracle(describes map[string]standardDescribeObject) map[string]standardDescribeChildRelationshipInfo {
	out := make(map[string]standardDescribeChildRelationshipInfo, 3697)
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

// The allowlist uses the reviewed public catalog and cited standard field
// documentation, with custom entity suffixes excluded. Keep failure output free
// of source-org values.
func TestStandardDescribeCatalogStandardPicklistValues(t *testing.T) {
	allowedFile, err := os.Open("testdata/standard_picklist_values.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer allowedFile.Close()
	allowedReader, err := gzip.NewReader(allowedFile)
	if err != nil {
		t.Fatal(err)
	}
	defer allowedReader.Close()
	var values []string
	if err := json.NewDecoder(allowedReader).Decode(&values); err != nil {
		t.Fatal(err)
	}
	allowed := make(map[string]bool, len(values))
	custom := regexp.MustCompile(`__(c|e|mdt|x)$`)
	for _, value := range values {
		if custom.MatchString(value) {
			t.Fatal("allowlist contains custom entity values")
		}
		allowed[value] = true
	}
	if len(allowed) != 59049 || len(values) != len(allowed) {
		t.Fatalf("allowlist count = %d/%d", len(values), len(allowed))
	}
	file, err := standardDescribeCatalogFS.Open("standard_describe_catalog.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var bundle struct {
		Describes map[string]struct {
			Fields []struct {
				PicklistValues []struct {
					Value string `json:"value"`
				} `json:"picklistValues"`
			} `json:"fields"`
		} `json:"describes"`
	}
	if err := json.NewDecoder(reader).Decode(&bundle); err != nil {
		t.Fatal(err)
	}
	entries, fields, objects := 0, 0, 0
	distinct := map[string]bool{}
	for _, object := range bundle.Describes {
		objectInvalid := false
		for _, field := range object.Fields {
			fieldInvalid := false
			for _, picklist := range field.PicklistValues {
				if custom.MatchString(picklist.Value) || !allowed[picklist.Value] {
					entries++
					distinct[picklist.Value] = true
					fieldInvalid, objectInvalid = true, true
				}
			}
			if fieldInvalid {
				fields++
			}
		}
		if objectInvalid {
			objects++
		}
	}
	if entries != 0 {
		t.Fatalf("non-standard picklists: entries=%d distinct=%d fields=%d objects=%d", entries, len(distinct), fields, objects)
	}
}
