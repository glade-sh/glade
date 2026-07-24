package storage

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

const standardDescribeCatalogV2HeaderSize = 16

var (
	standardDescribeCatalogV2Magic            = []byte{'G', 'L', 'A', 'D', 'E', 'C', '2', 0}
	standardDescribeChildRelationshipsV2Magic = []byte{'G', 'L', 'A', 'D', 'E', 'R', '2', 0}
)

//go:embed standard_describe_catalog_v2.pack
var standardDescribeCatalogV2Pack []byte

//go:embed standard_describe_child_relationships_v2.pack
var standardDescribeChildRelationshipsV2Pack []byte

type standardDescribeCatalogV2IndexEntry struct {
	Name               string
	Offset             uint64
	CompressedLength   uint32
	UncompressedLength uint32
	SHA256             [32]byte
}

type standardDescribeChildRelationshipsV2Member struct {
	ChildSObject string                                    `json:"childSObject"`
	Fields       []standardDescribeChildRelationshipV2Info `json:"fields"`
}

type standardDescribeChildRelationshipV2Info struct {
	Field            string `json:"field"`
	RelationshipName string `json:"relationshipName"`
	CascadeDelete    bool   `json:"cascadeDelete"`
	RestrictedDelete bool   `json:"restrictedDelete"`
	Conflict         bool   `json:"conflict"`
}

type standardDescribeCatalogV2DecodeFunc func(standardDescribeCatalogV2IndexEntry) (standardObjectCatalogEntry, error)

type standardDescribeCatalogV2Cache struct {
	entries sync.Map
	decode  standardDescribeCatalogV2DecodeFunc
}

type standardDescribeCatalogV2CacheEntry struct {
	once       sync.Once
	indexEntry standardDescribeCatalogV2IndexEntry
	value      standardObjectCatalogEntry
	err        error
}

var standardDescribeCatalogV2ProductionCache = newStandardDescribeCatalogV2Cache(standardDescribeCatalogV2EntryForResolvedIndexEntry)

func newStandardDescribeCatalogV2Cache(decode standardDescribeCatalogV2DecodeFunc) *standardDescribeCatalogV2Cache {
	return &standardDescribeCatalogV2Cache{decode: decode}
}

func (cache *standardDescribeCatalogV2Cache) entryForName(objectName string) (standardObjectCatalogEntry, bool, error) {
	objectName = strings.TrimSpace(objectName)
	if objectName == "" {
		return standardObjectCatalogEntry{}, false, nil
	}
	if loaded, ok := cache.entries.Load(objectName); ok {
		cached := loaded.(*standardDescribeCatalogV2CacheEntry)
		return cache.decodeCachedEntry(cached)
	}
	indexEntry, ok := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, objectName)
	if !ok {
		return standardObjectCatalogEntry{}, false, nil
	}
	if err := validateResolvedStandardDescribeCatalogV2IndexEntry(indexEntry); err != nil {
		return standardObjectCatalogEntry{}, true, err
	}

	loaded, _ := cache.entries.LoadOrStore(indexEntry.Name, &standardDescribeCatalogV2CacheEntry{indexEntry: indexEntry})
	cached := loaded.(*standardDescribeCatalogV2CacheEntry)
	return cache.decodeCachedEntry(cached)
}

func (cache *standardDescribeCatalogV2Cache) decodeCachedEntry(cached *standardDescribeCatalogV2CacheEntry) (standardObjectCatalogEntry, bool, error) {
	cached.once.Do(func() {
		cached.value, cached.err = cache.decode(cached.indexEntry)
		if cached.err != nil {
			cached.value = standardObjectCatalogEntry{}
		}
	})
	// The projected catalog entry contains maps and slices. Production callers
	// treat these shared values as immutable for the lifetime of the process.
	return cached.value, true, cached.err
}

func validateResolvedStandardDescribeCatalogV2IndexEntry(entry standardDescribeCatalogV2IndexEntry) error {
	canonical, ok := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, entry.Name)
	if !ok || canonical != entry {
		return fmt.Errorf("decode standard describe member %q: invalid generated index entry", entry.Name)
	}
	return nil
}

func lookupStandardDescribeCatalogV2(objectName string) (standardDescribeObject, bool, error) {
	entry, ok := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, objectName)
	if !ok {
		return standardDescribeObject{}, false, nil
	}
	describe, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry)
	return describe, true, err
}

func lookupStandardDescribeChildRelationshipsV2(objectName string) (standardDescribeChildRelationshipsV2Member, bool, error) {
	entry, ok := lookupStandardDescribeCatalogV2Index(standardDescribeChildRelationshipsV2Index, objectName)
	if !ok {
		return standardDescribeChildRelationshipsV2Member{}, false, nil
	}
	data, err := decodeStandardDescribeCatalogV2MemberBytes(standardDescribeChildRelationshipsV2Pack, entry, standardDescribeChildRelationshipsV2Magic, len(standardDescribeChildRelationshipsV2Index))
	if err != nil {
		return standardDescribeChildRelationshipsV2Member{}, true, err
	}
	var member standardDescribeChildRelationshipsV2Member
	if err := json.Unmarshal(data, &member); err != nil {
		return standardDescribeChildRelationshipsV2Member{}, true, fmt.Errorf("decode standard describe reverse member %q: %w", entry.Name, err)
	}
	return member, true, nil
}

