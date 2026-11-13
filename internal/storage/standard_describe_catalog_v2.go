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
	"sync/atomic"

	"github.com/glade-sh/glade/internal/apexlang"
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
	decoded    atomic.Bool
}

type standardDescribeCatalogV2RawDecodeFunc func(standardDescribeCatalogV2IndexEntry) (standardDescribeObject, error)

type standardDescribeCatalogV2RawCache struct {
	entries    sync.Map
	decode     standardDescribeCatalogV2RawDecodeFunc
	scalarOnly bool
	// scalars answers a scalar-only lookup without decoding the member. It
	// is set for the production pack, whose generated scalar table is checked
	// against every member; injected decoders keep decoding.
	scalars func(standardDescribeCatalogV2IndexEntry) (standardDescribeObject, bool)
	// narrow answers a narrow lookup without decoding the whole member. It is
	// set for the production pack; injected decoders answer from their full
	// decode.
	narrow func(standardDescribeCatalogV2IndexEntry) (standardDescribeNarrow, error)
}

type standardDescribeCatalogV2RawCacheEntry struct {
	mu          sync.Mutex
	once        sync.Once
	indexEntry  standardDescribeCatalogV2IndexEntry
	value       standardDescribeObject
	err         error
	fullReady   bool
	scalarReady bool
	scalar      standardDescribeObject
	scalarErr   error
	narrowReady bool
	narrowValue standardDescribeNarrow
	narrowErr   error
	fieldsReady bool
	fieldsValue standardDescribeObject
	fieldsErr   error
}

var standardDescribeCatalogV2ProductionCache = newStandardDescribeCatalogV2Cache(standardDescribeCatalogV2EntryForResolvedIndexEntry)

// Raw catalog entries are immutable after decode. Callers may share them, but
// must not mutate the returned slices or nested values.
var standardDescribeCatalogV2RawProductionCache = func() *standardDescribeCatalogV2RawCache {
	cache := newStandardDescribeCatalogV2RawCache(func(entry standardDescribeCatalogV2IndexEntry) (standardDescribeObject, error) {
		standardDescribeFullDecodes.Add(1)
		return decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry)
	})
	cache.scalarOnly = true
	cache.scalars = lookupStandardDescribeCatalogV2Scalar
	cache.narrow = func(entry standardDescribeCatalogV2IndexEntry) (standardDescribeNarrow, error) {
		return decodeStandardDescribeCatalogV2Narrow(standardDescribeCatalogV2Pack, entry)
	}
	return cache
}()

func newStandardDescribeCatalogV2Cache(decode standardDescribeCatalogV2DecodeFunc) *standardDescribeCatalogV2Cache {
	return &standardDescribeCatalogV2Cache{decode: decode}
}

func newStandardDescribeCatalogV2RawCache(decode standardDescribeCatalogV2RawDecodeFunc) *standardDescribeCatalogV2RawCache {
	return &standardDescribeCatalogV2RawCache{decode: decode}
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
		cached.decoded.Store(true)
	})
	// The projected catalog entry contains maps and slices. Production callers
	// treat these shared values as immutable for the lifetime of the process.
	return cached.value, true, cached.err
}

func (cache *standardDescribeCatalogV2RawCache) rawEntryForName(objectName string) (*standardDescribeCatalogV2RawCacheEntry, bool, error) {
	objectName = strings.TrimSpace(objectName)
	if objectName == "" {
		return nil, false, nil
	}
	indexEntry, ok := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, objectName)
	if !ok {
		return nil, false, nil
	}
	if err := validateResolvedStandardDescribeCatalogV2IndexEntry(indexEntry); err != nil {
		return nil, true, err
	}
	loaded, ok := cache.entries.Load(indexEntry.Name)
	if !ok {
		loaded, _ = cache.entries.LoadOrStore(indexEntry.Name, &standardDescribeCatalogV2RawCacheEntry{indexEntry: indexEntry})
	}
	return loaded.(*standardDescribeCatalogV2RawCacheEntry), true, nil
}

