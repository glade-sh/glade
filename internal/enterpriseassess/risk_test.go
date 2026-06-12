package enterpriseassess

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/enterprise"
)

func TestScoreNodeExplainsInputsAndHighRiskTriggerHandlerBecomesHigh(t *testing.T) {
	score := ScoreNode(RiskInputs{
		Symbol:           "AccountTriggerHandler",
		TriggerPath:      true,
		FanOut:           12,
		FanIn:            10,
		DMLOperations:    1,
		SOQLStatements:   3,
		PublicOrGlobal:   true,
		HasTestIndicator: false,
		DynamicReference: true,
	})

	if score.Score != 100 {
		t.Fatalf("score = %d, want 100", score.Score)
	}
	if score.Severity != enterprise.SeverityHigh {
		t.Fatalf("severity = %q, want high", score.Severity)
	}
	for _, want := range []string{"trigger path", "fan-out >= 10", "fan-in >= 10", "DML > 0", "SOQL >= 3", "public/global visibility", "no test indicator", "dynamic reference"} {
		if !strings.Contains(strings.Join(score.Explanations, "\n"), want) {
			t.Fatalf("explanations missing %q: %#v", want, score.Explanations)
		}
	}
}
