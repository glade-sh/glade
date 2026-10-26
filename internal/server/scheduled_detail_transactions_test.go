package server

import (
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

// The same owned project is natively captured at APIs 62/67. These requests
// exercise the Apex REST entry point that clones a VM over persistent storage.
func TestScheduledDetailServerSharedOrgRoutes(t *testing.T) {
	source, err := os.ReadFile("../apextest/testdata/conformance/scheduled_detail_transactions/project/force-app/main/default/classes/CedarTransactionProbe.cls")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"apexREST"} {
			for _, batch := range []bool{false, true} {
				t.Run(fmt.Sprintf("api%s/%s/batch=%v", api, route, batch), func(t *testing.T) {
					org := testOrg()
					handler := New(&org)
					sources := map[string]string{
						"CedarTransactionProbe.cls": string(source),
						"CedarResource.cls": fmt.Sprintf(`@RestResource(urlMapping='/cedar/*') global class CedarResource {
       @HttpPost global static String submit() {
        String name = RestContext.request.params.get('name');
        Id ticket = CedarTransactionProbe.submit(%v, name);
        CronTrigger job = [SELECT CronJobDetail.Name FROM CronTrigger WHERE Id = :ticket];
        System.assertEquals(name, job.CronJobDetail.Name);
        return ticket;
       }
      }`, batch),
					}
					root := t.TempDir()
					var files []string
					for name, source := range sources {
						path := filepath.Join(root, name)
						if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
							t.Fatal(err)
						}
						files = append(files, path)
					}
					index := typesys.Build(project.Project{Root: root, ApexFiles: files, SourceAPIVersion: api}, schema.Schema{})
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					handler.SetProjectIndex(index)
					var firstID string
					for _, name := range []string{"cedar first transaction", "cedar second transaction"} {
						path := "/services/apexrest/cedar/submit?name=" + strings.ReplaceAll(name, " ", "+")
						body := ""

						rec := httptest.NewRecorder()
						handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
						if rec.Code != http.StatusOK {
							t.Fatalf("%s: status=%d body=%s", name, rec.Code, rec.Body.String())
						}
						var id string
						if err := json.Unmarshal(rec.Body.Bytes(), &id); err != nil {
							t.Fatal(err)
						}
						if id == firstID {
							t.Fatal("REST requests reused trigger identity")
						}
						firstID = id
					}
					if len(org.Objects["CronJobDetail"].Records) != 2 {
						t.Fatal("two HTTP requests did not preserve distinct details")
					}
				})
			}
		}
	}
}