func (cache *standardDescribeCatalogV2RawCache) entryForName(objectName string) (standardDescribeObject, bool, error) {
	cached, found, err := cache.rawEntryForName(objectName)
	if err != nil || !found {
		return standardDescribeObject{}, found, err
	}
	cached.mu.Lock()
	defer cached.mu.Unlock()
	cached.once.Do(func() {
		defer func() { cached.fullReady = true }()
		if cached.scalarErr != nil {
			cached.err = cached.scalarErr
			return
		}
		cached.value, cached.err = cache.decode(cached.indexEntry)
		if cached.err != nil {
			cached.value = standardDescribeObject{}
		} else if cached.scalarReady {
			cached.value.KeyPrefix = cached.scalar.KeyPrefix
			cached.value.Triggerable = cached.scalar.Triggerable
		}
	})
	return cached.value, true, cached.err
}

// scalarForName keeps only scalar facets when the immutable production member
// has not yet been fully requested. The production cache reads them from the
// generated scalar table, so a later full lookup is the member's only decode.
// Injected decoders retain their single-decode behavior, including any
// stateful results and errors.
func (cache *standardDescribeCatalogV2RawCache) scalarForName(objectName string) (standardDescribeObject, bool, error) {
	if !cache.scalarOnly {
		return cache.entryForName(objectName)
	}
	cached, found, err := cache.rawEntryForName(objectName)
	if err != nil || !found {
		return standardDescribeObject{}, found, err
	}
	cached.mu.Lock()
	defer cached.mu.Unlock()
	if !cached.scalarReady {
		value, err := cached.value, cached.err
		if !cached.fullReady {
			var fromTable bool
			if cache.scalars != nil {
				value, fromTable = cache.scalars(cached.indexEntry)
			}
			if !fromTable {
				value, err = cache.decode(cached.indexEntry)
			}
		}
		if err == nil {
			cached.scalar = standardDescribeObject{KeyPrefix: value.KeyPrefix, Triggerable: value.Triggerable}
		}
		cached.scalarErr = err
		cached.scalarReady = true
	}
	return cached.scalar, true, cached.scalarErr
}

// narrowForName answers the overlay check plan's question about an object. When
// the member is already fully decoded the answer comes from that decode.
// Otherwise the production cache scans the member once and keeps only the
// narrow result, leaving the full decode to whoever needs it. The result is
// memoized per object and is immutable.
func (cache *standardDescribeCatalogV2RawCache) narrowForName(objectName string) (standardDescribeNarrow, bool, error) {
	if cache.narrow == nil {
		describe, found, err := cache.entryForName(objectName)
		if err != nil || !found {
			return standardDescribeNarrow{}, found, err
		}
		return standardDescribeNarrowFromDescribe(describe), true, nil
	}
	cached, found, err := cache.rawEntryForName(objectName)
	if err != nil || !found {
		return standardDescribeNarrow{}, found, err
	}
	cached.mu.Lock()
	defer cached.mu.Unlock()
	if !cached.narrowReady {
		if cached.fullReady {
			if cached.err == nil {
				cached.narrowValue = standardDescribeNarrowFromDescribe(cached.value)
			}
			cached.narrowErr = cached.err
		} else {
			cached.narrowValue, cached.narrowErr = cache.narrow(cached.indexEntry)
		}
		cached.narrowReady = true
	}
	return cached.narrowValue, true, cached.narrowErr
}

// fieldsForName retains only the field and mergeable facets used by enrichment.
func (cache *standardDescribeCatalogV2RawCache) fieldsForName(objectName string) (standardDescribeObject, bool, error) {
	if cache.narrow == nil {
		return cache.entryForName(objectName)
	}
	cached, found, err := cache.rawEntryForName(objectName)
	if err != nil || !found {
		return standardDescribeObject{}, found, err
	}
	cached.mu.Lock()
	defer cached.mu.Unlock()
	if !cached.fieldsReady {
		if cached.fullReady {
			cached.fieldsValue = standardDescribeObject{Fields: cached.value.Fields, Mergeable: cached.value.Mergeable}
			cached.fieldsErr = cached.err
		} else {
			cached.fieldsValue, cached.fieldsErr = decodeStandardDescribeCatalogV2Fields(standardDescribeCatalogV2Pack, cached.indexEntry)
		}
		cached.fieldsReady = true
	}
	return cached.fieldsValue, true, cached.fieldsErr
}

func lookupStandardDescribeFieldsV2(objectName string) (standardDescribeObject, bool, error) {
	return standardDescribeCatalogV2RawProductionCache.fieldsForName(objectName)
}

func validateResolvedStandardDescribeCatalogV2IndexEntry(entry standardDescribeCatalogV2IndexEntry) error {
	canonical, ok := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, entry.Name)
	if !ok || canonical != entry {
		return fmt.Errorf("decode standard describe member %q: invalid generated index entry", entry.Name)
	}
	return nil
}

