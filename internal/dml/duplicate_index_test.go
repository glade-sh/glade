package dml_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
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
			payload, err := json.Marshal(struct {
				Results []dml.Result
				Reports []dml.DuplicateReport
			}{got, reports})
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(payload)); got != "bcd83ca57af13a5ee86c474f901874853a06f97c53cd531b4a06051b3c8f74c5" {
				t.Fatalf("batch outcome digest %s", got)
			}
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

func TestDuplicateBulkMatchesFullScan20000(t *testing.T) {
	base, rows := duplicateIndexFixture(20000)
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
		results := engine.Insert(batch)
		reports := engine.TakeDuplicateReports()
		for _, result := range results {
			if !result.Success {
				t.Fatalf("insert: %+v", result)
			}
		}
		if fullScan {
			if len(reports) == 0 {
				t.Fatal("fixture has no matches")
			}
			referenceResults, referenceReports = results, reports
			continue
		}
		if !reflect.DeepEqual(results, referenceResults) || !reflect.DeepEqual(reports, referenceReports) {
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

func TestDuplicateScanOutcomes(t *testing.T) {
	base, _ := duplicateIndexFixture(400)
	object := base.Objects["Lead"]
	row := storage.Record{Object: "Lead", Fields: benchLeadFields(13, true)}
	for i := 1; i <= 150; i++ {
		id := storage.ID(fmt.Sprintf("00Q%012d", i))
		existing := object.Records[id].Clone()
		existing.Fields = benchLeadFields(13, false)
		object.PutRecord(id, existing)
	}
	base.Objects["Lead"] = object
	rule := object.Definition.DuplicateRules[0]
	for _, tc := range []struct {
		name      string
		want      string
		filter    string
		candidate storage.Record
		access    func(storage.Record) (bool, bool)
		exclude   func(storage.ID) bool
	}{
		{name: "cap", want: "812770762e7acd41c403d81318377a1ccdadd736233acba781c6a2da4abf8107", candidate: row},
		{name: "hidden", want: "15bd310b34b7378be255f80d07f3bfa5b88fdb27e0252dfa54cede2841f7f9f2", candidate: row, access: func(storage.Record) (bool, bool) { return true, false }},
		{name: "excluded", want: "ae1e126d7405a2a841112bceff1eb1f585cf371e206dab83652a3e5539d40c37", candidate: row, exclude: func(id storage.ID) bool { return id < "00Q000000000040" }},
		{name: "or", want: "85c7c0c311f101bfa3bb9c110d48debe69561f39031170bdb5d6fbd03c5f9044", filter: "1 OR 2", candidate: row},
		{name: "unsupported", want: "c0b2fa19bf5cd41cde746c1ef4e65932189c9cc3e2c1aff51564725fd82efd97", filter: "1 OR 99", candidate: row},
		{name: "blank", want: "bcd7bf15242ac25451e09f2eec270f4142d49cb16ad978782284c3a586bbfc2b", candidate: storage.Record{Object: "Lead", Fields: map[string]storage.Value{"FirstName": storage.NullValue(), "LastName": storage.StringValue("Smith")}}},
		{name: "folded", want: "bcd7bf15242ac25451e09f2eec270f4142d49cb16ad978782284c3a586bbfc2b", candidate: storage.Record{Object: "Lead", Fields: map[string]storage.Value{"firstname": storage.StringValue("James"), "lastname": storage.StringValue("Smith")}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := rule
			r.MatchRules = append([]storage.MatchingRule(nil), rule.MatchRules...)
			r.MatchRules[0].BooleanFilter = tc.filter
			result, supported := dml.FindDuplicateMatchesWithAccess(&base, r, tc.candidate, tc.exclude, tc.access)
			raw, err := json.Marshal(struct {
				Result    dml.DuplicateResult
				Supported bool
			}{result, supported})
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != tc.want {
				t.Fatalf("outcome digest %s, want %s", got, tc.want)
			}
			if tc.name == "cap" && (!supported || len(result.Matches) != 1 || len(result.Matches[0].Records) != 100) {
				t.Fatalf("cap: %+v", result)
			}
			if tc.name == "hidden" && (!result.Detected || len(result.Matches[0].Records) != 0) {
				t.Fatal("hidden matches were not detected")
			}
		})
	}
}

func TestDuplicateScanRefreshesValues(t *testing.T) {
	org, _ := duplicateIndexFixture(300)
	object := org.Objects["Lead"]
	ids := make([]storage.ID, 0, len(object.Records))
	for id, original := range object.Records {
		record := original.Clone()
		record.Fields["FirstName"] = storage.StringValue("other")
		object.PutRecord(id, record)
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	record := object.Records[ids[0]]
	record.Fields["FirstName"] = storage.StringValue("match")
	org.Objects["Lead"] = object
	rule := object.Definition.DuplicateRules[0]
	rule.MatchRules[0].Items = []storage.MatchingRuleItem{{Field: "FirstName", Method: "Exact", BlankBehavior: "NullNotAllowed"}}
	candidate := storage.Record{Object: "Lead", Fields: map[string]storage.Value{"FirstName": storage.StringValue("match")}}
	check := func(want storage.ID) {
		t.Helper()
		result, ok := dml.FindDuplicateMatches(&org, rule, candidate, nil)
		if !ok || len(result.Matches) != 1 || len(result.Matches[0].Records) != 1 || result.Matches[0].Records[0].ID != want {
			t.Fatalf("want %s, got %+v", want, result)
		}
	}
	check(ids[0])
	// Mutate the shared field maps without a record-map write: each scan must
	// observe fresh values even when the storage token does not change.
	record.Fields["FirstName"] = storage.StringValue("other")
	other := object.Records[ids[1]]
	other.Fields["FirstName"] = storage.StringValue("match")
	check(ids[1])
	candidate.Fields["FirstName"] = storage.StringValue("other")
	result, ok := dml.FindDuplicateMatches(&org, rule, candidate, nil)
	if !ok || len(result.Matches) != 1 || len(result.Matches[0].Records) != 100 {
		t.Fatalf("changed candidate: %+v", result)
	}
}

// A statement's later duplicate checks must observe changes to stored rows,
// including field-map writes that do not advance the storage record token.
func TestDuplicateBulkObservesStoredChanges(t *testing.T) {
	for _, mutation := range []string{"field-map", "put", "delete", "replace-map"} {
		t.Run(mutation, func(t *testing.T) {
			base, _ := duplicateIndexFixture(400)
			targetID := storage.ID("00Q000000000001")
			matching := benchLeadFields(900, false)
			if mutation == "delete" {
				object := base.Objects["Lead"]
				target := object.Records[targetID].Clone()
				target.Fields = matching
				object.PutRecord(targetID, target)
				base.Objects["Lead"] = object
			}
			rows := []storage.Record{
				{Object: "Lead", Fields: benchLeadFields(901, false)},
				{Object: "Lead", Fields: matching},
			}
			var expectedResults []dml.Result
			var expectedReports []dml.DuplicateReport
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
				created := 0
				engine.ChangeObserver = func(operation string, _, _ storage.Record) {
					if operation != "CREATE" {
						return
					}
					created++
					if created != 1 {
						return
					}
					object := org.Objects["Lead"]
					target := object.Records[targetID]
					switch mutation {
					case "field-map":
						for key, value := range matching {
							target.Fields[key] = value
						}
					case "put":
						target = target.Clone()
						target.Fields = matching
						object.PutRecord(targetID, target)
					case "delete":
						object.DeleteRecord(targetID)
					case "replace-map":
						target = target.Clone()
						target.Fields = matching
						records := make(map[storage.ID]storage.Record, len(object.Records))
						for id, record := range object.Records {
							records[id] = record
						}
						records[targetID] = target
						object.SetRecords(records)
					}
					org.Objects["Lead"] = object
				}
				batch := []storage.Record{rows[0].Clone(), rows[1].Clone()}
				results := engine.Insert(batch)
				reports := engine.TakeDuplicateReports()
				for _, result := range results {
					if !result.Success {
						t.Fatalf("insert: %+v", result)
					}
				}
				if fullScan {
					if mutation == "delete" && len(reports) != 0 || mutation != "delete" && len(reports) != 1 {
						t.Fatalf("reference reports: %+v", reports)
					}
					expectedResults, expectedReports = results, reports
				} else if !reflect.DeepEqual(results, expectedResults) || !reflect.DeepEqual(reports, expectedReports) {
					t.Fatalf("results or reports differ from full scan: %+v", reports)
				}
			}
		})
	}
}
