package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func criteriaRuleOrg() storage.OrgState {
	org := testOrg()
	org.Objects["Case"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Case", KeyPrefix: "500", SharingModel: "Private",
		Fields: map[string]storage.Field{"Subject": {APIName: "Subject", Type: storage.FieldString}},
		CriteriaSharingRules: []storage.CriteriaSharingRule{{Name: "Subject", TargetGroup: "CriteriaTarget", AccessLevel: "Read",
			Criteria: []storage.CriteriaSharingCondition{{Field: "Subject", Operation: "equals", Value: "MATCH"}}}}}, Records: map[storage.ID]storage.Record{}}
	org.Objects["Group"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Group", KeyPrefix: "00G"}, Records: map[storage.ID]storage.Record{
		"00G000000000002": {Object: "Group", ID: "00G000000000002", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("CriteriaTarget"), "Type": storage.StringValue("Regular")}},
	}}
	org.IDSequences["Group"] = 9
	return org
}

func criteriaRuleRows(org *storage.OrgState) map[storage.ID]storage.Record {
	rows := map[storage.ID]storage.Record{}
	for id, row := range org.Objects["CaseShare"].Records {
		rows[id] = row
	}
	return rows
}

func setCriteriaCase(org *storage.OrgState, id storage.ID, subject string, deleted bool) {
	cases := org.Objects["Case"]
	cases.Records[id] = storage.Record{Object: "Case", ID: id, Fields: map[string]storage.Value{"Subject": storage.StringValue(subject)}, System: storage.SystemFields{IsDeleted: deleted}}
	org.Objects["Case"] = cases
}

// Rule rows follow the criteria at each commit: a matching record gains one row
// for the target group, an unchanged match keeps its row and id, a record that
// stops matching or is deleted loses it, and a record that never matched never
// has one. Manual rows are left alone.
func TestReconcileCriteriaSharingGrantsAndRevokes(t *testing.T) {
	org := criteriaRuleOrg()
	if err := ReconcileCriteriaSharing(&org); err != nil {
		t.Fatal(err)
	}
	if len(criteriaRuleRows(&org)) != 0 {
		t.Fatal("rows without records")
	}
	setCriteriaCase(&org, "500000000000001", "MATCH", false)
	setCriteriaCase(&org, "500000000000002", "OTHER", false)
	if err := ReconcileCriteriaSharing(&org); err != nil {
		t.Fatal(err)
	}
	rows := criteriaRuleRows(&org)
	if len(rows) != 1 {
		t.Fatalf("rows=%d, want 1: %v", len(rows), rows)
	}
	var grantID storage.ID
	for id, row := range rows {
		grantID = id
		if row.Fields["CaseId"].ID != "500000000000001" || row.Fields["UserOrGroupId"].ID != "00G000000000002" || row.Fields["RowCause"].String != "Rule" || row.Fields["CaseAccessLevel"].String != "Read" {
			t.Fatalf("wrong row %v", row)
		}
	}
	manual := org.Objects["CaseShare"]
	manual.Records["50b000000000099"] = storage.Record{Object: "CaseShare", ID: "50b000000000099", Fields: map[string]storage.Value{"CaseId": storage.IDValue("500000000000002"), "UserOrGroupId": storage.IDValue("005000000000002"), "RowCause": storage.StringValue("Manual"), "CaseAccessLevel": storage.StringValue("Read")}}
	org.Objects["CaseShare"] = manual
	if err := ReconcileCriteriaSharing(&org); err != nil {
		t.Fatal(err)
	}
	if _, ok := criteriaRuleRows(&org)[grantID]; !ok || len(criteriaRuleRows(&org)) != 2 {
		t.Fatalf("unchanged match lost its row or manual row removed: %v", criteriaRuleRows(&org))
	}
	setCriteriaCase(&org, "500000000000002", "MATCH", false)
	setCriteriaCase(&org, "500000000000001", "OTHER", false)
	if err := ReconcileCriteriaSharing(&org); err != nil {
		t.Fatal(err)
	}
	rule := map[storage.ID]bool{}
	for _, row := range criteriaRuleRows(&org) {
		if row.Fields["RowCause"].String == "Rule" {
			rule[row.Fields["CaseId"].ID] = true
		}
	}
	if len(rule) != 1 || !rule["500000000000002"] {
		t.Fatalf("update into and out: %v", rule)
	}
	setCriteriaCase(&org, "500000000000002", "MATCH", true)
	if err := ReconcileCriteriaSharing(&org); err != nil {
		t.Fatal(err)
	}
	for _, row := range criteriaRuleRows(&org) {
		if row.Fields["RowCause"].String == "Rule" {
			t.Fatalf("deleted record kept its Rule row: %v", row)
		}
	}
	// Removing the rule configuration revokes the rows it granted.
	setCriteriaCase(&org, "500000000000003", "MATCH", false)
	if err := ReconcileCriteriaSharing(&org); err != nil {
		t.Fatal(err)
	}
	cases := org.Objects["Case"]
	cases.Definition.CriteriaSharingRules = nil
	org.Objects["Case"] = cases
	if err := ReconcileCriteriaSharing(&org); err != nil {
		t.Fatal(err)
	}
	for _, row := range criteriaRuleRows(&org) {
		if row.Fields["RowCause"].String == "Rule" {
			t.Fatalf("removed rule kept its row: %v", row)
		}
	}
}

