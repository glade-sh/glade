package storage

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestDescribeChildRelationshipMapConflictIsOrderIndependent(t *testing.T) {
	rows := []standardDescribeChildRelationship{
		{ChildSObject: "Child__c", Field: "Parent__c", RelationshipName: "First__r", CascadeDelete: true},
		{ChildSObject: "Child__c", Field: "Parent__c", RelationshipName: "Second__r", RestrictedDelete: true},
		{ChildSObject: "Child__c", Field: "Parent__c", RelationshipName: "First__r"},
	}
	for _, permutation := range [][]int{{0, 1, 2}, {2, 1, 0}, {1, 0, 2}} {
		describeRows := make([]standardDescribeChildRelationship, 0, len(rows))
		for _, index := range permutation {
			describeRows = append(describeRows, rows[index])
		}
		got := describeChildRelationshipMap(map[string]standardDescribeObject{
			"Parent__c": {ChildRelationships: describeRows},
		})[describeChildRelationshipKey("Child__c", "Parent__c")]
		if !got.conflict {
			t.Fatalf("permutation %v cleared conflict: %#v", permutation, got)
		}
		if !got.cascadeDelete || !got.restrictedDelete {
			t.Fatalf("permutation %v lost delete flags: %#v", permutation, got)
		}
	}
}

func TestStandardDescribeCatalogV2IndexCanonicalAndBounded(t *testing.T) {
	if got, want := len(standardDescribeCatalogV2Index), 5323; got != want {
		t.Fatalf("catalog index entries = %d, want %d", got, want)
	}
	assertStandardDescribeV2IndexBounded(t, standardDescribeCatalogV2Pack, standardDescribeCatalogV2Index, standardDescribeCatalogV2Magic, standardDescribeCatalogV2PackSHA256)
	assertStandardDescribeV2IndexBounded(t, standardDescribeChildRelationshipsV2Pack, standardDescribeChildRelationshipsV2Index, standardDescribeChildRelationshipsV2Magic, standardDescribeChildRelationshipsV2PackSHA256)
}

func assertStandardDescribeV2IndexBounded(t *testing.T, pack []byte, index []standardDescribeCatalogV2IndexEntry, magic []byte, wantSHA256 [32]byte) {
	t.Helper()
	if len(pack) < standardDescribeCatalogV2HeaderSize || !bytes.Equal(pack[:8], magic) {
		t.Fatalf("invalid pack header")
	}
	previous := ""
	for position, entry := range index {
		if strings.ToLower(entry.Name) <= strings.ToLower(previous) {
			t.Fatalf("index is not case-fold sorted at %d: %q after %q", position, entry.Name, previous)
		}
		previous = entry.Name
	}
	byOffset := append([]standardDescribeCatalogV2IndexEntry(nil), index...)
	sort.Slice(byOffset, func(left, right int) bool { return byOffset[left].Offset < byOffset[right].Offset })
	offset := uint64(standardDescribeCatalogV2HeaderSize)
	previous = ""
	for _, entry := range byOffset {
		if entry.Name <= previous {
			t.Fatalf("pack members are not bytewise sorted: %q after %q", entry.Name, previous)
		}
		if entry.Offset != offset {
			t.Fatalf("%s offset = %d, want exact neighbor bound %d", entry.Name, entry.Offset, offset)
		}
		if entry.CompressedLength == 0 || entry.UncompressedLength == 0 {
			t.Fatalf("%s has empty member lengths", entry.Name)
		}
		memberHeader := pack[entry.Offset : entry.Offset+10]
		if !bytes.Equal(memberHeader[:4], []byte{0x1f, 0x8b, 0x08, 0x00}) || !bytes.Equal(memberHeader[4:8], []byte{0, 0, 0, 0}) || memberHeader[8] != 2 || memberHeader[9] != 255 {
			t.Fatalf("%s has nondeterministic gzip header %x", entry.Name, memberHeader)
		}
		offset += uint64(entry.CompressedLength)
		if offset > uint64(len(pack)) {
			t.Fatalf("%s exceeds pack: %d > %d", entry.Name, offset, len(pack))
		}
		previous = entry.Name
	}
	if offset != uint64(len(pack)) {
		t.Fatalf("last member ends at %d, pack ends at %d", offset, len(pack))
	}
	if got := sha256.Sum256(pack); got != wantSHA256 {
		t.Fatalf("pack SHA256 = %x, want %x", got, wantSHA256)
	}
}

func TestStandardDescribeCatalogV2LookupDecodesOneObject(t *testing.T) {
	describe, ok, err := lookupStandardDescribeCatalogV2("CareProgram")
	if err != nil || !ok {
		t.Fatalf("lookup CareProgram: ok=%v err=%v", ok, err)
	}
	if describe.Name != "CareProgram" || len(describe.Fields) == 0 {
		t.Fatalf("unexpected CareProgram describe: name=%q fields=%d", describe.Name, len(describe.Fields))
	}
}

