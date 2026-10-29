package lwc_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/project"
)

// Synthetic import routes share native schema membership from
// standard_describe_profiles/P11/org.tsv:2577,2586,2602,12066,12075,12091
// and standard_recordtype_field_policy/org.tsv:16-25 (their ORG-RUN.txt files).
// P0's API62/67 named maps supply the bare Case absence controls. This is
// configured schema propagation through both compilers, not a native LWC capture.
func TestStandardFieldPolicyConfiguredLWCSchemaImportRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"configured", "bare-case"} {
			t.Run(api+"/"+shape, func(t *testing.T) {
				root := t.TempDir()
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
				fields := []struct{ object, name, xmlType, extra string }{
					{"Case", "HasCommentsUnreadByOwner", "Checkbox", "<defaultValue>false</defaultValue>"},
					{"Case", "IsVisibleInSelfService", "Checkbox", "<defaultValue>false</defaultValue>"},
					{"Case", "SourceId", "Lookup", "<referenceTo>EmailMessage</referenceTo><relationshipName>Source</relationshipName>"},
					{"Account", "RecordTypeId", "Lookup", "<referenceTo>RecordType</referenceTo><relationshipName>RecordType</relationshipName>"},
				}
				if shape == "bare-case" {
					fields = fields[:3]
				}
				for i, field := range fields {
					if shape == "configured" {
						xml := "<CustomField><fullName>" + field.name + "</fullName><type>" + field.xmlType + "</type>" + field.extra + "</CustomField>"
						write(filepath.Join("force-app/main/default/objects", field.object, "fields", field.name+".field-meta.xml"), xml)
					}
					bundle := fmt.Sprintf("policyField%d", i)
					base := filepath.Join("force-app/main/default/lwc", bundle, bundle)
					write(base+".js", "import { LightningElement } from 'lwc';\nimport field from '@salesforce/schema/"+field.object+"."+field.name+"';\nexport default class PolicyField extends LightningElement { fieldValue=field; }\n")
					write(base+".html", "<template><span>{fieldValue}</span></template>")
					write(base+".js-meta.xml", `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`)
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				if len(p.LWCMetaFiles) != len(fields) {
					t.Fatalf("loaded %d bundles, want %d", len(p.LWCMetaFiles), len(fields))
				}
				check := func(t *testing.T, object, field, bundle string, manifest compile.Manifest, compileErr error) {
					t.Helper()
					if shape == "configured" {
						if compileErr != nil {
							t.Fatalf("configured field import rejected: %v", compileErr)
						}
						if _, exists := manifest.Modules["c:"+bundle]; !exists {
							t.Fatalf("compiled module c:%s missing", bundle)
						}
					} else if compileErr == nil || !strings.Contains(compileErr.Error(), "Invalid reference "+object+"."+field+" of type sobjectField") {
						t.Fatalf("bare Case import needs field rejection, got %v", compileErr)
					}
				}
				t.Run("batch", func(t *testing.T) {
					results, err := compile.CompileBatch(p, compile.Options{OutDir: filepath.Join(root, "dist-batch"), Namespace: "c"})
					if err != nil {
						t.Fatalf("compiler setup/process failure: %v", err)
					}
					for i, field := range fields {
						bundle := fmt.Sprintf("policyField%d", i)
						result, exists := results["force-app/main/default/lwc/"+bundle]
						if !exists {
							t.Fatalf("missing batch result for %s", bundle)
						}
						t.Run(field.object+"."+field.name, func(t *testing.T) {
							check(t, field.object, field.name, bundle, result.Manifest, result.Err)
						})
					}
				})
				for i, field := range fields {
					t.Run("single/"+field.object+"."+field.name, func(t *testing.T) {
						bundle := fmt.Sprintf("policyField%d", i)
						base := filepath.Join(root, "force-app/main/default/lwc", bundle, bundle)
						single := p
						single.LWCFiles = []string{base + ".js"}
						single.LWCHTMLFiles = []string{base + ".html"}
						single.LWCMetaFiles = []string{base + ".js-meta.xml"}
						manifest, err := compile.Compile(single, compile.Options{OutDir: filepath.Join(root, "dist-single", bundle), Namespace: "c"})
						check(t, field.object, field.name, bundle, manifest, err)
					})
				}
			})
		}
	}
}
