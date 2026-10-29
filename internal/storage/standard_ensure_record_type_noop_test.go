package storage

import (
	"fmt"
	"testing"
)

func TestStandardEnsureSeededRecordTypesKeepSchemaGeneration(t *testing.T) {
	for _, name := range []string{"Account", "Task", "Case"} {
		t.Run(name, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, name)
			state := org.Objects[name]
			state.Definition = state.Definition.Clone()
			state.Definition.RecordTypes = []RecordTypeInfo{{DeveloperName: "Owned", Name: "Owned", Active: true, Available: true}}
			org.Objects[name] = state
			EnsureStandardObject(&org, name)
			org.RuntimeSchemaStamp = "trusted"
			generation := org.RuntimeGenerationID()
			EnsureStandardObject(&org, name)
			if org.RuntimeSchemaStamp != "trusted" || org.RuntimeGenerationID() != generation {
				t.Fatal("re-ensuring an already seeded record type discarded the schema generation")
			}
			// A record type edit, even within the same slice, must not match the
			// earlier no-op proof.
			state = org.Objects[name]
			state.Definition.RecordTypes[0].Name = "Changed"
			org.Objects[name] = state
			EnsureStandardObject(&org, name)
			if org.RuntimeSchemaStamp != "" || org.RuntimeGenerationID() == generation {
				t.Fatal("changed record type kept the schema generation")
			}
		})
	}
}

func TestStandardEnsureMissingRecordTypeSchemaStillInvalidates(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "Account")
	state := org.Objects["Account"]
	state.Definition.RecordTypes = []RecordTypeInfo{{DeveloperName: "Owned", Name: "Owned"}}
	org.Objects["Account"] = state
	EnsureStandardObject(&org, "Account")
	org.RuntimeSchemaStamp = "trusted"
	delete(org.Objects, "RecordType")
	EnsureStandardObject(&org, "Account")
	if org.RuntimeSchemaStamp != "" {
		t.Fatal("recreated RecordType schema kept an old generation")
	}
}

func TestStandardEnsureTemplateProofRejectsUnseededRecordTypes(t *testing.T) {
	for _, route := range []string{"missing_id", "missing_row", "conflicting_row", "noncanonical_row"} {
		t.Run(route, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, "Account")
			state := org.Objects["Account"]
			state.Definition.RecordTypes = []RecordTypeInfo{{DeveloperName: "Owned", Name: "Owned"}}
			org.Objects["Account"] = state
			EnsureStandardObject(&org, "Account")
			state = org.Objects["Account"]
			state.Definition = state.Definition.Clone()
			id := state.Definition.RecordTypes[0].ID
			switch route {
			case "missing_id":
				state.Definition.RecordTypes[0].ID = ""
			case "missing_row":
				table := org.Objects["RecordType"]
				delete(table.Records, id)
			case "noncanonical_row":
				table := org.Objects["RecordType"]
				row := table.Records[id]
				row.Fields["sobjecttype"] = row.Fields["SobjectType"]
				delete(row.Fields, "SobjectType")
				table.Records[id] = row
			case "conflicting_row":
				table := org.Objects["RecordType"]
				row := table.Records[id]
				row.Fields["DeveloperName"] = StringValue("Other")
				table.Records[id] = row
			}
			org.Objects["Account"] = state
			org.RuntimeSchemaStamp = "trusted"
			PrimeStandardObjectEnsureNoops(&org)
			EnsureStandardObject(&org, "Account")
			if org.RuntimeSchemaStamp != "" {
				t.Fatal("template priming trusted record types that still needed seeding")
			}
		})
	}
}

func TestStandardEnsureRecordTypeProofSurvivesSiblingClones(t *testing.T) {
	for _, name := range []string{"Account", "Task", "Case"} {
		t.Run(name, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, name)
			org.RuntimeSchemaStamp = "trusted"
			PrimeStandardObjectEnsureNoops(&org)
			generation := org.RuntimeGenerationID()
			for range 3 {
				clone := org.CloneRuntimeFrozenShared()
				for range 2 {
					EnsureStandardObject(&clone, name)
				}
				if clone.RuntimeGenerationID() != generation || clone.RuntimeSchemaStamp != "trusted" {
					t.Fatal("a sibling clone replaced the shared no-op proof")
				}
			}
		})
	}
}

func TestStandardEnsureRepairsIncompleteSeedRows(t *testing.T) {
	for _, route := range []string{"missing_name", "missing_active", "missing_description", "missing_system_stamp"} {
		t.Run(route, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, "Account")
			account := org.Objects["Account"]
			account.Definition = account.Definition.Clone()
			account.Definition.RecordTypes = []RecordTypeInfo{{DeveloperName: "Owned", Name: "Owned"}}
			org.Objects["Account"] = account
			EnsureStandardObject(&org, "Account")
			id := org.Objects["Account"].Definition.RecordTypes[0].ID
			table := org.Objects["RecordType"]
			row := table.Records[id].Clone()
			switch route {
			case "missing_name":
				delete(row.Fields, "Name")
			case "missing_active":
				delete(row.Fields, "IsActive")
			case "missing_description":
				delete(row.Fields, "Description")
			case "missing_system_stamp":
				row.System.CreatedDate = ""
			}
			table.Records[id] = row
			org.Objects["RecordType"] = table
			org.RuntimeSchemaStamp = "trusted"
			PrimeStandardObjectEnsureNoops(&org)
			EnsureStandardObject(&org, "Account")
			row = org.Objects["RecordType"].Records[id]
			for _, field := range []string{"Name", "IsActive", "Description"} {
				if _, ok := row.Fields[field]; !ok {
					t.Fatalf("missing seed field %s was not repaired", field)
				}
			}
			if row.System.CreatedDate == "" {
				t.Fatal("missing seed system timestamp was not repaired")
			}
			if org.RuntimeSchemaStamp != "" {
				t.Fatal("a seed repair retained the old schema stamp")
			}
		})
	}
}

func TestStandardEnsureRecordTypeProofRequiresLookup(t *testing.T) {
	for _, name := range []string{"Cedar__c", "Maple__c", "Pine__c"} {
		for _, beforePrime := range []bool{false, true} {
			t.Run(name+fmt.Sprint(beforePrime), func(t *testing.T) {
				org := NewOrgState()
				EnsureStandardObject(&org, name)
				state := org.Objects[name]
				state.Definition.RecordTypes = []RecordTypeInfo{{DeveloperName: "Owned", Name: "Owned"}}
				org.Objects[name] = state
				EnsureStandardObject(&org, name)
				org.RuntimeSchemaStamp = "trusted"
				if !beforePrime {
					PrimeStandardObjectEnsureNoops(&org)
				}
				state = org.Objects[name]
				delete(state.Definition.Fields, "RecordTypeId")
				org.Objects[name] = state
				if beforePrime {
					PrimeStandardObjectEnsureNoops(&org)
				}
				EnsureStandardObject(&org, name)
				if _, ok := ResolveFieldName(org.Objects[name].Definition, "", "RecordTypeId"); !ok {
					t.Fatal("prepared record-type proof suppressed lookup repair")
				}
				if org.RuntimeSchemaStamp != "" {
					t.Fatal("lookup repair retained the old schema stamp")
				}
			})
		}
	}
}
