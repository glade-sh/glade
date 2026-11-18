package server

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// The native anonymous rows are replayed through the local Tooling API route.
// The rows establish Salesforce anonymous execution, not Salesforce server-host
// behavior for Glade's local endpoint. The Tooling endpoint uses API 67
// for both rows; the captured class metadata retains each source API version.
// The 62.0 row is not an anonymous-execution API 62 route comparison.
func TestCapturedEnclosingStaticToolingAnonymousRoute(t *testing.T) {
	const fixture = "testdata/conformance/enclosing_static_anonymous"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || strings.Join(rows[0], ",") != "case,api,route,outcome" {
		t.Fatal("missing native row header")
	}
	source, err := os.ReadFile(filepath.Join(fixture, "inherited.apex"))
	if err != nil {
		t.Fatal(err)
	}
	parent, child, ok := strings.Cut(string(source), "\n")
	if !ok {
		t.Fatal("missing captured class sources")
	}
	anonymous, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || row[0] != "inherited_runtime" {
			continue
		}
		api := row[1]
		if (api != "62.0" && api != "67.0") || row[2] != "anonymous" || row[3] != "passed" || seen[api] {
			t.Fatalf("invalid native row: %v", row)
		}
		seen[api] = true
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			classes := filepath.Join(root, "force-app/main/default/classes")
			for name, body := range map[string]string{"Parent": parent, "Probe": child} {
				writeServerTestFile(t, filepath.Join(classes, name+".cls"), body)
				writeServerTestFile(t, filepath.Join(classes, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, sch)
			if index.HasErrors() {
				t.Fatalf("captured project could not be indexed: %v", index.Diagnostics)
			}
			org := storage.NewOrgState()
			org.APIVersion = "67.0"
			srv := newLightningTestServer(t, &org, metadata)
			srv.SetProjectIndex(index)
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			payload, err := json.Marshal(map[string]string{"anonymousBody": string(anonymous)})
			if err != nil {
				t.Fatal(err)
			}
			for attempt := 0; attempt < 2; attempt++ {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", bytes.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("attempt %d: status %d: %s", attempt, rec.Code, rec.Body.String())
				}
				var result struct {
					Compiled bool `json:"compiled"`
					Success  bool `json:"success"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || !result.Compiled || !result.Success {
					t.Fatalf("attempt %d: response %s: %v", attempt, rec.Body.String(), err)
				}
			}
		})
	}
	if !seen["62.0"] || !seen["67.0"] || len(seen) != 2 {
		t.Fatalf("incomplete native API matrix: %v", seen)
	}
}
