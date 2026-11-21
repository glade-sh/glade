package dml_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func duplicateIndexFixture(n int) (storage.OrgState, []storage.Record) {
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Lead")
	object := org.Objects["Lead"]
	match := storage.MatchingRule{Object: "Lead", Name: "Custom", Status: "Active", Items: []storage.MatchingRuleItem{
		{Field: "FirstName", Method: "FirstName", BlankBehavior: "NullNotAllowed"},
		{Field: "LastName", Method: "LastName", BlankBehavior: "NullNotAllowed"},
		{Field: "Company", Method: "CompanyName", BlankBehavior: "NullNotAllowed"},
		{Field: "Street", Method: "Street", BlankBehavior: "NullNotAllowed"},
	}}
	object.Definition.DuplicateRules = []storage.DuplicateRule{{Name: "Custom", Object: "Lead", Active: true, ActionOnInsert: "Allow", OperationsOnInsert: []string{"Report"}, MatchRules: []storage.MatchingRule{match}}}
	for i := 0; i < n; i++ {
		id := storage.ID(fmt.Sprintf("00Q%012d", i+1))
		object.PutRecord(id, storage.Record{ID: id, Object: "Lead", Fields: benchLeadFields(i, false), System: storage.SystemFields{CreatedDate: "2023-01-01T00:00:00Z"}})
	}
	org.Objects["Lead"] = object
	rows := make([]storage.Record, 200)
	for i := range rows {
		j := n + i
		if i%2 == 0 {
			j = i * 13
		}
		rows[i] = storage.Record{Object: "Lead", Fields: benchLeadFields(j, i%2 == 0)}
	}
	return org, rows
}

func TestDuplicateBulkMatchesFullScan(t *testing.T) {
	base, rows := duplicateIndexFixture(1000)
	var referenceResults []dml.Result
	var referenceReports []dml.DuplicateReport
	for _, fullScan := range []bool{true, false} {
		org := storage.SnapshotRuntimeOrg(&base)
		engine := dml.NewEngine(&org)
		engine.Options.EnforceDuplicateRules = true
		engine.Now = func() time.Time { return time.Date(2023, 10, 8, 0, 0, 0, 0, time.UTC) }
		if fullScan {
			engine.DuplicateMatcher = func(rule storage.DuplicateRule, row storage.Record, exclude func(storage.ID) bool) (dml.DuplicateResult, bool) {
				return dml.FindDuplicateMatches(&org, rule, row, exclude)
			}
		}
		batch := make([]storage.Record, len(rows))
		for i := range rows {
			batch[i] = rows[i].Clone()
		}
		got := engine.Insert(batch)
		reports := engine.TakeDuplicateReports()
		for _, result := range got {
			if !result.Success {
				t.Fatalf("insert: %+v", result)
			}
		}
		if fullScan {
			referenceResults, referenceReports = got, reports
			if len(reports) == 0 {
				t.Fatal("fixture has no matches")
			}
			continue
		}
		if !reflect.DeepEqual(got, referenceResults) || !reflect.DeepEqual(reports, referenceReports) {
			t.Fatal("batch results or ordered reports differ from full scan")
		}
	}
}

func BenchmarkDuplicateInsertFuzzy200By20000(b *testing.B) {
	base, rows := duplicateIndexFixture(20000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		org := storage.SnapshotRuntimeOrg(&base)
		engine := dml.NewEngine(&org)
		engine.Options.EnforceDuplicateRules = true
		batch := make([]storage.Record, len(rows))
		for j := range rows {
			batch[j] = rows[j].Clone()
		}
		b.StartTimer()
		for _, result := range engine.Insert(batch) {
			if !result.Success {
				b.Fatalf("insert: %+v", result)
			}
		}
	}
}
