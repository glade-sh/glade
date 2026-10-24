package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

var standardSObjectStubOverlayMagic = []byte{'G', 'L', 'A', 'D', 'E', 'S', '2', 0}

type standardSObjectStubPackMember struct {
	ObjectInfo        standardSObjectStubObjectInfo `json:"objectInfo"`
	Fields            map[string]Field              `json:"fields"`
	ReadOnlyFields    []string                      `json:"readOnlyFields"`
	Relationships     []Relationship                `json:"relationships"`
	hasObjectInfo     bool                          `json:"-"`
	hasFields         bool                          `json:"-"`
	hasReadOnlyFields bool                          `json:"-"`
	hasRelationships  bool                          `json:"-"`
}

type standardSObjectStubPackCacheEntry struct {
	once   sync.Once
	index  standardSObjectStubPackIndexEntry
	member standardSObjectStubPackMember
	err    error
}

var standardSObjectStubPackMembers sync.Map

// standardSObjectStubPackIndexEntryForName uses the generated sorted index so
// callers can test membership without decoding a packed member.
func standardSObjectStubPackIndexEntryForName(objectName string) (standardSObjectStubPackIndexEntry, bool) {
	key := strings.TrimSpace(objectName)
	if key == "" {
		return standardSObjectStubPackIndexEntry{}, false
	}
	low, high := 0, len(standardSObjectStubPackIndex)
	for low < high {
		middle := int(uint(low+high) >> 1)
		if standardSObjectStubNameAtLeast(standardSObjectStubPackIndex[middle].Name, key) {
			high = middle
		} else {
			low = middle + 1
		}
	}
	position := low
	if position == len(standardSObjectStubPackIndex) || !strings.EqualFold(standardSObjectStubPackIndex[position].Name, key) {
		return standardSObjectStubPackIndexEntry{}, false
	}
	return standardSObjectStubPackIndex[position], true
}

func standardSObjectStubNameAtLeast(name, key string) bool {
	for index := 0; index < len(name) && index < len(key); index++ {
		nameByte, keyByte := name[index], key[index]
		if nameByte >= 0x80 || keyByte >= 0x80 {
			return strings.ToLower(name) >= strings.ToLower(key)
		}
		if nameByte >= 'A' && nameByte <= 'Z' {
			nameByte += 'a' - 'A'
		}
		if keyByte >= 'A' && keyByte <= 'Z' {
			keyByte += 'a' - 'A'
		}
		if nameByte != keyByte {
			return nameByte > keyByte
		}
	}
	return len(name) >= len(key)
}

func standardSObjectStubHasFieldsExact(objectName string) bool {
	entry, ok := standardSObjectStubPackIndexEntryForName(objectName)
	return ok && entry.HasFields && entry.Name == objectName
}

func standardSObjectStubPackMemberForName(objectName string) (standardSObjectStubPackMember, bool) {
	index, ok := standardSObjectStubPackIndexEntryForName(objectName)
	if !ok {
		return standardSObjectStubPackMember{}, false
	}
	loaded, ok := standardSObjectStubPackMembers.Load(index.Name)
	if !ok {
		loaded, _ = standardSObjectStubPackMembers.LoadOrStore(index.Name, &standardSObjectStubPackCacheEntry{index: index})
	}
	cached := loaded.(*standardSObjectStubPackCacheEntry)
	cached.once.Do(func() {
		entry := standardDescribeCatalogV2IndexEntry{
			Name:               cached.index.Name,
			Offset:             cached.index.Offset,
			CompressedLength:   cached.index.CompressedLength,
			UncompressedLength: cached.index.UncompressedLength,
			SHA256:             cached.index.SHA256,
		}
		data, err := decodeStandardDescribeCatalogV2MemberBytes(
			standardSObjectStubOverlayPack,
			entry,
			standardSObjectStubOverlayMagic,
			len(standardSObjectStubPackIndex),
		)
		if err != nil {
			cached.err = fmt.Errorf("decode standard sObject stub member %q: %w", cached.index.Name, err)
			return
		}
		if err := json.Unmarshal(data, &cached.member); err != nil {
			cached.err = fmt.Errorf("decode standard sObject stub member %q: %w", cached.index.Name, err)
			cached.member = standardSObjectStubPackMember{}
			return
		}
		cached.member.hasObjectInfo = cached.index.HasObjectInfo
		cached.member.hasFields = cached.index.HasFields
		cached.member.hasReadOnlyFields = cached.index.HasReadOnlyFields
		cached.member.hasRelationships = cached.index.HasRelationships
	})
	if cached.err != nil {
		panic(cached.err)
	}
	return cached.member, true
}

func standardSObjectStubObjectInfoFor(objectName string) (standardSObjectStubObjectInfo, bool) {
	member, ok := standardSObjectStubPackMemberForName(objectName)
	if !ok || !member.hasObjectInfo {
		return standardSObjectStubObjectInfo{}, false
	}
	return member.ObjectInfo, true
}

func standardSObjectStubFieldsFor(objectName string) (map[string]Field, bool) {
	member, ok := standardSObjectStubPackMemberForName(objectName)
	if !ok || !member.hasFields {
		return nil, false
	}
	return member.Fields, true
}

func standardSObjectStubReadOnlyFieldsFor(objectName string) ([]string, bool) {
	member, ok := standardSObjectStubPackMemberForName(objectName)
	if !ok || !member.hasReadOnlyFields {
		return nil, false
	}
	return member.ReadOnlyFields, true
}

func standardSObjectStubRelationshipsFor(objectName string) ([]Relationship, bool) {
	member, ok := standardSObjectStubPackMemberForName(objectName)
	if !ok || !member.hasRelationships {
		return nil, false
	}
	return member.Relationships, true
}

func standardSObjectStubNames() []string {
	names := make([]string, 0, len(standardSObjectStubPackIndex))
	for _, entry := range standardSObjectStubPackIndex {
		if entry.HasFields {
			names = append(names, entry.Name)
		}
	}
	return names
}
