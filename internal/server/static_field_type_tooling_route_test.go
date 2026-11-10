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

// Captured anonymous execution replayed through the local Tooling API.
// Source classes use APIs 62 and 67; the local endpoint uses API 67.
func TestCapturedStaticFieldTypeToolingAnonymousRoute(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/static_field_type_collision"
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
	source, err := os.ReadFile(filepath.Join(fixture, "Parcel.apex"))
	if err != nil {
		t.Fatal(err)
	}
	anonymous, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	rejectedSource, err := os.ReadFile(filepath.Join(fixture, "reject.apex"))
	if err != nil {
		t.Fatal(err)
	}
	rejectionRows := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) == 4 && row[0] == "reject" && row[2] == "anonymous" && strings.Contains(row[3], "Method does not exist") {
			rejectionRows[row[1]] = true
		}
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || row[0] != "anonymous" {
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
			for name, body := range map[string]string{"Parcel": string(source)} {
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
			if !rejectionRows[api] {
				t.Fatalf("missing native rejection at %s", api)
			}
			rejectedPayload, err := json.Marshal(map[string]string{"anonymousBody": string(rejectedSource)})
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", bytes.NewReader(rejectedPayload))
			req.Header.Set("Content-Type", "application/json")
			srv.ServeHTTP(rec, req)
			var rejection struct {
				Compiled bool `json:"compiled"`
			}
			if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &rejection) != nil || rejection.Compiled {
				t.Fatalf("native rejection accepted: status %d response %s", rec.Code, rec.Body.String())
			}
		})
	}
	if !seen["62.0"] || !seen["67.0"] || len(seen) != 2 {
		t.Fatalf("incomplete native API matrix: %v", seen)
	}
}
