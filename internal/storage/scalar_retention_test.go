package storage

import "testing"

func TestScalarDescribeLookupDoesNotRetainFieldGraphs(t *testing.T) {
	for _, name := range []string{"Account", "Contact", "Opportunity"} {
		t.Run(name, func(t *testing.T) {
			cache := standardDescribeCatalogV2RawProductionCache
			previous, hadPrevious := cache.entries.LoadAndDelete(name)
			previousTriggerable, hadTriggerable := standardObjectTriggerableCache.LoadAndDelete(name)
			t.Cleanup(func() {
				cache.entries.Delete(name)
				if hadPrevious {
					cache.entries.Store(name, previous)
				}
				standardObjectTriggerableCache.Delete(name)
				if hadTriggerable {
					standardObjectTriggerableCache.Store(name, previousTriggerable)
				}
			})

			prefix, known := StandardObjectDescribeKeyPrefix(name)
			if !known || prefix == nil {
				t.Fatalf("expected known prefix for %s", name)
			}
			if _, known := StandardObjectTriggerable(name); !known {
				t.Fatalf("expected known triggerable flag for %s", name)
			}
			if entry, ok := cache.entries.Load(name); ok && len(entry.(*standardDescribeCatalogV2RawCacheEntry).value.Fields) != 0 {
				t.Fatal("scalar lookup retained the full field graph")
			}
			full, found, err := lookupStandardDescribeCatalogV2(name)
			if err != nil || !found || len(full.Fields) == 0 || full.KeyPrefix == nil || *full.KeyPrefix != *prefix {
				t.Fatalf("full lookup after scalar lookup: found=%v err=%v", found, err)
			}
		})
	}
}
