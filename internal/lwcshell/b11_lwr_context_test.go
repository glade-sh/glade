package lwcshell

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestB11LWRContextPresetRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name string
		flag string
		want bool
	}{{"legacy", "", false}, {"lwr", `,"lwr":true`, true}} {
		t.Run(tc.name, func(t *testing.T) {
			var p ContextPreset
			raw := `{"target":"communityPage","component":"c:probe","page":"Home","community":{"site":"Example"` + tc.flag + `}}`
			if err := json.Unmarshal([]byte(raw), &p); err != nil {
				t.Fatal(err)
			}
			ctx, err := p.ToPageContext()
			if err != nil {
				t.Fatal(err)
			}
			out, err := json.Marshal(ctx.Community)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(out), `"lwr":true`) != tc.want {
				t.Fatalf("community context = %s", out)
			}
		})
	}
}
