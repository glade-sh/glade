package visualforce_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/visualforce"
)

type v09ClickCase struct {
	SearchTerm string                     `json:"search_term"`
	ID         string                     `json:"id"`
	Kind       string                     `json:"kind"`
	Mode       string                     `json:"mode"`
	Inputs     map[string]v04Input        `json:"inputs"`
	Expected   map[string]json.RawMessage `json:"expected"`
}

type v09ClickTable struct {
	SearchOracleCommit string                `json:"search_oracle_commit"`
	OracleCommit       string                `json:"oracle_commit"`
	SourceFingerprint  string                `json:"source_fingerprint"`
	Fixtures           map[string]v09Fixture `json:"fixtures"`
	Cases              []v09ClickCase        `json:"cases"`
}

// Native API59/67 real shortcut and lookup popup actions. Raw samples are
// retained in the oracle. Only changing shortcut text and settling sample
// timing/count are qualified; field attributes, hidden values and outcomes
// remain exact. No expected answers reach the browser.
func TestV09ClickedFieldControlsSalesforceConformance(t *testing.T) {
	data, err := os.ReadFile("testdata/v09_click_controls.json")
	if err != nil {
		t.Fatal(err)
	}
	var table v09ClickTable
	if err := json.Unmarshal(data, &table); err != nil {
		t.Fatal(err)
	}
	if table.OracleCommit != "capture-revision-052894678750660176679507909893486948296617685750742419099560809719444478262827" || table.SourceFingerprint != "1ff0e27498ad7136faee1ad5fb7c07c404331148b13e609ca305e00fba3e4d4f" || table.SearchOracleCommit != "capture-revision-033766294255307155492926878231427315403712730656595610938183595242950901176794" || len(table.Cases) != 22 {
		t.Fatal("incomplete native clicked controls")
	}
	total := 0
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			v09WriteFiles(t, root, table.Fixtures[api].Files)
			for _, row := range table.Cases {
				v09WriteFiles(t, root, row.Inputs[api].Files)
			}
			matches := 0
			for _, row := range table.Cases {
				if row.Kind != "compile" {
					continue
				}
				got, diagnostics := v04Execute(t, root, row.Inputs[api].Name, v04Case{Kind: "compile"})
				if len(diagnostics) != 0 {
					got += "|" + v04Encode(t, diagnostics)
				}
				var want string
				if err := json.Unmarshal(row.Expected[api], &want); err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Errorf("%s actual <%s> expected <%s>", row.ID, got, want)
				} else {
					matches++
				}
				v09ReleaseRowState(t, root)
			}
			actual := v09ClickedBrowser(t, root, api, table.Cases)
			for _, row := range table.Cases {
				if row.Kind != "runtime" {
					continue
				}
				var want any
				if err := json.Unmarshal(row.Expected[api], &want); err != nil {
					t.Fatal(err)
				}
				got, ok := actual[row.ID]
				if !ok {
					t.Errorf("missing local row %s", row.ID)
					continue
				}
				gotText, wantText := v04Encode(t, v09ClickProjection(got)), v04Encode(t, v09ClickProjection(want))
				if gotText != wantText {
					t.Errorf("%s actual <%s> expected <%s>", row.ID, gotText, wantText)
				} else {
					matches++
				}
			}
			t.Logf("V09 clicked controls API %s matched %d/%d (dynamic shortcut and sample timing qualified)", api, matches, len(table.Cases))
			total += matches
		})
	}
	t.Logf("V09 clicked controls matched %d/%d", total, 2*len(table.Cases))
}

func v09ClickProjection(value any) any {
	switch row := value.(type) {
	case map[string]any:
		projected := make(map[string]any, len(row))
		for key, item := range row {
			// Raw samples are retained; sampling schedule is not a native contract.
			if key == "samples" && row["settled"] != nil {
				continue
			}
			projected[key] = v09ClickProjection(item)
		}
		if shortcut, ok := row["shortcutText"].(string); ok && shortcut != "" {
			projected["shortcutText"] = "CURRENT_DATE_SHORTCUT"
			if row["value"] == shortcut {
				projected["value"] = "CURRENT_DATE_SHORTCUT"
			}
		}
		return projected
	case []any:
		projected := make([]any, len(row))
		for i, item := range row {
			projected[i] = v09ClickProjection(item)
		}
		return projected
	default:
		return value
	}
}

func v09ClickedBrowser(t *testing.T, root, api string, rows []v09ClickCase) map[string]any {
	t.Helper()
	renderer := v09NewRenderer(t, root)
	defer visualforce.V09ReleaseRenderEnvironmentsForTest(root)
	var mu sync.Mutex
	var current *server.Server
	currentPath := ""
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if strings.HasPrefix(r.URL.Path, "/apex/") && (current == nil || currentPath != r.URL.Path) {
			org := renderer.org.CloneRuntimeOrg()
			org.APIVersion = api
			current = server.NewWithSource(&org, renderer.source)
			current.VisualforceHTMLUserID = renderer.principal
			machine := renderer.runtime.CloneRuntime(nil)
			for _, row := range rows {
				machine.RegisterPageReference(row.Inputs[api].Name)
			}
			current.SetProjectRuntime(renderer.index, machine, renderer.runtimeErr)
			currentPath = r.URL.Path
		}
		if current == nil {
			http.NotFound(w, r)
			return
		}
		current.ServeHTTP(w, r)
	}))
	defer local.Close()
	ids := []string{}
	mapping := map[string]any{}
	for _, row := range rows {
		if row.Kind == "runtime" {
			ids = append(ids, row.ID)
			mapping[row.ID] = map[string]string{"page": row.Inputs[api].Name, "mode": row.Mode, "search_term": row.SearchTerm}
		}
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	module := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if module == "" {
		module = filepath.Join(repo, "lwcruntime/node_modules/playwright")
	}
	config, err := json.Marshal(map[string]any{"url": local.URL, "ids": ids, "mapping": mapping, "playwrightModule": module, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"), "roles": map[string]string{"ownerName": v09OwnerName, "ownerId": string(renderer.principal), "accountId": v09AccountID}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "testdata/v09_click_browser.mjs")
	cmd.Stdin = bytes.NewReader(config)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("V09 clicked controls API %s local browser: %v: %s", api, err, stderr.String())
	}
	actual := map[string]any{}
	if err := json.Unmarshal(output, &actual); err != nil {
		t.Fatal(err)
	}
	if len(actual) != len(ids) {
		t.Fatal(fmt.Sprintf("local browser returned %d/%d", len(actual), len(ids)))
	}
	return actual
}