func lookupStandardDescribeCatalogV2(objectName string) (standardDescribeObject, bool, error) {
	return standardDescribeCatalogV2RawProductionCache.entryForName(objectName)
}

func lookupStandardDescribeNarrowV2(objectName string) (standardDescribeNarrow, bool, error) {
	return standardDescribeCatalogV2RawProductionCache.narrowForName(objectName)
}

func lookupStandardDescribeChildRelationshipsV2(objectName string) (standardDescribeChildRelationshipsV2Member, bool, error) {
	entry, ok := lookupStandardDescribeCatalogV2Index(standardDescribeChildRelationshipsV2Index, objectName)
	if !ok {
		return standardDescribeChildRelationshipsV2Member{}, false, nil
	}
	var member standardDescribeChildRelationshipsV2Member
	err := withStandardDescribeCatalogV2MemberBytes(standardDescribeChildRelationshipsV2Pack, entry, standardDescribeChildRelationshipsV2Magic, len(standardDescribeChildRelationshipsV2Index), func(data []byte) error {
		if err := json.Unmarshal(data, &member); err != nil {
			return fmt.Errorf("decode standard describe reverse member %q: %w", entry.Name, err)
		}
		return nil
	})
	if err != nil {
		return standardDescribeChildRelationshipsV2Member{}, true, err
	}
	return member, true, nil
}

func lookupStandardDescribeCatalogV2Index(index []standardDescribeCatalogV2IndexEntry, objectName string) (standardDescribeCatalogV2IndexEntry, bool) {
	key := apexlang.LowerName(strings.TrimSpace(objectName))
	if key == "" {
		return standardDescribeCatalogV2IndexEntry{}, false
	}
	position := sort.Search(len(index), func(position int) bool {
		return standardDescribeNameAtLeast(index[position].Name, key)
	})
	if position == len(index) || !strings.EqualFold(index[position].Name, key) {
		return standardDescribeCatalogV2IndexEntry{}, false
	}
	return index[position], true
}

