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

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Actual imperative LWC transport requests use the marked global describe
// route in compiled Apex. Bundle versions do not retarget the Apex class; both
// supported Apex endpoints are checked with each host version and warm calls.
func TestLightningDescribeHolderRequestIsolation(t *testing.T) {
	source, err := os.ReadFile("../vm/testdata/describe_ownership/DescribeOwner.cls")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, host := range []string{"59.0", "62.0", "67.0"} {
			for _, namespace := range []string{"", "zzns"} {
				t.Run(api+"/host="+host+"/"+namespace, func(t *testing.T) {
					root := t.TempDir()
					classes := filepath.Join(root, "force-app/main/default/classes")
					writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q,"namespace":%q}`, api, namespace))
					writeLightningFixtureFile(t, filepath.Join(classes, "DescribeOwner.cls"), string(source))
					writeLightningFixtureFile(t, filepath.Join(classes, "DescribeOwner.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
					bundle := filepath.Join(root, "force-app/main/default/lwc/describeOwner")
					writeLightningFixtureFile(t, filepath.Join(bundle, "describeOwner.js"), `import verify from '@salesforce/apex/DescribeOwner.verify'; export default verify;`)
					writeLightningFixtureFile(t, filepath.Join(bundle, "describeOwner.js-meta.xml"), "<LightningComponentBundle><apiVersion>"+host+"</apiVersion><isExposed>true</isExposed></LightningComponentBundle>")
					p, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					schema, err := gladeschema.LoadProject(p)
					if err != nil {
						t.Fatal(err)
					}
					metadata, err := NewSourceMetadataFromProject(p)
					if err != nil {
						t.Fatal(err)
					}
					org := storage.NewOrgState()
					org.Namespace = namespace
					for _, name := range []string{"Account", "User", "RecordType"} {
						storage.EnsureStandardObject(&org, name)
					}
					handler := NewWithStoreAndSource(&org, &memoryStore{}, metadata)
					handler.SetProjectIndex(typesys.Build(p, schema))
					for range 3 {
						request := httptest.NewRequest(http.MethodPost, "/lightning/apex/DescribeOwner/verify", strings.NewReader(`{}`))
						request.Header.Set("Content-Type", "application/json")
						rec := httptest.NewRecorder()
						handler.ServeHTTP(rec, request)
						var result lwcbrowser.WireResponse
						if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
							t.Fatalf("HTTP %d: %s: %v", rec.Code, rec.Body.String(), err)
						}
						if rec.Code != http.StatusOK || result.Error != nil || result.Data != "owned-describe-ok" {
							t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
						}
					}
				})
			}
		}
	}
}
