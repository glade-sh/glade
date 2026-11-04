package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"strings"
	"testing"
)

// The same fixture is run on both revisions; logged digests are compared outside
// the process so neither revision supplies the other's expected output.
func case334Fixture(reference string) Fixture {
	return Fixture{Version: FixtureVersion, Objects: []FixtureObject{
		{Name: "Account", Records: []FixtureRecord{{ID: "001000000000001", Alias: "parent", Fields: map[string]Value{
			"Name": StringValue("parent"), "Nested": ListValue(ListValue(StringValue("nested"))), "Removed": StringValue("ignored"),
		}, ExplicitNulls: []string{"Removed"}}}},
		{Name: "Contact", Records: []FixtureRecord{{ID: "003000000000001", Fields: map[string]Value{"LastName": StringValue("child")}, FieldRefs: map[string]string{"AccountId": reference}}}},
	}}
}

func TestFixtureEquivalenceReceipt(t *testing.T) {
	for _, tc := range []struct {
		name, reference, errorContains string
	}{
		{name: "valid", reference: "Account.parent"},
		{name: "missing", reference: "missing", errorContains: "unknown fixture alias"},
		{name: "wrong-target", reference: "Contact.child", errorContains: "cannot populate Contact.AccountId"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := case334Fixture(tc.reference)
			if tc.name == "wrong-target" {
				fixture.Objects[1].Records[0].Alias = "child"
			}
			org := fixtureRelationshipOrg()
			err := ApplyFixture(&org, fixture)
			message := ""
			if err != nil {
				message = err.Error()
			}
			if tc.errorContains == "" && err != nil || tc.errorContains != "" && !strings.Contains(message, tc.errorContains) {
				t.Fatalf("error %q, want containing %q", message, tc.errorContains)
			}
			if tc.name == "valid" {
				parent := org.Objects["Account"].Records["001000000000001"]
				child := org.Objects["Contact"].Records["003000000000001"]
				if !parent.ExplicitNulls["Removed"] || parent.Fields["Nested"].List[0].List[0].String != "nested" || child.Fields["AccountId"].ID != parent.ID {
					t.Fatalf("unexpected parent or reference: %#v %#v", parent, child)
				}
			}
			state, err := json.Marshal(org)
			if err != nil {
				t.Fatal(err)
			}
			var exported bytes.Buffer
			if err := WriteFixture(&exported, FixtureFromOrg(org)); err != nil {
				t.Fatal(err)
			}
			t.Logf("equivalence case=%s state=%x export=%x error=%q", tc.name, sha256.Sum256(state), sha256.Sum256(exported.Bytes()), message)
		})
	}
}

func BenchmarkFixtureEquivalenceSeed(b *testing.B) {
	fixture := case334Fixture("Account.parent")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		org := fixtureRelationshipOrg()
		if err := ApplyFixture(&org, fixture); err != nil {
			b.Fatal(err)
		}
	}
}