func TestStandardDescribeCatalogV2CaseInsensitiveLookup(t *testing.T) {
	describe, ok, err := lookupStandardDescribeCatalogV2("  careprogram  ")
	if err != nil || !ok || describe.Name != "CareProgram" {
		t.Fatalf("case-insensitive lookup: name=%q ok=%v err=%v", describe.Name, ok, err)
	}
}

func TestStandardDescribeCatalogV2UnknownName(t *testing.T) {
	if _, ok, err := lookupStandardDescribeCatalogV2("DefinitelyNotAStandardObject"); err != nil || ok {
		t.Fatalf("unknown lookup: ok=%v err=%v", ok, err)
	}
}

func TestStandardDescribeCatalogV2RejectsCorruptMember(t *testing.T) {
	entry := standardDescribeCatalogV2Index[0]
	corrupt := append([]byte(nil), standardDescribeCatalogV2Pack...)
	corrupt[int(entry.Offset)+int(entry.CompressedLength)/2] ^= 0xff
	if _, err := decodeStandardDescribeCatalogV2Member(corrupt, entry); err == nil || !strings.Contains(err.Error(), entry.Name) {
		t.Fatalf("corrupt member error = %v", err)
	}
}

func TestStandardDescribeCatalogV2RejectsTrailingMemberBytes(t *testing.T) {
	entry := standardDescribeCatalogV2Index[0]
	member := append([]byte(nil), standardDescribeCatalogV2Pack[entry.Offset:entry.Offset+uint64(entry.CompressedLength)]...)
	for _, testCase := range []struct {
		name     string
		trailing []byte
	}{
		{name: "garbage", trailing: []byte("trailing garbage")},
		{name: "second gzip member", trailing: member},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			pack := append([]byte(nil), standardDescribeCatalogV2Pack[:standardDescribeCatalogV2HeaderSize]...)
			pack = append(pack, member...)
			pack = append(pack, testCase.trailing...)
			trailingEntry := entry
			trailingEntry.Offset = standardDescribeCatalogV2HeaderSize
			trailingEntry.CompressedLength = uint32(len(member) + len(testCase.trailing))
			if _, err := decodeStandardDescribeCatalogV2Member(pack, trailingEntry); err == nil || !strings.Contains(err.Error(), entry.Name) {
				t.Fatalf("trailing member error = %v", err)
			}
		})
	}
	if _, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry); err != nil {
		t.Fatalf("exact member bound: %v", err)
	}
}

func TestStandardDescribeCatalogV2ReverseIndex(t *testing.T) {
	reverse, ok, err := lookupStandardDescribeChildRelationshipsV2("careprogram")
	if err != nil || !ok {
		t.Fatalf("reverse lookup: ok=%v err=%v", ok, err)
	}
	if reverse.ChildSObject != "CareProgram" || len(reverse.Fields) == 0 {
		t.Fatalf("unexpected reverse member: child=%q fields=%d", reverse.ChildSObject, len(reverse.Fields))
	}
}

func TestStandardDescribeCatalogV2ReverseProjectionMergesCaseFoldFields(t *testing.T) {
	reverse := standardDescribeChildRelationshipsV2Member{
		ChildSObject: "Child__c",
		Fields: []standardDescribeChildRelationshipV2Info{
			{Field: "ParentId", RelationshipName: "First", CascadeDelete: true},
			{Field: "parentID", RelationshipName: "Second", RestrictedDelete: true},
			{Field: "ParentId", RelationshipName: "First"},
		},
	}
	got := standardDescribeChildRelationshipMapV2(reverse)[describeChildRelationshipKey("Child__c", "ParentId")]
	if got.relationshipName != "First" || !got.conflict || !got.cascadeDelete || !got.restrictedDelete {
		t.Fatalf("case-fold projection merge = %#v", got)
	}
}

func TestStandardDescribeCatalogV2CacheCanonicalLookupsDecodeOnce(t *testing.T) {
	var decodes atomic.Int64
	cache := newStandardDescribeCatalogV2Cache(func(entry standardDescribeCatalogV2IndexEntry) (standardObjectCatalogEntry, error) {
		decodes.Add(1)
		return standardDescribeCatalogV2EntryForResolvedIndexEntry(entry)
	})

	var first standardObjectCatalogEntry
	for index, name := range []string{"CareProgram", "careprogram", "  CAREPROGRAM  "} {
		got, ok, err := cache.entryForName(name)
		if err != nil || !ok {
			t.Fatalf("lookup %q: ok=%v err=%v", name, ok, err)
		}
		if index == 0 {
			first = got
		} else if !reflect.DeepEqual(got, first) {
			t.Fatalf("lookup %q differs from canonical lookup", name)
		}
	}
	if got := decodes.Load(); got != 1 {
		t.Fatalf("canonical lookup decodes = %d, want 1", got)
	}
	if got := standardDescribeCatalogV2CacheEntryCount(cache); got != 1 {
		t.Fatalf("canonical lookup cache entries = %d, want 1", got)
	}
}

