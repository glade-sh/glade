package debuglog

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apexlog"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestBuildEditorAnalysisJSONContract(t *testing.T) {
	log, err := apexlog.Parse(strings.NewReader("00:00:00.001 (1)|METHOD_ENTRY|[2]|01p|Test.run()\n"))
	if err != nil {
		t.Fatal(err)
	}
	analysis := BuildEditorAnalysis(log, typesys.Index{}, EditorOptions{
		LogFile:     "apex.log",
		ProjectRoot: "/repo",
		Now: func() time.Time {
			return time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
		},
	})
	data, err := json.MarshalIndent(analysis, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		`"version": 1`,
		`"language": "apexlog"`,
		`"entries"`,
		`"symbols"`,
		`"folds"`,
		`"links"`,
		`"hovers"`,
		`"codeLenses"`,
		`"semanticTokens"`,
		`"diagnostics"`,
		`"variables"`,
		`"replayFrames"`,
		`"coverage"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("contract missing %q:\n%s", want, got)
		}
	}
}
