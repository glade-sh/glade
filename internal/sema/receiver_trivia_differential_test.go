package sema

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// These free-dot type neighbors have no matching native message row. Their
// complete JSON diagnostics are the output observed on origin/main at 67.0.
func TestReceiverUncapturedTypeNeighbors(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"generic", "{ List<System\n.String> s; }", `[{"severity":"error","code":"GLADESEMA006","message":"Invalid type: System.String","range":{"start":{"line":1,"column":3,"offset":2},"end":{"line":2,"column":8,"offset":21}},"nativeMessage":"Invalid type: System.String"}]`},
		{"cast", "{ Object s = (System\n.String) 'x'; }", "null"},
		{"instanceof", "{ Boolean b = 'x' instanceof System\n.String; }", "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(AnalyzeAnonymous(typesys.Index{}, tc.source, "67.0").Diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("diagnostics %s, want origin/main %s", got, tc.want)
			}
		})
	}
}
