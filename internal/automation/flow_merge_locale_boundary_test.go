package automation

import (
	"encoding/xml"
	"os"
	"testing"
)

// Metadata loading has no executing user. Numeric merges must remain unresolved
// until rendering, so a cached Flow is not fixed to the loader's locale.
func TestFlowMergeNumericConstantRemainsUserScoped(t *testing.T) {
	b, err := os.ReadFile("testdata/flow_merge_format/Format_constant.flow-meta.xml")
	if err != nil {
		t.Fatal(err)
	}
	var raw flowXML
	if err := xml.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	got := modeledFlowCustomError(raw.CustomErrors[0], raw.Constants).Messages[0].Message
	want := "constant={!large}|negative={!negative}|boundary={!boundary}|below=999.5"
	if got != want {
		t.Fatalf("metadata resolved user-specific numeric text: got %q; want authored %q", got, want)
	}
}