// standardDescribeNameAtLeast compares a catalog name with an already lowercased
// key. Catalog names are ASCII; folding each compared byte avoids allocating a
// lowercase copy at every step of the binary search. Keep strings.ToLower's
// ordering for non-ASCII names as well, including malformed UTF-8.
func standardDescribeNameAtLeast(name, key string) bool {
	for i := 0; i < len(name) && i < len(key); i++ {
		c := name[i]
		if c >= 0x80 {
			return strings.ToLower(name) >= key
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != key[i] {
			return c > key[i]
		}
	}
	return len(name) >= len(key)
}

func decodeStandardDescribeCatalogV2Member(pack []byte, entry standardDescribeCatalogV2IndexEntry) (standardDescribeObject, error) {
	var describe standardDescribeObject
	err := withStandardDescribeCatalogV2MemberBytes(pack, entry, standardDescribeCatalogV2Magic, len(standardDescribeCatalogV2Index), func(data []byte) error {
		var decodeErr error
		describe, decodeErr = decodeStandardDescribeObjectJSON(data)
		if decodeErr != nil {
			return fmt.Errorf("decode standard describe member %q: %w", entry.Name, decodeErr)
		}
		return nil
	})
	if err != nil {
		return standardDescribeObject{}, err
	}
	return describe, nil
}

// standardDescribeMemberBuffers holds uncompressed member buffers. A buffer is
// only lent to a callback that copies what it keeps: decoded values never
// reference it (strings are copied, encoding/json copies too).
var standardDescribeMemberBuffers sync.Pool

// standardDescribeMemberBufferMaxCap keeps the pool from holding an outlier.
const standardDescribeMemberBufferMaxCap = 4 << 20

// standardDescribeMemberBufferStep rounds up a replacement pooled buffer.
const standardDescribeMemberBufferStep = 256 << 10

// decodeStandardDescribeCatalogV2MemberBytes returns a member's JSON in a
// buffer the caller owns.
func decodeStandardDescribeCatalogV2MemberBytes(pack []byte, entry standardDescribeCatalogV2IndexEntry, magic []byte, count int) ([]byte, error) {
	return decodeStandardDescribeCatalogV2MemberInto(pack, entry, magic, count, nil)
}

// withStandardDescribeCatalogV2MemberBytes verifies a member and lends its JSON
// to use from a pooled buffer. use must not retain data or slices of it.
func withStandardDescribeCatalogV2MemberBytes(pack []byte, entry standardDescribeCatalogV2IndexEntry, magic []byte, count int, use func(data []byte) error) error {
	var buffer []byte
	if pooled, ok := standardDescribeMemberBuffers.Get().(*[]byte); ok {
		buffer = *pooled
	}
	// Size a replacement in steps so the next members of similar size reuse it.
	if need := int(entry.UncompressedLength) + 1; cap(buffer) < need && need <= standardDescribeMemberBufferMaxCap {
		buffer = make([]byte, 0, (need+standardDescribeMemberBufferStep-1)/standardDescribeMemberBufferStep*standardDescribeMemberBufferStep)
	}
	data, err := decodeStandardDescribeCatalogV2MemberInto(pack, entry, magic, count, buffer[:0])
	if err == nil {
		err = use(data)
	}
	if cap(data) > 0 && cap(data) <= standardDescribeMemberBufferMaxCap {
		data = data[:0]
		standardDescribeMemberBuffers.Put(&data)
	}
	return err
}

func decodeStandardDescribeCatalogV2MemberInto(pack []byte, entry standardDescribeCatalogV2IndexEntry, magic []byte, count int, buffer []byte) ([]byte, error) {
	if len(pack) < standardDescribeCatalogV2HeaderSize || !bytes.Equal(pack[:8], magic) || binary.BigEndian.Uint32(pack[8:12]) != 2 || int64(binary.BigEndian.Uint32(pack[12:16])) != int64(count) {
		return buffer, fmt.Errorf("decode standard describe member %q: invalid pack header", entry.Name)
	}
	end := entry.Offset + uint64(entry.CompressedLength)
	if end < entry.Offset || entry.Offset < standardDescribeCatalogV2HeaderSize || end > uint64(len(pack)) {
		return buffer, fmt.Errorf("decode standard describe member %q: member bounds [%d,%d) exceed pack length %d", entry.Name, entry.Offset, end, len(pack))
	}
	memberReader := bytes.NewReader(pack[entry.Offset:end])
	reader, err := standardDescribeGzipReader(memberReader)
	if err != nil {
		return buffer, fmt.Errorf("decode standard describe member %q: %w", entry.Name, err)
	}
	defer standardDescribeGzipReaders.Put(reader)
	reader.Multistream(false)
	data, readErr := readAllSized(io.LimitReader(reader, int64(entry.UncompressedLength)+1), int(entry.UncompressedLength)+1, buffer)
	closeErr := reader.Close()
	if readErr != nil {
		return data, fmt.Errorf("decode standard describe member %q: %w", entry.Name, readErr)
	}
	if closeErr != nil {
		return data, fmt.Errorf("decode standard describe member %q: %w", entry.Name, closeErr)
	}
	if memberReader.Len() != 0 {
		return data, fmt.Errorf("decode standard describe member %q: %d trailing compressed bytes", entry.Name, memberReader.Len())
	}
	if len(data) != int(entry.UncompressedLength) {
		return data, fmt.Errorf("decode standard describe member %q: uncompressed length %d, want %d", entry.Name, len(data), entry.UncompressedLength)
	}
	if digest := sha256.Sum256(data); digest != entry.SHA256 {
		return data, fmt.Errorf("decode standard describe member %q: SHA256 %x, want %x", entry.Name, digest, entry.SHA256)
	}
	return data, nil
}

func standardDescribeCatalogV2EntryForName(objectName string) (standardObjectCatalogEntry, bool, error) {
	return standardDescribeCatalogV2ProductionCache.entryForName(objectName)
}

func standardDescribeCatalogV2EntryForResolvedIndexEntry(indexEntry standardDescribeCatalogV2IndexEntry) (standardObjectCatalogEntry, error) {
	if err := validateResolvedStandardDescribeCatalogV2IndexEntry(indexEntry); err != nil {
		return standardObjectCatalogEntry{}, err
	}
	describe, ok, err := standardDescribeCatalogV2RawProductionCache.entryForName(indexEntry.Name)
	if err != nil {
		return standardObjectCatalogEntry{}, err
	}
	if !ok {
		return standardObjectCatalogEntry{}, fmt.Errorf("standard describe member %q not found", indexEntry.Name)
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
		KeyPrefix:   describe.keyPrefixValue(),
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

// standardDescribeGzipReaders reuses gzip readers, and their inflate windows,
// across catalog members. Reset reinitializes all reader state.
var standardDescribeGzipReaders sync.Pool

func standardDescribeGzipReader(source io.Reader) (*gzip.Reader, error) {
	if reader, ok := standardDescribeGzipReaders.Get().(*gzip.Reader); ok {
		if err := reader.Reset(source); err != nil {
			standardDescribeGzipReaders.Put(reader)
			return nil, err
		}
		return reader, nil
	}
	return gzip.NewReader(source)
}

// readAllSized is io.ReadAll with an initial capacity, so a member whose size
// the index records is read without regrowing the buffer.
// A non-nil buffer with enough capacity is reused.
func readAllSized(reader io.Reader, capacity int, buffer []byte) ([]byte, error) {
	data := buffer[:0]
	if cap(data) < capacity {
		data = make([]byte, 0, capacity)
	}
	for {
		if len(data) == cap(data) {
			data = append(data, 0)[:len(data)]
		}
		n, err := reader.Read(data[len(data):cap(data)])
		data = data[:len(data)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return data, err
		}
	}
}

var standardCatalogReleaseEnabled atomic.Bool

// EnableStandardCatalogRelease lets ReleaseStandardCatalogCaches drop the
// decoded describe catalogs. A product process that builds one org shape turns
// it on, because the catalogs are then read only while that org is built.
// Library callers and tests keep the permanent caches.
func EnableStandardCatalogRelease() { standardCatalogReleaseEnabled.Store(true) }

// SetStandardCatalogReleaseEnabled sets the opt-in and returns the previous
// value, so a test can restore it.
func SetStandardCatalogReleaseEnabled(enabled bool) bool {
	return standardCatalogReleaseEnabled.Swap(enabled)
}

// StandardCatalogReleaseEnabled reports whether EnableStandardCatalogRelease ran.
func StandardCatalogReleaseEnabled() bool { return standardCatalogReleaseEnabled.Load() }

// ReleaseStandardCatalogCaches frees the decoded catalog entries. The next
// lookup decodes the member again from the embedded pack, with the same
// result. Callers that already hold an entry keep it. It does nothing unless
// EnableStandardCatalogRelease ran.
func ReleaseStandardCatalogCaches() {
	if !standardCatalogReleaseEnabled.Load() {
		return
	}
	standardDescribeCatalogV2ProductionCache.release()
	standardDescribeCatalogV2RawProductionCache.release()
	releaseStandardSObjectStubMembers()
	standardCatalogReleases.Add(1)
}

var standardCatalogReleases atomic.Uint64

// StandardCatalogReleaseCount reports how many releases ran. Tests use it to
// bound the releases a sequence of org builds performs.
func StandardCatalogReleaseCount() uint64 { return standardCatalogReleases.Load() }

// StandardCatalogDecodedObjectCount reports how many catalog objects hold a
// decoded definition. Tests use it to check that a release ran.
func StandardCatalogDecodedObjectCount() int {
	count := 0
	standardDescribeCatalogV2ProductionCache.entries.Range(func(_, loaded any) bool {
		if loaded.(*standardDescribeCatalogV2CacheEntry).decoded.Load() {
			count++
		}
		return true
	})
	return count
}

// release keeps each decoded object's label pair, which the per-test schema
// checks read for every object, then frees the decoded entries.
func (cache *standardDescribeCatalogV2Cache) release() {
	cache.entries.Range(func(_, loaded any) bool {
		cached := loaded.(*standardDescribeCatalogV2CacheEntry)
		if cached.decoded.Load() {
			standardObjectCatalogLabels(cached.indexEntry.Name)
		}
		return true
	})
	cache.entries.Clear()
}

// release drops decoded objects but keeps the scalar facts (key prefix and
// triggerable), which callers read for every object and which would otherwise
// force a full decode per lookup.
func (cache *standardDescribeCatalogV2RawCache) release() {
	cache.entries.Range(func(key, loaded any) bool {
		cached := loaded.(*standardDescribeCatalogV2RawCacheEntry)
		cached.mu.Lock()
		keep := &standardDescribeCatalogV2RawCacheEntry{indexEntry: cached.indexEntry, scalarReady: cached.scalarReady, scalar: cached.scalar, scalarErr: cached.scalarErr}
		cached.mu.Unlock()
		if keep.scalarReady {
			cache.entries.CompareAndSwap(key, loaded, keep)
		} else {
			cache.entries.CompareAndDelete(key, loaded)
		}
		return true
	})
}