func TestStandardDescribeCatalogV2CacheWarmCanonicalLookupAllocatesNothing(t *testing.T) {
	cache := newStandardDescribeCatalogV2Cache(standardDescribeCatalogV2EntryForResolvedIndexEntry)
	if _, ok, err := cache.entryForName("CareProgram"); err != nil || !ok {
		t.Fatalf("warmup CareProgram: ok=%v err=%v", ok, err)
	}
	allocations := testing.AllocsPerRun(1000, func() {
		if _, ok, err := cache.entryForName("CareProgram"); err != nil || !ok {
			panic("cached CareProgram lookup failed")
		}
	})
	if allocations != 0 {
		t.Fatalf("warm canonical lookup allocations = %g, want 0", allocations)
	}
}

func TestStandardDescribeCatalogV2CacheConcurrentLookupDecodesOnce(t *testing.T) {
	var decodes atomic.Int64
	cache := newStandardDescribeCatalogV2Cache(func(entry standardDescribeCatalogV2IndexEntry) (standardObjectCatalogEntry, error) {
		decodes.Add(1)
		return standardDescribeCatalogV2EntryForResolvedIndexEntry(entry)
	})

	const workers = 64
	start := make(chan struct{})
	errorsByWorker := make(chan error, workers)
	var wait sync.WaitGroup
	wait.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func(worker int) {
			defer wait.Done()
			<-start
			name := []string{"CareProgram", "careprogram", " CAREPROGRAM "}[worker%3]
			entry, ok, err := cache.entryForName(name)
			if err != nil || !ok || entry.Definition.APIName != "CareProgram" {
				errorsByWorker <- errors.New("concurrent canonical lookup failed")
			}
		}(worker)
	}
	close(start)
	wait.Wait()
	close(errorsByWorker)
	for err := range errorsByWorker {
		t.Fatal(err)
	}
	if got := decodes.Load(); got != 1 {
		t.Fatalf("concurrent lookup decodes = %d, want 1", got)
	}
}

func TestStandardDescribeCatalogV2CacheStickyDecodeError(t *testing.T) {
	wantErr := errors.New("corrupt CareProgram member")
	var decodes atomic.Int64
	cache := newStandardDescribeCatalogV2Cache(func(standardDescribeCatalogV2IndexEntry) (standardObjectCatalogEntry, error) {
		decodes.Add(1)
		return standardObjectCatalogEntry{Definition: ObjectDefinition{APIName: "partial"}}, wantErr
	})

	for _, name := range []string{"CareProgram", " careprogram ", "CAREPROGRAM"} {
		entry, ok, err := cache.entryForName(name)
		if !ok || !errors.Is(err, wantErr) || entry.Definition.APIName != "" {
			t.Fatalf("sticky error lookup %q: entry=%#v ok=%v err=%v", name, entry, ok, err)
		}
	}
	if got := decodes.Load(); got != 1 {
		t.Fatalf("sticky error decodes = %d, want 1", got)
	}
	if got := standardDescribeCatalogV2CacheEntryCount(cache); got != 1 {
		t.Fatalf("sticky error cache entries = %d, want 1", got)
	}
}

func TestStandardDescribeCatalogV2CacheUnknownAndEmptyDoNotDecodeOrCache(t *testing.T) {
	var decodes atomic.Int64
	cache := newStandardDescribeCatalogV2Cache(func(entry standardDescribeCatalogV2IndexEntry) (standardObjectCatalogEntry, error) {
		decodes.Add(1)
		return standardDescribeCatalogV2EntryForResolvedIndexEntry(entry)
	})

	for _, name := range []string{"", "   ", "DefinitelyNotAStandardObject"} {
		entry, ok, err := cache.entryForName(name)
		if err != nil || ok {
			t.Fatalf("unknown lookup %q: entry=%#v ok=%v err=%v", name, entry, ok, err)
		}
	}
	if got := decodes.Load(); got != 0 {
		t.Fatalf("unknown lookups decoded %d members, want 0", got)
	}
	if got := standardDescribeCatalogV2CacheEntryCount(cache); got != 0 {
		t.Fatalf("unknown lookups cached %d entries, want 0", got)
	}
}

