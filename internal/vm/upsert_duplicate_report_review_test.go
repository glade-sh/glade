package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestUpsertDuplicateReportCommittedNative(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/dml_policies"
	source, err := os.ReadFile("../apextest/testdata/conformance/upsert_duplicate_review/report.apex")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(filepath.Join(fixture, "project"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
			storage.EnsureStandardObject(&org, "Contact")
			contact := org.Objects["Contact"]
			contact.Definition.Fields["RouteKey__c"] = storage.Field{APIName: "RouteKey__c", Type: storage.FieldString, ExternalID: true}
			org.Objects["Contact"] = contact
			installDMLPolicyAutomation(t, &org, loaded)
			contact = org.Objects["Contact"]
			foundReport := false
			for _, rule := range contact.Definition.DuplicateRules {
				if strings.EqualFold(rule.Name, "Synthetic_Block") {
					rule.Name = "Synthetic_Report"
					rule.ActionOnInsert = "Allow"
					rule.OperationsOnInsert = []string{"Report"}
					rule.Filter.Items = append([]storage.DuplicateRuleFilterItem(nil), rule.Filter.Items...)
					rule.Filter.Items[0].Value = "Report"
					rule.SortOrder = 5
					contact.Definition.DuplicateRules = append(contact.Definition.DuplicateRules, rule)
					foundReport = true
					break
				}
			}
			if !foundReport {
				t.Fatal("missing synthetic blocking rule")
			}
			org.Objects["Contact"] = contact
			program, err := CompileAnonymousWithOptions(string(source), CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			sets := machine.Org.Objects["DuplicateRecordSet"].Records
			items := machine.Org.Objects["DuplicateRecordItem"].Records
			if len(sets) != 1 || len(items) != 2 {
				t.Fatalf("native committed report: one set, two items; got %d sets, %d items", len(sets), len(items))
			}
		})
	}
}

// The uncaptured list route retains shipping behavior.
func TestUpsertDuplicateReportListPreservesShipping(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/dml_policies"
	source, err := os.ReadFile("../apextest/testdata/conformance/upsert_duplicate_review/report.apex")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(filepath.Join(fixture, "project"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
			storage.EnsureStandardObject(&org, "Contact")
			contact := org.Objects["Contact"]
			contact.Definition.Fields["RouteKey__c"] = storage.Field{APIName: "RouteKey__c", Type: storage.FieldString, ExternalID: true}
			org.Objects["Contact"] = contact
			installDMLPolicyAutomation(t, &org, loaded)
			contact = org.Objects["Contact"]
			foundReport := false
			for _, rule := range contact.Definition.DuplicateRules {
				if strings.EqualFold(rule.Name, "Synthetic_Block") {
					rule.Name = "Synthetic_Report"
					rule.ActionOnInsert = "Allow"
					rule.OperationsOnInsert = []string{"Report"}
					rule.Filter.Items = append([]storage.DuplicateRuleFilterItem(nil), rule.Filter.Items...)
					rule.Filter.Items[0].Value = "Report"
					rule.SortOrder = 5
					contact.Definition.DuplicateRules = append(contact.Definition.DuplicateRules, rule)
					foundReport = true
					break
				}
			}
			if !foundReport {
				t.Fatal("missing synthetic blocking rule")
			}
			org.Objects["Contact"] = contact
			listSource := strings.Replace(string(source), "upsert r RouteKey__c;", "List<Contact> rows=new List<Contact>{r}; upsert rows;", 1)
			program, err := CompileAnonymousWithOptions(listSource, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			sets := machine.Org.Objects["DuplicateRecordSet"].Records
			items := machine.Org.Objects["DuplicateRecordItem"].Records
			if len(sets) != 0 || len(items) != 0 {
				t.Fatalf("shipping list path retained no committed report; got %d sets, %d items", len(sets), len(items))
			}
		})
	}
}

// No report observation exists for an implicit Id upsert.
func TestUpsertDuplicateReportImplicitIDPreservesShipping(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/dml_policies"
	source, err := os.ReadFile("../apextest/testdata/conformance/upsert_duplicate_review/report.apex")
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(filepath.Join(fixture, "project"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
			storage.EnsureStandardObject(&org, "Contact")
			contact := org.Objects["Contact"]
			contact.Definition.Fields["RouteKey__c"] = storage.Field{APIName: "RouteKey__c", Type: storage.FieldString, ExternalID: true}
			org.Objects["Contact"] = contact
			installDMLPolicyAutomation(t, &org, loaded)
			contact = org.Objects["Contact"]
			foundReport := false
			for _, rule := range contact.Definition.DuplicateRules {
				if strings.EqualFold(rule.Name, "Synthetic_Block") {
					rule.Name = "Synthetic_Report"
					rule.ActionOnInsert = "Allow"
					rule.OperationsOnInsert = []string{"Report"}
					rule.Filter.Items = append([]storage.DuplicateRuleFilterItem(nil), rule.Filter.Items...)
					rule.Filter.Items[0].Value = "Report"
					rule.SortOrder = 5
					contact.Definition.DuplicateRules = append(contact.Definition.DuplicateRules, rule)
					foundReport = true
					break
				}
			}
			if !foundReport {
				t.Fatal("missing synthetic blocking rule")
			}
			org.Objects["Contact"] = contact
			listSource := strings.Replace(string(source), "upsert r RouteKey__c;", "upsert r;", 1)
			program, err := CompileAnonymousWithOptions(listSource, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			sets := machine.Org.Objects["DuplicateRecordSet"].Records
			items := machine.Org.Objects["DuplicateRecordItem"].Records
			if len(sets) != 0 || len(items) != 0 {
				t.Fatalf("shipping implicit Id path retained no committed report; got %d sets, %d items", len(sets), len(items))
			}
		})
	}
}
