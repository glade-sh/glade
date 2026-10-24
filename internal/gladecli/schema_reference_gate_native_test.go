package gladecli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/testreport"
)

// The native deployment rejection and unaffected executions are separate
// observations. Running the valid subset of a local project is gate policy.
func TestSchemaReferenceGateProjectRoutesNativeConformance(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	const dir = "../sema/testdata/schema_reference_gate"
	raw, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 17 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	oracle := map[string]bool{}
	for _, r := range rows[1:] {
		if len(r) != 5 || (r[3] != "true" && r[3] != "false") {
			t.Fatalf("invalid native row: %v", r)
		}
		oracle[strings.Join(r[:3], "/")] = r[3] == "true"
	}
	var data struct {
		Cases []struct {
			API             string            `json:"api"`
			ID              string            `json:"id"`
			Files           map[string]string `json:"files"`
			FieldPath       string            `json:"fieldPath"`
			IndependentTest string            `json:"independentTest"`
			DependentTest   string            `json:"dependentTest"`
			Anonymous       string            `json:"anonymous"`
			Named           string            `json:"named"`
		} `json:"cases"`
	}
	raw, err = os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 4 {
		t.Fatalf("cases=%d", len(data.Cases))
	}
	for _, c := range data.Cases {
		t.Run(c.API+"/"+c.ID, func(t *testing.T) {
			root := t.TempDir()
			for path, source := range c.Files {
				writeTestFile(t, filepath.Join(root, path), source)
			}
			for _, folder := range []string{"base", "bad", "loaded"} {
				if err := os.MkdirAll(filepath.Join(root, folder), 0755); err != nil {
					t.Fatal(err)
				}
			}
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"base","default":true},{"path":"bad"},{"path":"loaded"}],"sourceApiVersion":"`+c.API+`"}`)
			observed := func(route string) bool {
				t.Helper()
				value, ok := oracle[c.API+"/"+c.ID+"/"+route]
				if !ok {
					t.Fatalf("missing native %s row", route)
				}
				return value
			}
			accepted := observed("metadata_deploy")
			t.Run("check", func(t *testing.T) {
				var out, errs bytes.Buffer
				code := Run(context.Background(), []string{"check", "--project", root, "--no-cache", "--no-progress", "--json"}, &out, &errs)
				if (code == 0) != accepted || code > 1 {
					t.Fatalf("native accepted=%t code=%d out=%s stderr=%s", accepted, code, out.String(), errs.String())
				}
				var response struct{ Diagnostics []struct{ Code, File string } }
				if err := json.Unmarshal(out.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, d := range response.Diagnostics {
					if d.Code == "GLADESEMA003" {
						count++
						// CLI JSON uses project-relative paths; sema and LSP retain
						// the absolute metadata source checked by their route twins.
						if d.File != c.FieldPath {
							t.Errorf("check diagnostic file=%q want=%q", d.File, c.FieldPath)
						}
					}
				}
				if !accepted && count != 2 {
					t.Fatalf("lookup errors=%d; out=%s", count, out.String())
				}
			})
			t.Run("runner_istest", func(t *testing.T) {
				if !observed("istest") {
					t.Fatal("native test did not pass")
				}
				var out, errs bytes.Buffer
				code := Run(context.Background(), []string{"test", "--project", root, "--no-cache", "--no-serve", "--no-progress", "--json"}, &out, &errs)
				if (code == 0) != accepted || code > 1 {
					t.Fatalf("runner code=%d accepted=%t out=%s stderr=%s", code, accepted, out.String(), errs.String())
				}
				run, err := decodeTestRunJSON(out.Bytes())
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]testreport.Status{}
				for _, suite := range run.Suites {
					for _, test := range suite.Cases {
						got[test.ClassName] = test.Status
					}
				}
				dependent := testreport.StatusCompileError
				wantCases := 3
				if accepted {
					dependent = testreport.StatusPass
					wantCases = 2
				}
				if !accepted && got["project"] != testreport.StatusCompileError {
					t.Fatalf("unreferenced generated ChangeEvent metadata error must remain a project failure: %v", got)
				}
				if len(got) != wantCases || got[c.IndependentTest] != testreport.StatusPass || got[c.DependentTest] != dependent {
					t.Fatalf("native unrelated test passes; want independent=pass dependent=%s; got %v", dependent, got)
				}
			})
			for _, route := range []string{"anonymous", "named"} {
				t.Run("exec_"+route, func(t *testing.T) {
					source := c.Anonymous
					if route == "named" {
						source = c.Named
					}
					var out, errs bytes.Buffer
					code := Run(context.Background(), []string{"exec", "--project", root, "--json", source}, &out, &errs)
					if (code == 0) != observed(route) || code > 1 {
						t.Fatalf("native success=%t code=%d out=%s stderr=%s", observed(route), code, out.String(), errs.String())
					}
				})
			}
			t.Run("lsp_metadata", func(t *testing.T) {
				index, err := loadIndex(root)
				if err != nil {
					t.Fatal(err)
				}
				handler := lsp.NewHandler(index)
				defer handler.HandleJSON([]byte(`{"jsonrpc":"2.0","id":1,"method":"shutdown"}`))
				uri := lsp.DocumentURI((&url.URL{Scheme: "file", Path: filepath.Join(root, c.FieldPath)}).String())
				// didClose republishes the project analysis without parsing XML as Apex.
				notes := handler.DidClose(lsp.DidCloseTextDocumentParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
				count := 0
				for _, note := range notes {
					payload, ok := note.Params.(lsp.PublishDiagnosticsParams)
					if !ok {
						t.Fatalf("unexpected payload %T", note.Params)
					}
					for _, d := range payload.Diagnostics {
						if d.Code == "GLADESEMA003" {
							count++
						}
					}
				}
				want := 0
				if !accepted {
					want = 2
				}
				if count != want {
					t.Fatalf("LSP lookup diagnostics=%d want=%d: %+v", count, want, notes)
				}
			})
		})
	}
}
