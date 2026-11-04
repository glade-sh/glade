package storage

import (
	"errors"
	"testing"
)

func TestScalarCacheDecodeOrderingAndErrors(t *testing.T) {
	for _, scalarFirst := range []bool{true, false} {
		calls := 0
		cache := newStandardDescribeCatalogV2RawCache(func(standardDescribeCatalogV2IndexEntry) (standardDescribeObject, error) {
			calls++
			prefix := "001"
			flag := true
			return standardDescribeObject{KeyPrefix: &prefix, Triggerable: &flag, Fields: []standardDescribeField{{}}}, nil
		})
		cache.scalarOnly = true
		var scalar, full standardDescribeObject
		var found bool
		var err error
		if scalarFirst {
			scalar, found, err = cache.scalarForName("Account")
		} else {
			full, found, err = cache.entryForName("Account")
		}
		if err != nil || !found {
			t.Fatalf("first lookup: found=%v err=%v", found, err)
		}
		if scalarFirst {
			full, found, err = cache.entryForName("Account")
		} else {
			scalar, found, err = cache.scalarForName("Account")
		}
		if err != nil || !found || scalar.KeyPrefix != full.KeyPrefix || scalar.Triggerable != full.Triggerable || len(full.Fields) != 1 {
			t.Fatalf("second lookup: found=%v err=%v", found, err)
		}
		wantCalls := 1
		if scalarFirst {
			wantCalls = 2
		}
		if calls != wantCalls {
			t.Fatalf("decode calls=%d, want %d", calls, wantCalls)
		}
	}

	sentinel := errors.New("synthetic decode failure")
	for _, scalarOnly := range []bool{true, false} {
		calls := 0
		cache := newStandardDescribeCatalogV2RawCache(func(standardDescribeCatalogV2IndexEntry) (standardDescribeObject, error) {
			calls++
			return standardDescribeObject{}, sentinel
		})
		cache.scalarOnly = scalarOnly
		_, foundScalar, scalarErr := cache.scalarForName("Account")
		_, foundFull, fullErr := cache.entryForName("Account")
		if !foundScalar || !foundFull || scalarErr != sentinel || fullErr != sentinel || calls != 1 {
			t.Fatalf("sticky failure: scalar=%v full=%v calls=%d", scalarErr, fullErr, calls)
		}
		if _, found, err := cache.scalarForName("unknown synthetic object"); found || err != nil {
			t.Fatalf("unknown lookup: found=%v err=%v", found, err)
		}
	}
}
