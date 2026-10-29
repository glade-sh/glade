package vm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Preserve release behavior outside the captured unmatched Id insertion routes.
// This intentionally makes no native parity claim for these deferred shapes.
func TestUpsertDuplicateUncapturedRoutesRetainReleaseBehavior(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/dml_policies"
	source, err := os.ReadFile(filepath.Join(fixture, "upsert_release_controls.apex"))
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := project.Load(filepath.Join(fixture, "project"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"custom_insert", "implicit_id_update", "explicit_id_update", "custom_update"} {
			for _, route := range []string{"keyword", "database_partial", "database_atomic"} {
				t.Run(api+"/"+shape+"/"+route, func(t *testing.T) {
					body := strings.ReplaceAll(string(source), "new List<String>{'custom_insert','implicit_id_update','explicit_id_update','custom_update'}", "new List<String>{'"+shape+"'}")
					body = strings.ReplaceAll(body, "new List<String>{'keyword','database_partial','database_atomic'}", "new List<String>{'"+route+"'}")
					org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
					storage.EnsureStandardObject(&org, "Contact")
					contact := org.Objects["Contact"]
					contact.Definition.Fields["RouteKey__c"] = storage.Field{APIName: "RouteKey__c", Type: storage.FieldString, ExternalID: true}
					org.Objects["Contact"] = contact
					installDMLPolicyAutomation(t, &org, loaded)
					program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := New(nil)
					machine.SetOrg(&org)
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
