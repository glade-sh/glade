package automation

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowMergeNativeReferenceRejection(t *testing.T) {
	const dir = "testdata/flow_merge_format"
	flow, diagnostics, err := loadFlow(filepath.Join(dir, "Format_invalid.flow-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(flow.Rules) != 0 {
		t.Fatal("rejected flow has executable rules")
	}
	for _, d := range diagnostics {
		if strings.Contains(d.Message, `"Reject" refers to an element: "doesNotExist", which doesn't exist`) {
			return
		}
	}
	t.Fatalf("missing native rejection: %+v", diagnostics)
}
