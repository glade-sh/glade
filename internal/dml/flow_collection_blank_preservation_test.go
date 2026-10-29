package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Preserve the pre-existing local IsBlank result, including its treatment of
// the Boolean operand, until a native collection IsBlank row is available.
func TestFlowLookupCollectionBlankPreservesBranches(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		for _, operand := range []string{"true", "false"} {
			t.Run(fmt.Sprintf("count=%d/operand=%s", count, operand), func(t *testing.T) {
				org := testOrg()
				engine := NewEngine(&org)
				frame := newFlowFrame()
				frame.lookupCollections["found"] = flowRecordCollection{records: make([]storage.Record, count)}
				branch := storage.FlowBranch{Criteria: []storage.WorkflowCriteriaItem{{Field: "Found", Operation: "IsBlank", Value: operand}}}
				matched, supported := engine.evaluateFlowFrameBranch(branch, storage.Record{}, org.Objects["Account"].Definition, frame)
				if !supported || matched != (count == 0) {
					t.Fatalf("matched=%t supported=%t; want %t, true", matched, supported, count == 0)
				}
			})
		}
	}
}
