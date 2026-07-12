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
		entry, ok, err := standardDescribeCatalogV2EntryForName("CareProgram")
		if err != nil || !ok || entry.Definition.APIName != "CareProgram" {
			b.Fatalf("CareProgram lookup: ok=%v err=%v", ok, err)
		}
	}
}

func BenchmarkStandardDescribeCatalogV2CareProgramWarm(b *testing.B) {
	if _, ok, err := standardDescribeCatalogV2EntryForName("CareProgram"); err != nil || !ok {
		b.Fatalf("warmup CareProgram: ok=%v err=%v", ok, err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, ok, err := standardDescribeCatalogV2EntryForName("CareProgram"); err != nil || !ok {
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
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		if _, ok, err := standardDescribeCatalogV2EntryForName(names[index%len(names)]); err != nil || !ok {
			b.Fatalf("%s lookup: ok=%v err=%v", names[index%len(names)], ok, err)
		}
	}
}

func BenchmarkStandardDescribeCatalogV2Parallel(b *testing.B) {
	var next atomic.Uint64
	b.ReportAllocs()
	b.RunParallel(func(parallel *testing.PB) {
		for parallel.Next() {
			name := standardDescribeCatalogV2Index[next.Add(1)%uint64(len(standardDescribeCatalogV2Index))].Name
			if _, ok, err := standardDescribeCatalogV2EntryForName(name); err != nil || !ok {
				b.Fatalf("%s lookup: ok=%v err=%v", name, ok, err)
			}
		}
	})
}
