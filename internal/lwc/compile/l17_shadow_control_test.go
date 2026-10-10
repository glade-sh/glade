package compile_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/gladehome"
)

// TestL17ShadowAccessSalesforceConformance distinguishes a raw null shadowRoot
// from a visible or opaque root using the captured component itself. The three
// observations come from native API 59/67, and the local side uses the same
// production tab adapter as the complete resource conformance table.
func TestL17ShadowAccessSalesforceConformance(t *testing.T) {
	raw, err := os.ReadFile("testdata/l17_shadow_access_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Source   string                     `json:"source"`
		Case     l17Case                    `json:"case"`
		Captured map[string]json.RawMessage `json:"captured"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.Source == "" || filepath.IsAbs(data.Source) ||
		data.Case.ID == "" || data.Case.Kind != "runtime" ||
		data.Case.JS == "" || data.Case.Template == "" || len(data.Captured) != 2 {
		t.Fatal("incomplete native shadow-access control export")
	}
	dependencies, err := gladehome.EnsureRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			native, ok := data.Captured[api]
			if !ok {
				t.Fatalf("missing native shadow-access controls for API %s", api)
			}
			want := l17ShadowAccessControls(t, native, data.Case.ID)
			observed, rowErrors, err := l17ObserveDOM(t, api, []l17Case{data.Case}, dependencies)
			if err != nil {
				t.Fatal(err)
			}
			if len(rowErrors) != 0 {
				t.Fatalf("shadow-access browser errors: %v", rowErrors)
			}
			local, ok := observed[data.Case.ID]
			if !ok || len(observed) != 1 {
				t.Fatalf("expected one observed shadow-access row %s, got %d", data.Case.ID, len(observed))
			}
			got := l17ShadowAccessControls(t, local, data.Case.ID)
			matched := 0
			for _, name := range []string{"baseButton", "ordinaryElement", "component"} {
				expectedText := l13CanonicalJSON(t, string(want[name]))
				observedText := l13CanonicalJSON(t, string(got[name]))
				if observedText != expectedText {
					t.Errorf("%s API %s %s expected <%s> actual <%s>", data.Case.ID, api, name, expectedText, observedText)
				} else {
					matched++
				}
			}
			t.Logf("L17 shadow-access controls API %s matches %d/3", api, matched)
		})
	}
}

func l17ShadowAccessControls(t *testing.T, raw json.RawMessage, id string) map[string]json.RawMessage {
	t.Helper()
	var row struct {
		ID       string                     `json:"id"`
		Controls map[string]json.RawMessage `json:"shadowAccessControls"`
	}
	if err := json.Unmarshal(raw, &row); err != nil {
		t.Fatal(err)
	}
	if row.ID != id || len(row.Controls) != 3 {
		t.Fatalf("invalid shadow-access row %q: expected %q with exactly three observations", row.ID, id)
	}
	for _, name := range []string{"baseButton", "ordinaryElement", "component"} {
		observation, ok := row.Controls[name]
		if !ok {
			t.Fatalf("missing %s shadow-access observation", name)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(observation, &fields); err != nil {
			t.Fatal(err)
		}
		if len(fields) != 4 {
			t.Fatalf("%s shadow-access observation needs exactly four captured fields, got %d", name, len(fields))
		}
		for _, field := range []string{"hostPresent", "rootIsNull", "rootIsUndefined", "rootType"} {
			if _, ok := fields[field]; !ok {
				t.Fatalf("%s shadow-access observation missing %s", name, field)
			}
		}
	}
	return row.Controls
}