func TestStandardDescribeCatalogV2CachedAndUncachedEntriesMatchLegacy(t *testing.T) {
	withReverse := "CareProgram"
	withoutReverse := ""
	for _, indexEntry := range standardDescribeCatalogV2Index {
		if _, ok := lookupStandardDescribeCatalogV2Index(standardDescribeChildRelationshipsV2Index, indexEntry.Name); !ok {
			withoutReverse = indexEntry.Name
			break
		}
	}
	if withoutReverse == "" {
		t.Fatal("catalog has no object without a reverse member")
	}

	legacy := loadEmbeddedStandardDescribeCatalog()
	cache := newStandardDescribeCatalogV2Cache(standardDescribeCatalogV2EntryForResolvedIndexEntry)
	for _, name := range []string{withReverse, withoutReverse} {
		indexEntry, ok := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, name)
		if !ok {
			t.Fatalf("resolve %s", name)
		}
		uncached, err := standardDescribeCatalogV2EntryForResolvedIndexEntry(indexEntry)
		if err != nil {
			t.Fatalf("uncached %s: %v", name, err)
		}
		cached, ok, err := cache.entryForName(strings.ToLower(name))
		if err != nil || !ok {
			t.Fatalf("cached %s: ok=%v err=%v", name, ok, err)
		}
		want, ok := legacy[name]
		if !ok {
			t.Fatalf("legacy %s missing", name)
		}
		if !reflect.DeepEqual(cached, uncached) || !reflect.DeepEqual(canonicalizeStandardDescribeProjectedEntry(cached), canonicalizeStandardDescribeProjectedEntry(want)) {
			t.Fatalf("cached/uncached/legacy mismatch for %s", name)
		}
	}
}

func TestStandardDescribeCatalogV2NamesOnlyPathsDecodeNothing(t *testing.T) {
	resetKnownStandardObjectCacheForTest()
	defer resetKnownStandardObjectCacheForTest()

	var decodes atomic.Int64
	fresh := newStandardDescribeCatalogV2Cache(func(entry standardDescribeCatalogV2IndexEntry) (standardObjectCatalogEntry, error) {
		decodes.Add(1)
		return standardDescribeCatalogV2EntryForResolvedIndexEntry(entry)
	})
	previous := standardDescribeCatalogV2ProductionCache
	standardDescribeCatalogV2ProductionCache = fresh
	defer func() { standardDescribeCatalogV2ProductionCache = previous }()

	names := KnownStandardObjectNames()
	if len(names) == 0 || !IsKnownStandardObject(" careprogram ") || IsKnownStandardObject("DefinitelyNotAStandardObject") {
		t.Fatal("names-only standard object resolution returned unexpected results")
	}
	if got := decodes.Load(); got != 0 {
		t.Fatalf("names-only paths decoded %d V2 members, want 0", got)
	}
	if got := standardDescribeCatalogV2CacheEntryCount(fresh); got != 0 {
		t.Fatalf("names-only paths cached %d V2 entries, want 0", got)
	}
	if standardObjectCatalogLookupCache.describeByLC != nil {
		t.Fatalf("names-only paths hydrated %d legacy catalog entries", len(standardObjectCatalogLookupCache.describeByLC))
	}
}

func TestStandardDescribeCatalogV2CacheMixedConcurrentLookups(t *testing.T) {
	var decodes atomic.Int64
	cache := newStandardDescribeCatalogV2Cache(func(entry standardDescribeCatalogV2IndexEntry) (standardObjectCatalogEntry, error) {
		decodes.Add(1)
		return standardDescribeCatalogV2EntryForResolvedIndexEntry(entry)
	})
	lookups := []string{"CareProgram", " careprogram ", "Account", "ACCOUNT", "Contact", " contact ", "", "DefinitelyNotAStandardObject"}

	const workers = 32
	var wait sync.WaitGroup
	wait.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func(worker int) {
			defer wait.Done()
			for iteration := 0; iteration < 20; iteration++ {
				name := lookups[(worker+iteration)%len(lookups)]
				_, ok, err := cache.entryForName(name)
				wantKnown := strings.TrimSpace(name) != "" && name != "DefinitelyNotAStandardObject"
				if err != nil || ok != wantKnown {
					t.Errorf("mixed lookup %q: ok=%v err=%v", name, ok, err)
					return
				}
			}
		}(worker)
	}
	wait.Wait()
	if got := decodes.Load(); got != 3 {
		t.Fatalf("mixed lookups decoded %d canonical objects, want 3", got)
	}
	if got := standardDescribeCatalogV2CacheEntryCount(cache); got != 3 {
		t.Fatalf("mixed lookups cached %d entries, want 3", got)
	}
}

func standardDescribeCatalogV2CacheEntryCount(cache *standardDescribeCatalogV2Cache) int {
	count := 0
	cache.entries.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
