package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The rejection is the anonymous observation in standard_field_access/org.tsv.
// Both server transports delegate to the same anonymous semantic check.
func TestStandardFieldAccessServerRoutes(t *testing.T) {
	root := t.TempDir()
	shape := schema.Schema{Objects: []schema.Object{{Name: "Account", CompleteSnapshot: true}, {Name: "Contact", CompleteSnapshot: true}}}
	data, err := json.Marshal(shape)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.ReplaceAll(string(data), `"completeSnapshot"`, `"fieldsComplete"`))
	if err := os.WriteFile(filepath.Join(root, "snapshot.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(data)
	if err := os.WriteFile(filepath.Join(root, "glade.yml"), []byte(fmt.Sprintf("project:\n  schemaSnapshot: snapshot.json\n  schemaSnapshotSHA256: %x\n", digest)), 0600); err != nil {
		t.Fatal(err)
	}
	org := testOrg()
	handler := New(&org)
	p := project.Project{Root: root, SchemaSnapshot: filepath.Join(root, "snapshot.json"), SchemaSnapshotSHA256: fmt.Sprintf("%x", digest)}
	loaded, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, loaded)
	handler.Index = &index
	for _, api := range []string{"62.0", "67.0"} {
		for _, source := range []string{"Object x = new Account().BogusField__c;", "Object x = new Contact().OtherMissing__c;"} {
			t.Run(api+"/"+source, func(t *testing.T) {
				payload, err := json.Marshal(map[string]string{"anonymousBody": source})
				if err != nil {
					t.Fatal(err)
				}
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", strings.NewReader(string(payload))))
				if api == "62.0" {
					if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "unsupported API version") {
						t.Fatalf("REST version boundary status=%d body=%s", rec.Code, rec.Body.String())
					}
				} else if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"compiled":false`) || !strings.Contains(rec.Body.String(), "Variable does not exist") {
					t.Fatalf("REST status=%d body=%s", rec.Code, rec.Body.String())
				}
				rec = httptest.NewRecorder()
				path := "/services/Soap/s/" + api + "/00DLOCAL000000001"
				body := soapEnvelope("<apex:executeAnonymous><apex:String>" + source + "</apex:String></apex:executeAnonymous>")
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
				if api == "62.0" {
					if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "INVALID_VERSION") {
						t.Fatalf("SOAP version boundary status=%d body=%s", rec.Code, rec.Body.String())
					}
					return
				}
				result := decodeSOAPResult(t, rec.Body.String())
				if rec.Code != http.StatusOK || result["compiled"] != "false" || !strings.Contains(result["compileProblem"], "Variable does not exist") {
					t.Fatalf("SOAP status=%d body=%s", rec.Code, rec.Body.String())
				}
			})
		}
	}
}
