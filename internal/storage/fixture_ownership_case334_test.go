package storage

import (
	"bytes"
	"testing"
)

func TestFixtureRecordOwnershipAndRoundTrip(t *testing.T) {
	fixture := Fixture{Version: FixtureVersion, Objects: []FixtureObject{{Name: "Account", Records: []FixtureRecord{
		{ID: "001000000000001", Fields: map[string]Value{"Name": StringValue("first"), "Nested": ListValue(ListValue(StringValue("original"))), "NullField": StringValue("removed")}, ExplicitNulls: []string{"NullField"}},
		{ID: "001000000000002", Fields: map[string]Value{"Name": StringValue("second")}},
	}}}}
	org := benchmarkFixtureOrg()
	if err := ApplyFixture(&org, fixture); err != nil {
		t.Fatal(err)
	}
	var before bytes.Buffer
	if err := WriteFixture(&before, FixtureFromOrg(org)); err != nil {
		t.Fatal(err)
	}
	input := fixture.Objects[0].Records[0].Fields["Nested"]
	input.List[0].List[0] = StringValue("input changed")
	record := org.Objects["Account"].Records["001000000000001"]
	if got := record.Fields["Nested"].List[0].List[0].String; got != "original" {
		t.Fatalf("stored value changed with fixture: %q", got)
	}
	if !record.ExplicitNulls["NullField"] {
		t.Fatal("explicit null lost")
	}
	if _, ok := record.Fields["NullField"]; ok {
		t.Fatal("explicit null retained as field")
	}
	var after bytes.Buffer
	if err := WriteFixture(&after, FixtureFromOrg(org)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before.Bytes(), after.Bytes()) {
		t.Fatal("fixture input mutation changed exported org")
	}
	record.Fields["Nested"].List[0].List[0] = StringValue("stored changed")
	if got := input.List[0].List[0].String; got != "input changed" {
		t.Fatalf("fixture value changed with stored record: %q", got)
	}
	if got := org.Objects["Account"].Records["001000000000002"].Fields["Name"].String; got != "second" {
		t.Fatalf("other record changed: %q", got)
	}
}
