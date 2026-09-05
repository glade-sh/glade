package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRuntimePatchAmbientFingerprintPreservesCloneIdentity(t *testing.T) {
	record := storage.Record{
		ID: "001000000000001", Object: "Account",
		Fields:   map[string]storage.Value{"Name": storage.StringValue("Fixture")},
		Children: map[string][]storage.Record{"Contacts": nil, "Empty": {}},
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{
		"Account": {
			Definition: storage.ObjectDefinition{
				APIName: "Account",
				Fields: map[string]storage.Field{"Name": {
					APIName: "Name", Type: storage.FieldString, ReferenceTo: []string{},
					PicklistValueSettings: []storage.PicklistSetting{{ValueName: "A", ControllingFieldValues: []string{}}},
				}},
				Relations: []storage.Relationship{{Field: "OwnerId", ParentObjects: []string{}}},
				Indexes:   []storage.IndexDefinition{{Name: "Empty", Fields: []string{}}},
			},
			Records: map[storage.ID]storage.Record{record.ID: record},
			Indexes: map[string]storage.IndexSet{"Empty": {
				Definition: storage.IndexDefinition{Fields: []string{}},
				Entries:    map[string][]storage.ID{"nil": nil, "empty": {}},
			}},
		},
		"ApexClass": {Records: map[storage.ID]storage.Record{"01p000000000001": record}},
	}, Transactions: []storage.TransactionFrame{{Mutations: []storage.Mutation{{Before: &record, After: &record}}}}}
	for _, tc := range []struct {
		name string
		org  storage.OrgState
	}{
		{"nil objects", storage.OrgState{}},
		{"empty objects", storage.OrgState{Objects: map[string]storage.ObjectState{}}},
		{"nested empty collections", org},
		{"standard org", standardApexTestOrg()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, err := json.Marshal(tc.org)
			if err != nil {
				t.Fatal(err)
			}
			for _, pages := range [][]string{nil, {}, {"Second", "First"}} {
				want, wantOK := legacyRuntimePatchAmbientFingerprint(tc.org, pages)
				got, ok := runtimePatchAmbientFingerprint(tc.org, pages)
				if !wantOK || !ok || got != want {
					t.Fatalf("fingerprint = %q, %v; legacy = %q, %v", got, ok, want, wantOK)
				}
			}
			after, err := json.Marshal(tc.org)
			if err != nil || string(before) != string(after) {
				t.Fatal("fingerprinting changed the source org")
			}
		})
	}
	// Field definitions remain part of the authority.
	object := org.Objects["Account"]
	field := object.Definition.Fields["Name"]
	before, _ := runtimePatchAmbientFingerprint(org, nil)
	field.Label = "Changed"
	object.Definition.Fields["Name"] = field
	if after, ok := runtimePatchAmbientFingerprint(org, nil); !ok || after == before {
		t.Fatal("field mutation did not change the fingerprint")
	}
}

func legacyRuntimePatchAmbientFingerprint(org storage.OrgState, pageNames []string) (string, bool) {
	ambientOrg := org.Clone()
	delete(ambientOrg.Objects, "ApexClass")
	payload := struct {
		Org       storage.OrgState `json:"org"`
		PageNames []string         `json:"pageNames,omitempty"`
	}{ambientOrg, append([]string(nil), pageNames...)}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), true
}

func BenchmarkRuntimePatchAmbientFingerprint(b *testing.B) {
	org := standardApexTestOrg()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, ok := runtimePatchAmbientFingerprint(org, nil); !ok {
			b.Fatal("fingerprint failed")
		}
	}
}
