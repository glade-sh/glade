package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// record_type_visibility rows: a standard object with a record type and no
// default rejects an explicit null RecordTypeId in SYSTEM_MODE and inserts it
// in USER_MODE; the Master token always fails; USER_MODE rejects a record type
// the running user's profile does not see, on insert and on update.
func TestApexRecordTypeVisibilityOnStandardObject(t *testing.T) {
	const hidden storage.ID = "012000000000HID"
	newOrg := func() storage.OrgState {
		org := testOrg()
		account := org.Objects["Account"]
		account.Definition.RecordTypes = append(account.Definition.RecordTypes, storage.RecordTypeInfo{ID: hidden, DeveloperName: "Hidden", Name: "Hidden", Active: true, Unavailable: true})
		org.Objects["Account"] = account
		recordTypes := org.Objects["RecordType"]
		recordTypes.Records[hidden] = storage.Record{Object: "RecordType", ID: hidden, Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("Hidden"), "SobjectType": storage.StringValue("Account")}}
		org.Objects["RecordType"] = recordTypes
		return org
	}
	for _, row := range []struct {
		mode, shape, code string
	}{
		{"SYSTEM_MODE", "null", "INVALID_CROSS_REFERENCE_KEY"},
		{"USER_MODE", "null", ""},
		{"SYSTEM_MODE", "master", "INVALID_CROSS_REFERENCE_KEY"},
		{"USER_MODE", "master", "INVALID_CROSS_REFERENCE_KEY"},
		{"SYSTEM_MODE", "hidden", ""},
		{"USER_MODE", "hidden", "INVALID_CROSS_REFERENCE_KEY"},
		{"", "null", ""},
		{"", "hidden", ""},
	} {
		t.Run(row.mode+"/"+row.shape, func(t *testing.T) {
			org := newOrg()
			record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Row")}}
			switch row.shape {
			case "null":
				record.ExplicitNulls = map[string]bool{"RecordTypeId": true}
			case "master":
				record.Fields["RecordTypeId"] = storage.IDValue("012000000000000AAA")
			case "hidden":
				record.Fields["RecordTypeId"] = storage.IDValue(hidden)
			}
			engine := NewEngine(&org)
			engine.Options.ApexRecordTypeInsertMode = row.mode
			results := engine.Insert([]storage.Record{record})
			if row.code == "" {
				if len(results) != 1 || !results[0].Success {
					t.Fatalf("insert = %#v", results)
				}
				return
			}
			if len(results) != 1 || results[0].Success || results[0].StatusCode != row.code {
				t.Fatalf("insert = %#v; want %s", results, row.code)
			}
		})
	}
	for _, mode := range []string{"SYSTEM_MODE", "USER_MODE", ""} {
		t.Run("update/"+mode, func(t *testing.T) {
			org := newOrg()
			engine := NewEngine(&org)
			inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Row")}}})
			if len(inserted) != 1 || !inserted[0].Success {
				t.Fatalf("seed insert = %#v", inserted)
			}
			engine.Options.ApexRecordTypeUpdateMode = mode
			results := engine.Update([]storage.Record{{Object: "Account", ID: inserted[0].ID, Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(hidden)}}})
			if mode != "USER_MODE" {
				if len(results) != 1 || !results[0].Success {
					t.Fatalf("update = %#v", results)
				}
				return
			}
			if len(results) != 1 || results[0].Success || results[0].StatusCode != "INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY" || results[0].Error != "invalid record type" {
				t.Fatalf("update = %#v", results)
			}
		})
	}
}
