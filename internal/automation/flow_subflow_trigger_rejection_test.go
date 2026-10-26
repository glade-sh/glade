package automation

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
)

func TestFlowSubflowNativeTriggerRejections(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/flow_record_route_twins/rejected"
	for _, name := range []string{"BoundaryBeforeSubflow", "BoundaryEventNested"} {
		t.Run(name, func(t *testing.T) {
			flow, diagnostics, err := loadFlow(filepath.Join(fixture, name+".flow-meta.xml"))
			if err != nil {
				t.Fatal(err)
			}
			for _, d := range diagnostics {
				if d.Severity == diagnostic.Error && strings.Contains(d.Message, "Subflow") {
					if len(flow.Rules) != 0 {
						t.Fatal("rejected metadata retained executable rules")
					}
					return
				}
			}
			t.Fatalf("native rejects Subflow for this trigger type; diagnostics=%v; rules=%d", diagnostics, len(flow.Rules))
		})
	}
}
