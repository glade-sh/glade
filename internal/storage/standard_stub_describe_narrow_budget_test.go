package storage

import (
	"runtime"
	"sort"
	"testing"
)

// standardStubDescribeObjectNames lists catalog objects that carry a generated
// stub overlay with fields, the only objects the overlay check plan asks the
// describe catalog about.
func standardStubDescribeObjectNames(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, entry := range standardDescribeCatalogV2Index {
		if standardSObjectStubHasFields(entry.Name) {
			names = append(names, entry.Name)
		}
	}
	if len(names) == 0 {
		t.Fatal("no stub objects in the describe catalog")
	}
	return names
}

// largestStandardStubDescribeObjectNames returns up to limit stub objects with
// the largest members, where a full decode costs the most.
func largestStandardStubDescribeObjectNames(t *testing.T, limit int) []string {
	t.Helper()
	names := standardStubDescribeObjectNames(t)
	size := func(name string) uint32 {
		entry, _ := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, name)
		return entry.UncompressedLength
	}
	sort.SliceStable(names, func(i, j int) bool { return size(names[i]) > size(names[j]) })
	if len(names) > limit {
		names = names[:limit]
	}
	return names
}

func totalAllocDelta(run func()) uint64 {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	run()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// The overlay check plan needs the reference fields of a stub object. Learning
// them must not allocate what a full describe decode allocates (all fields,
// picklist values, child relationships).
func TestStandardStubReferenceFieldsAllocationBudget(t *testing.T) {
	names := largestStandardStubDescribeObjectNames(t, 60)
	// Warm pools and lazily built tables so only the lookup is measured.
	for _, name := range names {
		standardStubReferenceFieldsFromV2Describe(name)
	}
	// Measure both over the whole loop: a GC per object would empty the
	// buffer and reader pools and inflate the full decode.
	full := totalAllocDelta(func() {
		for _, name := range names {
			entry, _ := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, name)
			if _, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry); err != nil {
				t.Fatal(err)
			}
		}
	})
	standardDescribeCatalogV2RawProductionCache.release()
	fullDecodesBefore := standardDescribeFullDecodes.Load()
	narrow := totalAllocDelta(func() {
		for _, name := range names {
			standardStubReferenceFieldsFromV2Describe(name)
		}
	})
	if got := standardDescribeFullDecodes.Load() - fullDecodesBefore; got != 0 {
		t.Fatalf("reference lookup ran %d full member decodes, want 0", got)
	}
	t.Logf("%d stub objects: full decode %d bytes, reference lookup %d bytes", len(names), full, narrow)
	if narrow*3 > full {
		t.Fatalf("reference lookup allocated %d bytes, budget one third of the full decode (%d)", narrow, full)
	}
}