// A rule only grants on the operations it declares: notEqual matches a value
// other than the literal, every item must hold, and a rule whose group is not
// loaded grants nothing.
func TestReconcileCriteriaSharingOperations(t *testing.T) {
	org := criteriaRuleOrg()
	cases := org.Objects["Case"]
	cases.Definition.Fields["Origin"] = storage.Field{APIName: "Origin", Type: storage.FieldPicklist}
	cases.Definition.CriteriaSharingRules = []storage.CriteriaSharingRule{
		{Name: "Both", TargetGroup: "CriteriaTarget", AccessLevel: "Read", Criteria: []storage.CriteriaSharingCondition{{Field: "Subject", Operation: "notEqual", Value: "SKIP"}, {Field: "Origin", Operation: "equals", Value: "Web"}}},
		{Name: "NoGroup", TargetGroup: "Missing", AccessLevel: "Read", Criteria: []storage.CriteriaSharingCondition{{Field: "Subject", Operation: "notEqual", Value: "SKIP"}}},
	}
	for id, fields := range map[storage.ID]map[string]string{"500000000000001": {"Subject": "A", "Origin": "Web"}, "500000000000002": {"Subject": "SKIP", "Origin": "Web"}, "500000000000003": {"Subject": "A", "Origin": "Phone"}} {
		record := storage.Record{Object: "Case", ID: id, Fields: map[string]storage.Value{}}
		for name, value := range fields {
			record.Fields[name] = storage.StringValue(value)
		}
		cases.Records[id] = record
	}
	org.Objects["Case"] = cases
	if err := ReconcileCriteriaSharing(&org); err != nil {
		t.Fatal(err)
	}
	rows := criteriaRuleRows(&org)
	if len(rows) != 1 {
		t.Fatalf("rows=%v", rows)
	}
	for _, row := range rows {
		if row.Fields["CaseId"].ID != "500000000000001" {
			t.Fatalf("wrong record granted: %v", row)
		}
	}
}

func TestCriteriaSharingPreservesUnownedRuleRows(t *testing.T) {
	org := criteriaRuleOrg()
	storage.EnsureStandardObject(&org, "CaseShare")
	shares := org.Objects["CaseShare"]
	shares.Records["50b000000000099"] = storage.Record{ID: "50b000000000099", Object: "CaseShare", Fields: map[string]storage.Value{"CaseId": storage.IDValue("500000000000099"), "UserOrGroupId": storage.IDValue("00G000000000099"), "RowCause": storage.StringValue("Rule")}}
	org.Objects["CaseShare"] = shares
	cases := org.Objects["Case"]
	cases.Definition.CriteriaSharingRules = nil
	org.Objects["Case"] = cases
	if err := ReconcileCriteriaSharing(&org); err != nil {
		t.Fatal(err)
	}
	if _, ok := org.Objects["CaseShare"].Records["50b000000000099"]; !ok {
		t.Fatal("unowned Rule share removed")
	}
}
