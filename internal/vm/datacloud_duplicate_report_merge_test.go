package vm

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// mergeDuplicateReportsReference is the pairwise definition of the merge: a
// report joins the first earlier set of the same rule and object that shares a
// record with it.
func mergeDuplicateReportsReference(reports []dml.DuplicateReport) []dml.DuplicateReport {
	shares := func(a, b dml.DuplicateReport) bool {
		left := map[storage.ID]bool{a.RecordID: true}
		for _, match := range a.Matches {
			left[match.ID] = true
		}
		if left[b.RecordID] {
			return true
		}
		for _, match := range b.Matches {
			if left[match.ID] {
				return true
			}
		}
		return false
	}
	var merged []dml.DuplicateReport
	for _, report := range reports {
		target := -1
		for i := range merged {
			if strings.EqualFold(merged[i].Rule.Name, report.Rule.Name) && strings.EqualFold(merged[i].Rule.Object, report.Rule.Object) && strings.EqualFold(merged[i].Object, report.Object) && shares(merged[i], report) {
				target = i
				break
			}
		}
		if target < 0 {
			merged = append(merged, report)
			continue
		}
		into := &merged[target]
		members := map[storage.ID]bool{into.RecordID: true}
		for _, match := range into.Matches {
			members[match.ID] = true
		}
		if !members[report.RecordID] {
			into.Matches = append(into.Matches, storage.Record{ID: report.RecordID, Object: report.Object})
			members[report.RecordID] = true
		}
		for _, match := range report.Matches {
			if !members[match.ID] {
				into.Matches = append(into.Matches, match)
				members[match.ID] = true
			}
		}
	}
	return merged
}

func randomDuplicateReports(r *rand.Rand, count, ids int) []dml.DuplicateReport {
	reports := make([]dml.DuplicateReport, count)
	for i := range reports {
		report := dml.DuplicateReport{
			Rule:     storage.DuplicateRule{Name: []string{"RuleA", "rulea", "RuleB"}[r.Intn(3)], Object: "Contact"},
			Object:   []string{"Contact", "Lead"}[r.Intn(2)],
			RecordID: storage.ID(fmt.Sprintf("r%03d", r.Intn(ids))),
		}
		for n := r.Intn(4); n > 0; n-- {
			report.Matches = append(report.Matches, storage.Record{ID: storage.ID(fmt.Sprintf("r%03d", r.Intn(ids))), Object: report.Object})
		}
		reports[i] = report
	}
	return reports
}

func cloneDuplicateReports(in []dml.DuplicateReport) []dml.DuplicateReport {
	out := make([]dml.DuplicateReport, len(in))
	for i, report := range in {
		out[i] = report
		out[i].Matches = append([]storage.Record(nil), report.Matches...)
	}
	return out
}

func TestMergeDuplicateReportsMatchesPairwiseDefinition(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	for round := 0; round < 400; round++ {
		reports := randomDuplicateReports(r, 1+r.Intn(30), 3+r.Intn(40))
		want := mergeDuplicateReportsReference(cloneDuplicateReports(reports))
		got := mergeDuplicateReports(cloneDuplicateReports(reports))
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("round %d differs\nwant %+v\ngot  %+v", round, want, got)
		}
	}
}

// A bulk Allow+Report insert queues one report per record; merging them must
// not cost a pass over every earlier set per record.
func BenchmarkMergeDuplicateReports(b *testing.B) {
	r := rand.New(rand.NewSource(5))
	reports := randomDuplicateReports(r, 4000, 20000)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		mergeDuplicateReports(cloneDuplicateReports(reports))
	}
}

// NATIVE entity type of an empty custom-object result ../apextest/testdata/conformance/duplicate_runtime_behavior/org.tsv ENT anonymous 62.0
// The platform names the object's 15-character 01I definition id; Glade has no
// definition ids, so it derives a stable one from the object name.
func TestDuplicateObjectDefinitionIDIsStable(t *testing.T) {
	a := duplicateObjectDefinitionID("DupGate__c")
	if len(a) != 15 || !strings.HasPrefix(a, "01I") {
		t.Fatalf("id shape: %q", a)
	}
	if a != duplicateObjectDefinitionID("dupgate__C") {
		t.Fatal("id depends on letter case")
	}
	if a == duplicateObjectDefinitionID("DupKey__c") {
		t.Fatal("two objects share an id")
	}
}