func lookupStandardDescribeCatalogV2Index(index []standardDescribeCatalogV2IndexEntry, objectName string) (standardDescribeCatalogV2IndexEntry, bool) {
	key := strings.ToLower(strings.TrimSpace(objectName))
	if key == "" {
		return standardDescribeCatalogV2IndexEntry{}, false
	}
	position := sort.Search(len(index), func(position int) bool {
		return strings.ToLower(index[position].Name) >= key
	})
	if position == len(index) || !strings.EqualFold(index[position].Name, key) {
		return standardDescribeCatalogV2IndexEntry{}, false
	}
	return index[position], true
}

func decodeStandardDescribeCatalogV2Member(pack []byte, entry standardDescribeCatalogV2IndexEntry) (standardDescribeObject, error) {
	data, err := decodeStandardDescribeCatalogV2MemberBytes(pack, entry, standardDescribeCatalogV2Magic, len(standardDescribeCatalogV2Index))
	if err != nil {
		return standardDescribeObject{}, err
	}
	var describe standardDescribeObject
	if err := json.Unmarshal(data, &describe); err != nil {
		return standardDescribeObject{}, fmt.Errorf("decode standard describe member %q: %w", entry.Name, err)
	}
	return describe, nil
}

func decodeStandardDescribeCatalogV2MemberBytes(pack []byte, entry standardDescribeCatalogV2IndexEntry, magic []byte, count int) ([]byte, error) {
	if len(pack) < standardDescribeCatalogV2HeaderSize || !bytes.Equal(pack[:8], magic) || binary.BigEndian.Uint32(pack[8:12]) != 2 || int64(binary.BigEndian.Uint32(pack[12:16])) != int64(count) {
		return nil, fmt.Errorf("decode standard describe member %q: invalid pack header", entry.Name)
	}
	end := entry.Offset + uint64(entry.CompressedLength)
	if end < entry.Offset || entry.Offset < standardDescribeCatalogV2HeaderSize || end > uint64(len(pack)) {
		return nil, fmt.Errorf("decode standard describe member %q: member bounds [%d,%d) exceed pack length %d", entry.Name, entry.Offset, end, len(pack))
	}
	memberReader := bytes.NewReader(pack[entry.Offset:end])
	reader, err := gzip.NewReader(memberReader)
	if err != nil {
		return nil, fmt.Errorf("decode standard describe member %q: %w", entry.Name, err)
	}
	reader.Multistream(false)
	data, readErr := io.ReadAll(io.LimitReader(reader, int64(entry.UncompressedLength)+1))
	closeErr := reader.Close()
	if readErr != nil {
		return nil, fmt.Errorf("decode standard describe member %q: %w", entry.Name, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("decode standard describe member %q: %w", entry.Name, closeErr)
	}
	if memberReader.Len() != 0 {
		return nil, fmt.Errorf("decode standard describe member %q: %d trailing compressed bytes", entry.Name, memberReader.Len())
	}
	if len(data) != int(entry.UncompressedLength) {
		return nil, fmt.Errorf("decode standard describe member %q: uncompressed length %d, want %d", entry.Name, len(data), entry.UncompressedLength)
	}
	if digest := sha256.Sum256(data); digest != entry.SHA256 {
		return nil, fmt.Errorf("decode standard describe member %q: SHA256 %x, want %x", entry.Name, digest, entry.SHA256)
	}
	return data, nil
}

func standardDescribeCatalogV2EntryForName(objectName string) (standardObjectCatalogEntry, bool, error) {
	return standardDescribeCatalogV2ProductionCache.entryForName(objectName)
}

func standardDescribeCatalogV2EntryForResolvedIndexEntry(indexEntry standardDescribeCatalogV2IndexEntry) (standardObjectCatalogEntry, error) {
	describe, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, indexEntry)
	if err != nil {
		return standardObjectCatalogEntry{}, err
	}
	childRelationships := map[string]standardDescribeChildRelationshipInfo{}
	if reverse, reverseOK, reverseErr := lookupStandardDescribeChildRelationshipsV2(describe.Name); reverseErr != nil {
		return standardObjectCatalogEntry{}, reverseErr
	} else if reverseOK {
		childRelationships = standardDescribeChildRelationshipMapV2(reverse)
	}
	entry := standardObjectCatalogEntry{Definition: ObjectDefinition{
		APIName:     describe.Name,
		Label:       firstStorageString(describe.Label, describe.Name),
		PluralLabel: firstStorageString(describe.LabelPlural, describe.Label+"s", describe.Name+"s"),
		KeyPrefix:   describe.KeyPrefix,
		Fields:      describeFieldMap(describe.Fields),
		Relations:   describeRelationships(describe.Name, describe.Fields, childRelationships),
		RecordTypes: describeRecordTypes(describe.RecordTypeInfos),
	}}
	return entry, nil
}

func standardDescribeChildRelationshipMapV2(reverse standardDescribeChildRelationshipsV2Member) map[string]standardDescribeChildRelationshipInfo {
	out := make(map[string]standardDescribeChildRelationshipInfo, len(reverse.Fields))
	for _, field := range reverse.Fields {
		key := describeChildRelationshipKey(reverse.ChildSObject, field.Field)
		existing := out[key]
		if existing.relationshipName == "" || field.RelationshipName < existing.relationshipName {
			if existing.relationshipName != "" && existing.relationshipName != field.RelationshipName {
				existing.conflict = true
			}
			existing.relationshipName = field.RelationshipName
		} else if field.RelationshipName != "" && existing.relationshipName != field.RelationshipName {
			existing.conflict = true
		}
		existing.conflict = existing.conflict || field.Conflict
		existing.cascadeDelete = existing.cascadeDelete || field.CascadeDelete
		existing.restrictedDelete = existing.restrictedDelete || field.RestrictedDelete
		out[key] = existing
	}
	return out
}
