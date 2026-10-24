package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func statementInsertAllocationInput(size int, element string) Value {
	value := List()
	value.Type = "List<" + element + ">"
	for i := 0; i < size; i++ {
		record := Object("Account")
		record.Fields["Name"] = String("Synthetic account")
		value.List = append(value.List, record)
	}
	return value
}

func TestStatementInsertStandardEventCheckAllocations(t *testing.T) {
	for _, element := range []string{"Account", "SObject"} {
		t.Run(element, func(t *testing.T) {
			machine := New(nil)
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			machine.SetOrg(&org)
			measure := func(size int) float64 {
				value := statementInsertAllocationInput(size, element)
				return testing.AllocsPerRun(20, func() {
					if err := machine.standardEventStatementInsertBoundary(value); err != nil {
						t.Fatal(err)
					}
				})
			}
			small, large := measure(1), measure(64)
			t.Logf("standard-event check allocs/op: 1 record=%.0f, 64 records=%.0f", small, large)
			if large > small+4 {
				t.Fatalf("standard-event check allocations scale with records: %.0f -> %.0f (growth limit 4)", small, large)
			}
		})
	}
}

func BenchmarkStatementInsertStandardEventCheck(b *testing.B) {
	for _, size := range []int{1, 64} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			machine := New(nil)
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			machine.SetOrg(&org)
			value := statementInsertAllocationInput(size, "SObject")
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := machine.standardEventStatementInsertBoundary(value); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
