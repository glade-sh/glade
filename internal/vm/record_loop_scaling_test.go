package vm

import (
	"fmt"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/ir"
)

// Keep setup outside the measurement: this isolates writes to a stable list.
// ShortcutVisits includes flat-root verification and replacement slot visits.
// These observations characterize work; they do not assert a linear bound.
func TestRecordFieldLoopScaling(t *testing.T) {
	for _, mutation := range []string{"assignment", "put"} {
		for _, count := range []int{200, 800, 4000} {
			t.Run(fmt.Sprintf("%s/%d", mutation, count), func(t *testing.T) {
				machine, program := recordFieldLoopFixture(t, count, mutation)
				recorder := NewPerfRecorder()
				machine.SetPerfRecorder(recorder)
				started := time.Now()
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
				elapsed := time.Since(started)
				assertRecordFieldLoopRows(t, machine, count)
				stats := recorder.Snapshot().ScopeAlias
				t.Logf("records=%d elapsed=%s calls=%d roots=%d recursive=%d shortcut=%d elision_fallbacks=%d replaced_roots=%d counted_visits=%d", count, elapsed, stats.Calls, stats.Roots, stats.RecursiveVisits, stats.ShortcutVisits, stats.ElisionFallbackWalks, stats.ReplacedRoots, stats.Roots+stats.RecursiveVisits+stats.ShortcutVisits+stats.ElisionFallbackWalks)
			})
		}
	}
}

func TestRecordFieldLoopLinear(t *testing.T) {
	const maxVisitsPerRecord = 40
	for _, mutation := range []string{"assignment", "put"} {
		for _, reverse := range []bool{false, true} {
			order := "forward"
			if reverse {
				order = "reverse"
			}
			t.Run(fmt.Sprintf("%s/%s", mutation, order), func(t *testing.T) {
				visitsByCount := make(map[int]uint64, 2)
				for _, count := range []int{200, 800} {
					machine, program := recordFieldLoopFixtureOrder(t, count, mutation, reverse)
					recorder := NewPerfRecorder()
					machine.SetPerfRecorder(recorder)
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					assertRecordFieldLoopRows(t, machine, count)
					stats := recorder.Snapshot().ScopeAlias
					visits := stats.Roots + stats.RecursiveVisits + stats.ShortcutVisits + stats.ElisionFallbackWalks
					visitsByCount[count] = visits
					if limit := uint64(maxVisitsPerRecord * count); visits > limit {
						t.Fatalf("records=%d counted_visits=%d, want <= %d", count, visits, limit)
					}
				}
				if visitsByCount[800] > 5*visitsByCount[200] {
					t.Fatalf("counted visits grew from %d to %d; want at most 5x", visitsByCount[200], visitsByCount[800])
				}
			})
		}
	}
}

func TestRecordFieldLoopRepeatedAliases(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "AliasLoopNode", Fields: map[string]Field{
		"Value": {Name: "Value", Type: "String", Access: "public"},
	}}); err != nil {
		t.Fatal(err)
	}
	target := Object("AliasLoopNode")
	target.Fields["Value"] = String("before")
	rows := List(target, target)
	rows.Type = "List<AliasLoopNode>"
	machine.Globals["rows"] = rows
	program, err := CompileAnonymous("for (AliasLoopNode row : rows) { row.Value = 'after'; }")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	for index, row := range machine.Globals["rows"].List {
		if got := row.Fields["Value"]; got.Kind != ValueString || got.Text != "after" {
			t.Fatalf("row %d Value = %#v, want after", index, got)
		}
	}
}

func BenchmarkRecordFieldLoopScaling(b *testing.B) {
	for _, mutation := range []string{"assignment", "put"} {
		for _, count := range []int{200, 800, 4000} {
			b.Run(fmt.Sprintf("%s/%d", mutation, count), func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					b.StopTimer()
					machine, program := recordFieldLoopFixture(b, count, mutation)
					recorder := NewPerfRecorder()
					machine.SetPerfRecorder(recorder)
					b.StartTimer()
					if _, err := machine.Execute(program); err != nil {
						b.Fatal(err)
					}
					b.StopTimer()
					assertRecordFieldLoopRows(b, machine, count)
					stats := recorder.Snapshot().ScopeAlias
					b.ReportMetric(float64(stats.Roots+stats.RecursiveVisits+stats.ShortcutVisits+stats.ElisionFallbackWalks), "counted_visits/op")
				}
			})
		}
	}
}

func recordFieldLoopFixture(t testing.TB, count int, mutation string) (*VM, ir.Program) {
	return recordFieldLoopFixtureOrder(t, count, mutation, false)
}

func recordFieldLoopFixtureOrder(t testing.TB, count int, mutation string, reverse bool) (*VM, ir.Program) {
	t.Helper()
	source := "for (Account row : rows) { row.Description = 'processed'; }"
	if mutation == "put" {
		source = "for (Account row : rows) { row.put('Description', 'processed'); }"
	}
	if reverse {
		write := "row.Description = 'processed';"
		if mutation == "put" {
			write = "row.put('Description', 'processed');"
		}
		source = "for (Integer i = rows.size() - 1; i >= 0; i--) { Account row = rows[i]; " + write + " }"
	}
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	rows := List()
	rows.Type = "List<Account>"
	for range count {
		row := Object("Account")
		row.Fields["Name"] = String("Synthetic")
		rows.List = append(rows.List, row)
	}
	machine.Globals["rows"] = rows
	return machine, program
}

func assertRecordFieldLoopRows(t testing.TB, machine *VM, count int) {
	t.Helper()
	rows := machine.Globals["rows"].List
	if len(rows) != count {
		t.Fatalf("got %d rows, want %d", len(rows), count)
	}
	for i, row := range rows {
		if got := row.Fields["Description"]; got.Kind != ValueString || got.Text != "processed" {
			t.Fatalf("row %d Description = %#v, want processed", i, got)
		}
	}
}
