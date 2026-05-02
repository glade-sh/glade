package storage

import (
	"bytes"
	"fmt"
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
