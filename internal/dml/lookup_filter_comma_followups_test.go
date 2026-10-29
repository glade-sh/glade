package dml

import (
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestLookupFilterCommaFollowups(t *testing.T) {
	data, err := os.ReadFile("../../testdata/conformance/lookup_filter_followups/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	criteria := map[string]struct{ path, value string }{
		"comma.Name__c":          {"Account.RecordType.Name", "Birch, East"},
		"comma.QuotedName__c":    {"Account.RecordType.Name", `"Birch, East"`},
		"comma.QuotedLabel__c":   {"Account.RecordTypeId", `"Birch, East"`},
		"comma.OtherLabel__c":    {"Account.RecordTypeId", `"Cedar, West"`},
		"comma.ListLabel__c":     {"Account.RecordTypeId", `"Birch, East","Cedar, West"`},
		"number.Number__c":       {"Account.Amount__c", "1,234"},
		"number.NumberPair__c":   {"Account.Amount__c", "12.5,67.25"},
		"number.NumberQuoted__c": {"Account.Amount__c", `"1,234"`},
		"date.DateOne__c":        {"Account.Day__c", "10/26/2026"},
		"date.DateLocale__c":     {"Account.Day__c", "10/26/2026,10/27/2026"},
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		row := strings.Split(line, "\t")
		if len(row) != 5 || row[1] != "anonymous" {
			continue
		}
		criterion, ok := criteria[row[2]]
		if !ok {
			continue
		}
		for _, route := range []string{"insert", "update"} {
			t.Run(strings.Join(row[:4], "/")+"/"+route, func(t *testing.T) {
				org := testOrg()
				org.APIVersion = row[0]
				storage.EnsureStandardObject(&org, "Contact")
				account := org.Objects["Account"]
				account.Definition.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal}
				account.Definition.Fields["Day__c"] = storage.Field{APIName: "Day__c", Type: storage.FieldDate}
				account.Definition.RecordTypes = []storage.RecordTypeInfo{
					{DeveloperName: "Birch_East", Name: "Birch, East", Active: true, Available: true},
					{DeveloperName: "Cedar_West", Name: "Cedar, West", Active: true, Available: true},
					{DeveloperName: "Plain", Name: "Plain", Active: true, Available: true},
				}
				storage.EnsureRecordTypeIDField(&account.Definition)
				org.Objects["Account"] = account
				storage.EnsureDeterministicPlatformData(&org)
				contact := org.Objects["Contact"]
				field := contact.Definition.Fields["AccountId"]
				field.FilteredLookupInfo = storage.FilteredLookupInfo{Active: true, FilterItems: []storage.LookupFilterItem{{Field: criterion.path, Operation: "equals", Value: criterion.value}}}
				contact.Definition.Fields["AccountId"] = field
				org.Objects["Contact"] = contact
				engine := NewEngine(&org)
				fields := map[string]storage.Value{"Name": storage.StringValue("Target")}
				if strings.HasPrefix(row[2], "comma.") {
					for _, rt := range org.Objects["Account"].Definition.RecordTypes {
						if rt.DeveloperName == row[3] {
							fields["RecordTypeId"] = storage.IDValue(rt.ID)
						}
					}
				} else if strings.HasPrefix(row[2], "number.") {
					fields["Amount__c"] = storage.DecimalValue(row[3])
				} else {
					fields["Day__c"] = storage.DateValue("2026-10-" + row[3])
				}
				target := engine.Insert([]storage.Record{{Object: "Account", Fields: fields}})
				if !target[0].Success {
					t.Fatal(target)
				}
				child := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Child")}}
				if route == "update" {
					inserted := engine.Insert([]storage.Record{child})
					if !inserted[0].Success {
						t.Fatal(inserted)
					}
					child.ID = inserted[0].ID
				}
				if row[3] != "null" {
					child.Fields["AccountId"] = storage.IDValue(target[0].ID)
				}
				var result []Result
				if route == "update" {
					result = engine.Update([]storage.Record{child})
				} else {
					result = engine.Insert([]storage.Record{child})
				}
				got := "OK"
				if !result[0].Success {
					got = result[0].Errors[0].StatusCode
				}
				if got != row[4] {
					t.Fatalf("native %s; Glade %s (%+v)", row[4], got, result)
				}
			})
		}
	}
}
