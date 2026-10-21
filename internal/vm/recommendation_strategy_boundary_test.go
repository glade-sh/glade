package vm

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Recommendation strategies require a hosted Flow engine. The local VM
// must report its boundary before executing strategy steps.
func TestRecommendationStrategyUsesHostedBoundary(t *testing.T) {
	program, err := CompileAnonymous(`
Flow.Interview interview=Flow.Interview.createInterview('OwnedHostedStrategy',new Map<String,Object>{'seed'=>'leaf'});
interview.start();
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	rule := scalarInterviewRule("OwnedHostedStrategy")
	rule.ProcessType = "RecommendationStrategy"
	org.Metadata.Flows = []storage.FlowRule{rule}
	machine := New(nil)
	machine.SetOrg(&org)
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" {
		t.Fatalf("expected explicit hosted boundary, got %v", err)
	}
}
