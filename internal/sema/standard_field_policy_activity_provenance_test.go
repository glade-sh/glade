package sema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native membership: standard_describe_profiles/P1/org.tsv:708,2120,3199,
// 4626,10664,12224,13317,14892 and P1/ORG-RUN.txt. Explicit Activity metadata
// supplies the captured field to both concrete objects without an org feature
// marker. Constructor admission must see the same inherited schema as setters.
func TestStandardFieldPolicyConfiguredActivitySemaRouteTwins(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, object := range []string{"Task", "Event"} {
			for _, route := range []string{"anonymous", "named", "is-test"} {
				for _, operation := range []string{"constructor", "static-setter"} {
					t.Run(api+"/"+object+"/"+route+"/"+operation, func(t *testing.T) {
						body := configuredActivityPolicyBody(object, operation)
						index := configuredActivityPolicyIndex(t, api, route, body)
						result := Analyze(index)
						if route == "anonymous" {
							result = AnalyzeAnonymous(index, body, api)
						}
						if result.HasErrors() {
							t.Fatalf("configured Activity field rejected on %s: %v", route, result.Diagnostics)
						}
					})
				}
			}
		}
	}
}

// Exercise the new constructor-definition cache through the public anonymous
// entry point: successive and concurrent bodies reuse one immutable setup.
// The native field rows are the same API62/67 P1 rows cited above.
func TestStandardFieldPolicyConfiguredActivityCachedParallelAnonymousRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := configuredActivityPolicyIndex(t, api, "anonymous", "")
			setup := anonymousSetupFor(index, api)
			original, err := json.Marshal(index.Objects)
			if err != nil {
				t.Fatal(err)
			}
			var bodies []string
			for _, object := range []string{"Task", "Event"} {
				for _, operation := range []string{"constructor", "static-setter"} {
					bodies = append(bodies, configuredActivityPolicyBody(object, operation))
				}
			}
			for _, pass := range []string{"cold-constructor-cache", "warm-constructor-cache"} {
				t.Run(pass, func(t *testing.T) {
					for _, body := range bodies {
						if result := AnalyzeAnonymous(index, body, api); result.HasErrors() {
							t.Errorf("cached inherited field rejected: %v", result.Diagnostics)
						}
					}
				})
			}
			t.Run("parallel-shared-cache", func(t *testing.T) {
				results := make(chan Result, len(bodies))
				var workers sync.WaitGroup
				for _, body := range bodies {
					workers.Add(1)
					go func(body string) {
						defer workers.Done()
						results <- AnalyzeAnonymous(index, body, api)
					}(body)
				}
				workers.Wait()
				close(results)
				for result := range results {
					if result.HasErrors() {
						t.Errorf("parallel inherited field rejected: %v", result.Diagnostics)
					}
				}
			})
			current, err := json.Marshal(index.Objects)
			if err != nil || !bytes.Equal(current, original) {
				t.Fatalf("constructor cache mutated declared schema: %v", err)
			}
			if anonymousSetupFor(index, api) != setup {
				t.Fatal("constructor routes did not reuse the same anonymous setup")
			}
		})
	}
}

func configuredActivityPolicyBody(object, operation string) string {
	if operation == "constructor" {
		return fmt.Sprintf("%s row=new %s(CurrencyIsoCode='USD'); System.assertEquals('USD',row.CurrencyIsoCode);", object, object)
	}
	return fmt.Sprintf("%s row=new %s(); row.CurrencyIsoCode='USD'; System.assertEquals('USD',row.CurrencyIsoCode);", object, object)
}

func configuredActivityPolicyIndex(t *testing.T, api, route, body string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config/project-scratch-def.json"), []byte(`{"edition":"Developer"}`), 0600); err != nil {
		t.Fatal(err)
	}

	write := func(relative, content string) {
		t.Helper()
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("sfdx-project.json", `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	write("force-app/main/default/objects/Activity/fields/CurrencyIsoCode.field-meta.xml", `<CustomField><fullName>CurrencyIsoCode</fullName><type>Picklist</type></CustomField>`)
	if route != "anonymous" {
		source := "public class ConfiguredActivityPolicy { public static void sample(){" + body + "} }"
		if route == "is-test" {
			source = "@IsTest private class ConfiguredActivityPolicy { @IsTest static void sample(){" + body + "} }"
		}
		write("force-app/main/default/classes/ConfiguredActivityPolicy.cls", source)
		write("force-app/main/default/classes/ConfiguredActivityPolicy.cls-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Objects) != 1 || s.Objects[0].Name != "Activity" || len(s.Objects[0].Fields) != 1 || s.Objects[0].Fields[0].Name != "CurrencyIsoCode" {
		t.Fatalf("fixture must configure only the inherited Activity field: %+v", s.Objects)
	}
	index := typesys.Build(p, s)
	if index.HasErrors() || len(index.OrgShapeFeatures) != 0 {
		t.Fatalf("configured-field fixture must compile without org features: %v, %v", index.Diagnostics, index.OrgShapeFeatures)
	}
	return index
}
