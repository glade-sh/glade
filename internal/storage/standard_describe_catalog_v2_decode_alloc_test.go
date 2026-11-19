package storage

import (
	"reflect"
	"runtime"
	"testing"
)

func describeDecodeBytesPerCall(t *testing.T, name string, runs int) (bytesPer uint64, uncompressed uint32) {
	t.Helper()
	entry, ok := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, name)
	if !ok {
		t.Fatalf("%s missing from catalog index", name)
	}
	if _, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry); err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for i := 0; i < runs; i++ {
		if _, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry); err != nil {
			t.Fatal(err)
		}
	}
	runtime.ReadMemStats(&after)
	return (after.TotalAlloc - before.TotalAlloc) / uint64(runs), entry.UncompressedLength
}

// The uncompressed member buffer is transient: decoded values copy every
// string out of it, so a decode must not allocate a fresh buffer of the
// member's size. Base allocates about 2.2 times the JSON size per decode; the
// budget is 1.6 times.
func TestStandardDescribeCatalogV2DecodeDoesNotAllocateMemberBuffer(t *testing.T) {
	for _, name := range []string{"Account", "User"} {
		got, size := describeDecodeBytesPerCall(t, name, 20)
		t.Logf("%s: %d bytes per decode, member JSON %d bytes", name, got, size)
		if got >= uint64(size)*decodeBudgetNumerator/decodeBudgetDenominator {
			t.Fatalf("%s: %d bytes allocated per decode, budget %d (member JSON %d)", name, got, uint64(size)*decodeBudgetNumerator/decodeBudgetDenominator, size)
		}
	}
}

const (
	decodeBudgetNumerator   = 8
	decodeBudgetDenominator = 5
)

// Decoding other members through the pooled buffer must not change a value
// decoded earlier.
func TestStandardDescribeCatalogV2PooledBufferDoesNotAliasDecodedValues(t *testing.T) {
	entryA, _ := lookupStandardDescribeCatalogV2Index(standardDescribeCatalogV2Index, "Account")
	a, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entryA)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entryA)
	if err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 3; round++ {
		for _, entry := range standardDescribeCatalogV2Index {
			if _, err := decodeStandardDescribeCatalogV2Member(standardDescribeCatalogV2Pack, entry); err != nil {
				t.Fatal(err)
			}
		}
		if !reflect.DeepEqual(a, reference) {
			t.Fatalf("Account changed after decoding other members (round %d)", round)
		}
	}
}
