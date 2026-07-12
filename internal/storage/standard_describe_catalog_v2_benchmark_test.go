package storage

import (
	"sync/atomic"
	"testing"
)

func BenchmarkStandardDescribeCatalogLegacyLoad(b *testing.B) {
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		if catalog := loadEmbeddedStandardDescribeCatalog(); len(catalog) != 5323 {
			b.Fatalf("legacy catalog objects = %d", len(catalog))
		}
	}
}

func BenchmarkStandardDescribeCatalogV2CareProgramCold(b *testing.B) {
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		cache := newStandardDescribeCatalogV2Cache(standardDescribeCatalogV2EntryForResolvedIndexEntry)
		entry, ok, err := cache.entryForName("CareProgram")
		if err != nil || !ok || entry.Definition.APIName != "CareProgram" {
			b.Fatalf("CareProgram lookup: ok=%v err=%v", ok, err)
		}
	}
}

func BenchmarkStandardDescribeCatalogV2CareProgramWarm(b *testing.B) {
	cache := newStandardDescribeCatalogV2Cache(standardDescribeCatalogV2EntryForResolvedIndexEntry)
	if _, ok, err := cache.entryForName("CareProgram"); err != nil || !ok {
		b.Fatalf("warmup CareProgram: ok=%v err=%v", ok, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, ok, err := cache.entryForName("CareProgram"); err != nil || !ok {
			b.Fatalf("CareProgram lookup: ok=%v err=%v", ok, err)
		}
	}
}

func BenchmarkStandardDescribeCatalogV2Rotating100(b *testing.B) {
	names := make([]string, 0, 100)
	for _, entry := range standardDescribeCatalogV2Index {
		if entry.Name != "Account" {
			names = append(names, entry.Name)
		}
		if len(names) == 100 {
			break
		}
	}
	cache := newStandardDescribeCatalogV2Cache(standardDescribeCatalogV2EntryForResolvedIndexEntry)
	for _, name := range names {
		if _, ok, err := cache.entryForName(name); err != nil || !ok {
			b.Fatalf("warmup %s: ok=%v err=%v", name, ok, err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, ok, err := cache.entryForName(names[index%len(names)]); err != nil || !ok {
			b.Fatalf("%s lookup: ok=%v err=%v", names[index%len(names)], ok, err)
		}
	}
}

func BenchmarkStandardDescribeCatalogV2Parallel(b *testing.B) {
	names := []string{"Account", "CareProgram", "Contact", "Opportunity", "User"}
	cache := newStandardDescribeCatalogV2Cache(standardDescribeCatalogV2EntryForResolvedIndexEntry)
	for _, name := range names {
		if _, ok, err := cache.entryForName(name); err != nil || !ok {
			b.Fatalf("warmup %s: ok=%v err=%v", name, ok, err)
		}
	}
	var next atomic.Uint64
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(parallel *testing.PB) {
		for parallel.Next() {
			name := names[next.Add(1)%uint64(len(names))]
			if _, ok, err := cache.entryForName(name); err != nil || !ok {
				b.Fatalf("%s lookup: ok=%v err=%v", name, ok, err)
			}
		}
	})
}

func BenchmarkStandardDescribeCatalogV2NamesOnlyCanonicalResolution(b *testing.B) {
	names := []string{"Account", " careprogram ", "CONTACT", "Opportunity", "user"}
	for _, name := range names {
		if _, ok := standardDescribeCatalogCanonicalName(name); !ok {
			b.Fatalf("warmup canonical name lookup failed for %q", name)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, ok := standardDescribeCatalogCanonicalName(names[index%len(names)]); !ok {
			b.Fatalf("canonical name lookup failed for %q", names[index%len(names)])
		}
	}
}
