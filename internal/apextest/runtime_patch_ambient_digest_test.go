package apextest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

type recordingHash struct {
	bytes.Buffer
}

func (*recordingHash) Sum(b []byte) []byte { return b }
func (*recordingHash) Size() int           { return 0 }
func (*recordingHash) BlockSize() int      { return 1 }

// TestRuntimePatchAmbientJSONMatchesMarshal pins the streamed digest input to
// json.Marshal of the same payload, byte for byte.
func TestRuntimePatchAmbientJSONMatchesMarshal(t *testing.T) {
	record := storage.Record{
		ID: "001000000000001", Object: "Account",
		Fields: map[string]storage.Value{"Name": storage.StringValue("<a & b> \xff")},
	}
	odd := storage.OrgState{
		OrgID: "00D000000000001", APIVersion: "67.0", Namespace: "ns", DomainURL: "https://example.my.salesforce.com",
		Metadata:    storage.MetadataRegistry{Labels: []storage.LabelMetadata{{Name: "objects", Value: `"objects":null`}}},
		IDSequences: map[string]uint64{"Account": 3},
		Objects: map[string]storage.ObjectState{
			"Account":      {Definition: storage.ObjectDefinition{APIName: "Account", Label: "<Account>"}, Records: map[storage.ID]storage.Record{record.ID: record}},
			"<html>&":      {Definition: storage.ObjectDefinition{APIName: "x"}},
			"Café":         {},
			"bad\xffkey":   {Records: map[storage.ID]storage.Record{}},
			"ns__Thing__c": {Indexes: map[string]storage.IndexSet{}},
			"":             {},
		},
		Transactions: []storage.TransactionFrame{{Mutations: []storage.Mutation{{Before: &record, After: &record}}}},
	}
	orgs := map[string]storage.OrgState{
		"nil objects":   {},
		"empty objects": {Objects: map[string]storage.ObjectState{}},
		"odd keys":      odd,
		"standard org":  standardApexTestOrg(),
	}
	for name, org := range orgs {
		for _, pages := range [][]string{nil, {}, {"Second", "First<"}} {
			payload := runtimePatchAmbientPayload{Org: org, PageNames: pages}
			want, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			var got recordingHash
			if !writeRuntimePatchAmbientJSON(&got, payload) {
				t.Fatalf("%s: streamed encoding failed", name)
			}
			if !bytes.Equal(got.Bytes(), want) {
				t.Fatalf("%s pages %v: streamed encoding differs from json.Marshal", name, pages)
			}
			digest, ok := runtimePatchAmbientDigest(payload)
			if wantDigest := sha256.Sum256(want); !ok || digest != hex.EncodeToString(wantDigest[:]) {
				t.Fatalf("%s: digest = %q, %v", name, digest, ok)
			}
		}
	}
}
