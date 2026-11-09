package storage

import (
	"reflect"
	"sync"
	"testing"
)

func TestReleaseStandardCatalogCachesKeepsResults(t *testing.T) {
	names := []string{"Account", "Contact", "Opportunity", "Lead", "ApptBookingUrlConfig"}
	type snapshot struct {
		Entry       standardObjectCatalogEntry
		Label       string
		PluralLabel string
		KeyPrefix   *string
		Triggerable *bool
	}
	take := func() map[string]snapshot {
		out := map[string]snapshot{}
		for _, name := range names {
			entry, ok := standardObjectCatalogEntryForName(name)
			if !ok {
				t.Fatalf("catalog entry %s missing", name)
			}
			label, plural, _ := standardObjectCatalogLabels(name)
			scalar, _, err := standardDescribeCatalogV2RawProductionCache.scalarForName(name)
			if err != nil {
				t.Fatal(err)
			}
			out[name] = snapshot{entry, label, plural, scalar.KeyPrefix, scalar.Triggerable}
		}
		return out
	}
	before := take()

	was := standardCatalogReleaseEnabled.Load()
	t.Cleanup(func() { standardCatalogReleaseEnabled.Store(was) })
	standardCatalogReleaseEnabled.Store(false)
	ReleaseStandardCatalogCaches() // disabled: no effect
	if _, ok := standardDescribeCatalogV2ProductionCache.entries.Load("ApptBookingUrlConfig"); !ok {
		t.Fatal("release ran while disabled")
	}
	EnableStandardCatalogRelease()
	ReleaseStandardCatalogCaches()
	if _, ok := standardDescribeCatalogV2ProductionCache.entries.Load("ApptBookingUrlConfig"); ok {
		t.Fatal("decoded Account entry survived release")
	}
	after := take()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("catalog results changed after release")
	}
}

func TestReleaseStandardCatalogCachesConcurrentUse(t *testing.T) {
	was := standardCatalogReleaseEnabled.Load()
	t.Cleanup(func() { standardCatalogReleaseEnabled.Store(was) })
	EnableStandardCatalogRelease()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if i%2 == 0 {
					ReleaseStandardCatalogCaches()
				}
				if _, ok := standardObjectCatalogEntryForName("Contact"); !ok {
					t.Error("Contact missing")
				}
				standardObjectCatalogLabels("Account")
				_, _, _ = standardDescribeCatalogV2RawProductionCache.scalarForName("Lead")
			}
		}(i)
	}
	wg.Wait()
}
