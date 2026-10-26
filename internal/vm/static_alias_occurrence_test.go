package vm

import (
	"encoding/json"
	"fmt"
	"testing"
)

func staticRecordAliasFixture(t testing.TB, count int) (*VM, Value) {
	t.Helper()
	machine := New(nil)
	rows := List()
	rows.Type = "List<Account>"
	for range count {
		row := Object("Account")
		row.Fields["Name"] = String("Synthetic")
		duplicate := row
		duplicate.Fields = cloneAliasOccurrenceFields(row.Fields)
		rows.List = append(rows.List, row, duplicate)
	}
	class := Class{Name: "StaticRows", Namespace: "sample", StaticFields: map[string]Field{
		"Rows": {Name: "Rows", Type: rows.Type, Static: true, Access: "public"},
	}}
	if err := machine.RegisterClass(class); err != nil {
		t.Fatal(err)
	}
	class = machine.Classes["StaticRows"]
	field := class.StaticFields["Rows"]
	machine.writeStaticFieldValue("StaticRows", "Rows", class, field, rows)
	return machine, rows
}

func TestStaticRecordAliasOccurrenceLinearWork(t *testing.T) {
	for _, order := range []string{"forward", "reverse", "shuffled"} {
		t.Run(order, func(t *testing.T) {
			var last uint64
			for _, count := range []int{200, 800, 4000} {
				machine, rows := staticRecordAliasFixture(t, count)
				recorder := NewPerfRecorder()
				machine.SetPerfRecorder(recorder)
				for step := range count {
					position := step
					if order == "reverse" {
						position = count - 1 - step
					}
					if order == "shuffled" {
						position = (step * 137) % count
					}
					previous := rows.List[2*position]
					updated := previous
					updated.Fields = cloneAliasOccurrenceFields(previous.Fields)
					updated.Fields["Description"] = String(fmt.Sprintf("step-%d", position))
					machine.propagateAliasSnapshotToStatics(snapshotAlias(previous), updated)
					for _, className := range []string{"StaticRows", "sample.StaticRows"} {
						stored := machine.Classes[className].StaticFields["Rows"].Value
						for _, slot := range []int{2 * position, 2*position + 1} {
							if got := stored.List[slot]; got.Ref != previous.Ref || got.Fields["Description"].Text != updated.Fields["Description"].Text {
								t.Fatalf("%s/%d position %d: duplicate static alias lost update", order, count, slot)
							}
						}
					}
				}
				for i, row := range rows.List {
					if got, want := row.Fields["Description"].Text, fmt.Sprintf("step-%d", i/2); got != want {
						t.Fatalf("row %d Description=%q, want %q", i, got, want)
					}
				}
				// Read the exported report so this regression compiles on runtimes
				// whose static replacement work is not yet measured.
				raw, err := json.Marshal(recorder.Snapshot().StaticAlias)
				if err != nil {
					t.Fatal(err)
				}
				counters := map[string]uint64{}
				if err := json.Unmarshal(raw, &counters); err != nil {
					t.Fatal(err)
				}
				replacement := counters["replacementVisits"]
				visits := replacement + counters["refScanVisits"] + counters["locationVisits"] + recorder.Snapshot().ScopeAlias.ShortcutVisits
				t.Logf("records=%d replacement=%d counted_visits=%d all_results=matched", count, replacement, visits)
				if replacement == 0 {
					t.Errorf("records=%d: static replacement work is unmeasured", count)
				}
				if visits > uint64(120*count) {
					t.Errorf("records=%d counted_visits=%d, want <= %d", count, visits, 120*count)
				}
				if last != 0 && visits > 6*last {
					t.Errorf("counted visits grew from %d to %d, want <= 6x", last, visits)
				}
				last = visits
			}
		})
	}
}

func TestStaticRecordAliasOccurrenceFallbackAndCloneRoutes(t *testing.T) {
	machine, rows := staticRecordAliasFixture(t, 2)
	first := rows.List[0]
	changed := first
	changed.Fields = cloneAliasOccurrenceFields(first.Fields)
	changed.Fields["Description"] = String("warm")
	machine.propagateAliasSnapshotToStatics(snapshotAlias(first), changed)
	machine.FreezeClassLookup()
	sibling := machine.CloneRuntime(nil)
	clone := machine.CloneRuntime(nil)
	cloneRows := clone.Classes["StaticRows"].StaticFields["Rows"].Value
	target := cloneRows.List[0]
	updated := target
	updated.Fields = cloneAliasOccurrenceFields(target.Fields)
	updated.Fields["Description"] = String("clone")
	clone.propagateAliasSnapshotToStatics(snapshotAlias(target), updated)
	for name, other := range map[string]*VM{"template": machine, "sibling": sibling} {
		for _, row := range other.Classes["StaticRows"].StaticFields["Rows"].Value.List[:2] {
			if row.Fields["Description"].Text != "warm" {
				t.Fatalf("%s changed through clone", name)
			}
		}
	}
	// Publish a nested target after warming the flat index; both nested and
	// direct aliases must be refreshed by the unsupported-graph fallback.
	clone.setGraphFieldValue(&cloneRows.List[2], "Parent", cloneRows.List[0])
	target = cloneRows.List[0]
	updated = target
	updated.Fields = cloneAliasOccurrenceFields(target.Fields)
	updated.Fields["Description"] = String("nested")
	clone.propagateAliasSnapshotToStatics(snapshotAlias(target), updated)
	if got := cloneRows.List[2].Fields["Parent"].Fields["Description"].Text; got != "nested" {
		t.Fatalf("nested alias=%q", got)
	}
}

func BenchmarkStaticRecordAliasOccurrence(b *testing.B) {
	for _, count := range []int{200, 800, 4000} {
		b.Run(fmt.Sprint(count), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				b.StopTimer()
				machine, rows := staticRecordAliasFixture(b, count)
				b.StartTimer()
				for i := range count {
					previous := rows.List[2*i]
					updated := previous
					updated.Fields = cloneAliasOccurrenceFields(previous.Fields)
					updated.Fields["Description"] = String("processed")
					machine.propagateAliasSnapshotToStatics(snapshotAlias(previous), updated)
				}
			}
		})
	}
}
