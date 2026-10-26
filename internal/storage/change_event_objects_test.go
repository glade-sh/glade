package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChangeEventObjectNativeSchemaAnonymousAndIsTestAPIRoutes(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				raw, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance/change_event_objects/native", "api"+api, route, "org.tsv"))
				if err != nil {
					t.Fatal(err)
				}
				org := OrgState{APIVersion: api + ".0", Objects: map[string]ObjectState{
					"Parcel__c": {Definition: ObjectDefinition{APIName: "Parcel__c", Fields: map[string]Field{
						"Marker__c": {APIName: "Marker__c", Type: FieldString},
					}}},
				}}
				for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
					cells := strings.SplitN(line, "\t", 3)
					var want map[string]any
					if len(cells) != 3 || json.Unmarshal([]byte(cells[2]), &want) != nil {
						t.Fatal("invalid native row")
					}
					name := want["name"].(string)
					if !EnsureChangeEventObject(&org, name) {
						t.Fatalf("%s was not installed", name)
					}
					state := org.Objects[name]
					for field, value := range want {
						if field == "name" {
							continue
						}
						_, exists := state.Definition.Fields[field]
						if exists != value.(bool) {
							t.Errorf("%s.%s present=%v; native=%v", name, field, exists, value)
						}
					}
					state.Records["event"] = Record{ID: "event", Object: name}
					lowered := strings.ToLower(name)
					if EnsureChangeEventObject(&org, lowered) || len(org.Objects[name].Records) != 1 {
						t.Fatal("repeat installation changed event state")
					}
				}
				custom := org.Objects["Parcel__c"].Definition
				if len(custom.Fields) != 1 {
					t.Fatal("event schema mutated the base definition")
				}
				if EnsureChangeEventObject(&org, "Parcel__c") || EnsureChangeEventObject(&org, "Absent__ChangeEvent") {
					t.Fatal("non-event or absent base installed")
				}
			})
		}
	}
}

type changeEventDescribeRow struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

func readChangeEventDescribeRows(t *testing.T, api string) map[string]map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../apextest/testdata/conformance/change_event_describe/native", "api"+api, "anonymous", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 3)
		var row changeEventDescribeRow
		if len(cells) != 3 || json.Unmarshal([]byte(cells[2]), &row) != nil || len(row.Fields) == 0 {
			t.Fatal("invalid native row")
		}
		fields := map[string]bool{}
		for _, field := range row.Fields {
			fields[field] = true
		}
		rows[row.Name] = fields
	}
	return rows
}

// The native describe rows pin the event field rule: the event carries the
// base fields minus those its row lacks, plus those only its row has. The
// custom event matches its row exactly.
func TestChangeEventObjectFieldSetFollowsNativeDescribeAPIRoutes(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			native := readChangeEventDescribeRows(t, api)
			custom := ObjectDefinition{APIName: "Parcel__c", Fields: map[string]Field{
				"Name":      {APIName: "Name", Type: FieldString},
				"Acct__c":   {APIName: "Acct__c", Type: FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Acct__r", ChildRelationshipName: "Parcels"},
				"Weight__c": {APIName: "Weight__c", Type: FieldDecimal, Precision: 10, Scale: 2},
				"Fx__c":     {APIName: "Fx__c", Type: FieldDecimal, Formula: "Weight__c * 2", Precision: 18, Scale: 2},
				"Kind__c":   {APIName: "Kind__c", Type: FieldPicklist},
				"Flag__c":   {APIName: "Flag__c", Type: FieldBoolean},
				"Note__c":   {APIName: "Note__c", Type: FieldString, Length: 50},
			}}
			EnsureStandardObjectFields(&custom)
			org := OrgState{APIVersion: api + ".0", Objects: map[string]ObjectState{"Parcel__c": {Definition: custom}}}
			for _, pair := range [][2]string{{"Account", "AccountChangeEvent"}, {"Parcel__c", "Parcel__ChangeEvent"}} {
				base, event := pair[0], pair[1]
				if !EnsureChangeEventObject(&org, event) {
					t.Fatalf("%s was not installed", event)
				}
				var baseDefinition ObjectDefinition
				if state, ok := org.Objects[base]; ok {
					baseDefinition = state.Definition
				} else {
					baseDefinition, _ = StandardObjectDefinition(base)
					EnsureStandardObjectFields(&baseDefinition)
				}
				want := map[string]bool{}
				for key := range baseDefinition.Fields {
					want[strings.ToLower(key)] = true
				}
				for field := range native[base] {
					if !native[event][field] {
						delete(want, field)
					}
				}
				for field := range native[event] {
					if !native[base][field] {
						want[field] = true
					}
				}
				if base == "Parcel__c" {
					want = native[event]
				} else {
					applyChangeEventFollowupNativeExclusions(t, api+".0", want)
				}
				got := map[string]bool{}
				for key, field := range org.Objects[event].Definition.Fields {
					got[strings.ToLower(key)] = true
					if field.ChildRelationshipName != "" {
						t.Errorf("%s.%s carries child relationship %s", event, key, field.ChildRelationshipName)
					}
				}
				for field := range got {
					if !want[field] {
						t.Errorf("%s.%s installed; the native rule excludes it", event, field)
					}
				}
				for field := range want {
					if !got[field] {
						t.Errorf("%s.%s required by the native rule; not installed", event, field)
					}
				}
			}
		})
	}
}
