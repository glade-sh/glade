package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRecordSaveResidualsNativeConformance replays the record_save_residuals
// capture (PersonAccounts scratch org) through the named-test route at Apex
// API 62.0 and 67.0:
//   - owner: validation rules on Task and a custom object see OwnerId on
//     insert and update, and see CreatedById, CreatedDate and LastModifiedById
//     blank on insert only;
//   - objecttype: $ObjectType.<object>.Fields.<field> in a validation formula
//     is not null and not blank;
//   - bind: a SOQL bind of a string literal or a parenthesized expression;
//   - person: a person account's contact is created without Contact
//     validation rules, Contact triggers or the Contact.AccountId lookup
//     filter, while a business contact on a person account is rejected;
//   - flow: a Task inserted with WhatId = an Account reads back that
//     AccountId; a Task after-save flow decides on $Record.Account.Id, writes
//     it to a lookup, and creates a record from $Record.Account.Name and
//     $Record.RecordType.DeveloperName;
//   - page: Page.<name> for a page that binds an Activity lookup on Event and
//     Task rows.
//
// Each probe ends with assertEquals('SENTINEL', observed), so the failure
// message carries the observed text that org.tsv records.
func TestRecordSaveResidualsNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/record_save_residuals"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	native := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("malformed row %q", line)
		}
		native[fields[0]+"/"+fields[1]+"/"+fields[2]] = fields[3]
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	run := Run(index, Options{SelectedClasses: []string{"CedarSave62Test", "CedarSave67Test"}, NoDiskCache: true, Parallelism: 1})
	const marker = "Expected: SENTINEL, Actual: "
	seen := 0
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			api := "67.0"
			if strings.Contains(c.ClassName, "62") {
				api = "62.0"
			}
			key := api + "/istest/" + c.MethodName
			want, ok := native[key]
			if !ok {
				t.Fatalf("no native row for %s", key)
			}
			seen++
			if c.Problem == nil || !strings.Contains(c.Problem.Message, marker) {
				t.Errorf("%s: want the sentinel assertion, got %+v", key, c.Problem)
				continue
			}
			got := strings.TrimRight(c.Problem.Message[strings.Index(c.Problem.Message, marker)+len(marker):], " \n")
			if got != want {
				t.Errorf("%s:\n got  %s\n want %s", key, got, want)
			}
		}
	}
	if seen != 12 {
		t.Fatalf("ran %d probe cases, want 12: %+v", seen, run.Summary())
	}
}
