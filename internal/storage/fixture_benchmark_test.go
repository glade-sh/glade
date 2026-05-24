package storage

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"
)

func BenchmarkFixtureSeedAndExport(b *testing.B) {
	fixture := benchmarkFixture(500)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		org := benchmarkFixtureOrg()
		if err := ApplyFixture(&org, fixture); err != nil {
			b.Fatal(err)
		}
		var out bytes.Buffer
		if err := WriteFixture(&out, FixtureFromOrg(org)); err != nil {
			b.Fatal(err)
		}
		if out.Len() == 0 {
			b.Fatal("empty fixture export")
		}
	}
}

func BenchmarkSQLiteStoreSaveAndLoadLargeFixture(b *testing.B) {
	fixture := benchmarkFixture(5000)
	baseDir := b.TempDir()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		store, err := OpenSQLite(filepath.Join(baseDir, fmt.Sprintf("glade-%d.db", i)))
		if err != nil {
			b.Fatal(err)
		}
		org := benchmarkFixtureOrg()
		if err := ApplyFixture(&org, fixture); err != nil {
			_ = store.Close()
			b.Fatal(err)
		}
		if err := store.Save(org); err != nil {
			_ = store.Close()
			b.Fatal(err)
		}
		loaded, err := store.Load()
		if err != nil {
			_ = store.Close()
			b.Fatal(err)
		}
		if len(loaded.Objects["Account"].Records) != 5000 {
			_ = store.Close()
			b.Fatalf("loaded Account records = %d", len(loaded.Objects["Account"].Records))
		}
		if err := store.Close(); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkFixtureOrg() OrgState {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]Field{
				"Name": {APIName: "Name", Type: FieldString},
			},
		},
		Records: make(map[ID]Record),
	}
	return org
}

func benchmarkFixture(records int) Fixture {
	fixture := Fixture{Version: FixtureVersion}
	object := FixtureObject{Name: "Account", Records: make([]FixtureRecord, 0, records)}
	for i := 0; i < records; i++ {
		object.Records = append(object.Records, FixtureRecord{
			Alias: fmt.Sprintf("account-%03d", i),
			Fields: map[string]Value{
				"Name": StringValue(fmt.Sprintf("Account %03d", i)),
			},
		})
	}
	fixture.Objects = append(fixture.Objects, object)
	return fixture
}
