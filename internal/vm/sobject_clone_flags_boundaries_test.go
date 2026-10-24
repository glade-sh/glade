package vm

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestCloneAuditDMLFilterCallerAndMetadataBoundaries(t *testing.T) {
	for _, cell := range []struct {
		name, object, category       string
		kind                         storage.FieldType
		writable, assigned, ordinary bool
		filtered                     bool
	}{
		{name: "jsonTimestamp", kind: storage.FieldDateTime, filtered: true},
		{name: "ordinaryTimestamp", kind: storage.FieldDateTime, ordinary: true},
		{name: "writableMetadata", kind: storage.FieldDateTime, writable: true},
		{name: "userAssignment", kind: storage.FieldDateTime, assigned: true},
		{name: "nonTimestampMetadata", kind: storage.FieldString},
		{name: "unknownCategory", kind: storage.FieldDateTime, category: "unknown"},
		{name: "customSetting", kind: storage.FieldDateTime, category: "setting"},
		{name: "standardObject", kind: storage.FieldDateTime, object: "Account"},
	} {
		t.Run(cell.name, func(t *testing.T) {
			org := storage.NewOrgState()
			object := cell.object
			if object == "" {
				object = "AuditBoundary__c"
			}
			metadata := map[string]string{"kind": "customObject", "nameFieldType": "Text"}
			if cell.category == "unknown" {
				metadata = nil
			}
			if cell.category == "setting" {
				metadata["customSettingsType"] = "List"
			}
			org.Objects[object] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: object, Metadata: metadata, Fields: map[string]storage.Field{
				"CreatedDate": {APIName: "CreatedDate", Type: cell.kind, Createable: storage.BoolFlag(cell.writable), Updateable: storage.BoolFlag(false)},
			}}}
			machine := New(nil)
			machine.SetOrg(&org)
			row := Object(object)
			row.Fields["CreatedDate"] = platformScalar("Datetime", "2001-02-03T04:05:06.000Z")
			if !cell.ordinary {
				machine.markJSONDeserializedSObjectFields(&row, row.Type)
			}
			if cell.assigned {
				markUserSetSObjectField(&row, "CreatedDate")
			}
			records := []storage.Record{{Object: row.Type, Fields: map[string]storage.Value{"CreatedDate": storage.DateTimeValue("2001-02-03T04:05:06.000Z"), "Editable__c": storage.StringValue("retained")}, ExplicitNulls: map[string]bool{"EditableNull__c": true}}}
			// Keep this unit regression buildable before the optional audit filter
			// is installed. An absent filter leaves the copied payload intact.
			if filter, ok := any(machine).(interface {
				filterSObjectAuditDMLFields([]storage.Record, []*Value)
			}); ok {
				filter.filterSObjectAuditDMLFields(records, []*Value{&row})
			}
			_, retained := records[0].Fields["CreatedDate"]
			if retained == cell.filtered {
				t.Fatalf("timestamp retained=%t, expected filtering=%t", retained, cell.filtered)
			}
			if row.Fields["CreatedDate"].Kind == ValueNull || records[0].Fields["Editable__c"].String != "retained" || !records[0].ExplicitNulls["EditableNull__c"] {
				t.Fatal("caller or unrelated payload changed")
			}
		})
	}
}

func TestSObjectCloneFlagsNativeFixtureIntegrity(t *testing.T) {
	const fixture = "testdata/conformance/sobject_clone_flags"
	var baseline string
	for _, api := range []string{"62", "67"} {
		for _, route := range []string{"anonymous", "istest"} {
			dir := filepath.Join(fixture, "native", "api"+api, route)
			data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			if baseline == "" {
				baseline = string(data)
			} else if string(data) != baseline {
				t.Fatal("native routes differ")
			}
			if len(strings.Split(strings.TrimSpace(string(data)), "\n")) != 74 {
				t.Fatal("native rows missing")
			}
			run, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{"source_api=" + api + ".0", "context=" + route, "rows=73", "native_complete=true"} {
				if !strings.Contains(string(run), required+"\n") {
					t.Fatal("incomplete native capture: " + required)
				}
			}
			checked := 0
			for _, line := range strings.Split(string(run), "\n") {
				path, digest, ok := strings.Cut(line, " sha256=")
				if !ok {
					continue
				}
				source, err := os.ReadFile(filepath.Join(fixture, path))
				if err != nil {
					t.Fatal(err)
				}
				if strings.HasSuffix(path, ".cls-meta.xml") {
					source = []byte(strings.ReplaceAll(string(source), "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+".0</apiVersion>"))
				}
				if fmt.Sprintf("%x", sha256.Sum256(source)) != digest {
					t.Fatal("native source changed: " + path)
				}
				checked++
			}
			if checked != 9 {
				t.Fatalf("native source files=%d, want 9", checked)
			}
		}
	}
}
