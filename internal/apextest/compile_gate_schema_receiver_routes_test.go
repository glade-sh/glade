package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native trigger constructor/index/get/loop receivers and a singleton query
// selecting only Id keep attribution separate from a query's selected fields.
func TestSchemaReferenceGateTriggerAndSingletonReceiverRoutes(t *testing.T) {
	var data struct {
		Receivers []struct{ API, Trigger, Singleton string }
	}
	raw, err := os.ReadFile("../sema/testdata/schema_relationship_gate/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Receivers) != 2 {
		t.Fatalf("native receiver APIs=%d", len(data.Receivers))
	}
	for _, c := range data.Receivers {
		routes := map[string]string{"query_singleton": c.Singleton}
		for name, body := range map[string]string{"constructor": "Object a=new Contact().Account;", "indexed": "Object b=Trigger.new[0].Account;", "loop": "for(Contact c:Trigger.new){Object d=c.Account;}", "get": "Object e=Trigger.new.get(0).Account;"} {
			routes["trigger_"+name] = strings.SplitN(c.Trigger, "{", 2)[0] + "{" + body + "}"
		}
		for route, source := range routes {
			t.Run(c.API+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				file := filepath.Join(root, "Probe.cls")
				if strings.HasPrefix(route, "trigger_") {
					file = filepath.Join(root, "Probe.trigger")
				}
				writeFile(t, file, source)
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: c.API, ApexFiles: []string{file}}, schemaRelationshipGateMetadata("Account"))
				units := newCompileGateUnits(index)
				// Warm the shared expression model before parallel readers.
				if !compileGateReferencesSchemaObject(file, source, units, "Account") {
					t.Fatal("initial relationship receiver lost Account ownership")
				}
				for _, repeat := range []string{"warm_repeat", "shared_parallel_a", "shared_parallel_b"} {
					t.Run(repeat, func(t *testing.T) {
						t.Parallel()
						if !compileGateReferencesSchemaObject(file, source, units, "Account") {
							t.Fatal("typed relationship receiver lost Account ownership")
						}
					})
				}
			})
		}
	}
}
