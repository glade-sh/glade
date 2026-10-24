package vm_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

var metadataDefaultRoutes = []struct{ name, expression string }{
	{"new", "new DefaultProbe__mdt()"},
	{"json-typed", "(DefaultProbe__mdt)JSON.deserialize('{}',DefaultProbe__mdt.class)"},
	{"json-generic", "(DefaultProbe__mdt)JSON.deserialize('{\"attributes\":{\"type\":\"DefaultProbe__mdt\"}}',SObject.class)"},
	{"json-map-dynamic", "(DefaultProbe__mdt)JSON.deserialize(JSON.serialize(new Map<String,Object>{'DeveloperName'=>'Product','MasterLabel'=>'Product'}),Type.forName('Schema.DefaultProbe__mdt'))"},
	{"newSObject", "(DefaultProbe__mdt)DefaultProbe__mdt.SObjectType.newSObject()"},
	{"newSObject-false", "(DefaultProbe__mdt)DefaultProbe__mdt.SObjectType.newSObject(null,false)"},
	{"newSObject-true", "(DefaultProbe__mdt)DefaultProbe__mdt.SObjectType.newSObject(null,true)"},
	{"clone", "new DefaultProbe__mdt().clone(false,true,false,false)"},
	{"clone-explicit", "new DefaultProbe__mdt(Priority__c=7,Enabled__c=true,Title__c='set',Choice__c='Second').clone(false,true,false,false)"},
}

// Each route runs as an Apex test in an imported synthetic metadata project.
// The sprout variant tests local canonical identity, not managed-org parity.
func TestMetadataDefaultRoutesProjectNativeConformance(t *testing.T) {
	testDefaultRoutesProjectNativeConformance(t, "DefaultProbe__mdt", "routes-org.tsv", []string{"", "sprout"})
}

func TestCustomObjectDefaultRoutesProjectNativeConformance(t *testing.T) {
	testDefaultRoutesProjectNativeConformance(t, "DefaultThing__c", "object-routes-org.tsv", []string{""})
}

func testDefaultRoutesProjectNativeConformance(t *testing.T, objectName, oracle string, namespaces []string) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	f, err := os.Open(filepath.Join("testdata/constructed_metadata_defaults", oracle))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 235 {
		t.Fatalf("want header + 234 route observations; got %d", len(rows))
	}
	for _, namespace := range namespaces {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(namespace+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				if err := os.CopyFS(root, os.DirFS("testdata/constructed_metadata_defaults")); err != nil {
					t.Fatal(err)
				}
				config := fmt.Sprintf(`{"namespace":%q,"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, namespace, api)
				if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(config), 0600); err != nil {
					t.Fatal(err)
				}
				var source strings.Builder
				source.WriteString("@IsTest private class MetadataRouteProof {\n")
				for _, route := range metadataDefaultRoutes {
					expression := strings.ReplaceAll(route.expression, "DefaultProbe__mdt", objectName)
					if objectName == "DefaultThing__c" {
						expression = strings.ReplaceAll(expression, "'DeveloperName'=>'Product','MasterLabel'=>'Product'", "'Name'=>'Product'")
					}
					fmt.Fprintf(&source, "@IsTest static void route_%s() { %s row=%s;\n", strings.ReplaceAll(route.name, "-", "_"), objectName, expression)
					checked := 0
					for _, observation := range rows[1:] {
						if observation[0] != api || observation[1] != route.name {
							continue
						}
						field, access, want := observation[2], observation[3], observation[4]
						var expression string
						switch access {
						case "dot":
							expression = "row." + field
						case "get":
							expression = "row.get('" + field + "')"
						case "populated":
							expression = "row.getPopulatedFieldsAsMap().containsKey('" + field + "')"
						case "dto":
							fmt.Fprintf(&source, "Metadata.CustomMetadata dto=new Metadata.CustomMetadata();dto.fullName=row.getSObjectType().getDescribe().getName().removeEnd('%s')+'.Product';\n", map[bool]string{true: "__mdt", false: "__c"}[strings.HasSuffix(objectName, "__mdt")])
							expression = "dto.fullName"
							if namespace != "" {
								want = namespace + "__" + want
							}
						default:
							t.Fatalf("unexpected access %s", access)
						}
						fmt.Fprintf(&source, "System.assertEquals('%s',(''+String.valueOf(%s)), '%s/%s/%s');\n", want, expression, route.name, field, access)
						checked++
					}
					if checked != 13 {
						t.Fatalf("%s has %d observations", route.name, checked)
					}
					source.WriteString("}\n")
				}
				source.WriteString("}\n")
				classPath := filepath.Join(root, "force-app/main/default/classes/MetadataRouteProof.cls")
				if err := os.WriteFile(classPath, []byte(source.String()), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(classPath+"-meta.xml", []byte(fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api)), 0600); err != nil {
					t.Fatal(err)
				}
				observerName := "RouteObservation"
				if objectName == "DefaultThing__c" {
					observerName = "ObjectRouteObservation"
				}
				observerName += strings.TrimSuffix(api, ".0")
				observerSource, err := os.ReadFile(filepath.Join(root, "force-app/main/default/classes", observerName+".cls"))
				if err != nil {
					t.Fatal(err)
				}
				// Host twins reuse the captured observer body; rows are returned instead of debugged.
				hosts := map[string]string{"lwc": "RouteHostLWC" + strings.TrimSuffix(api, ".0"), "visualforce": "RouteHostVF" + strings.TrimSuffix(api, ".0")}
				for kind, hostName := range hosts {
					signature := "global String observe() {"
					if kind == "lwc" {
						signature = "@AuraEnabled global static String observe() {"
					}
					host := strings.Replace(string(observerSource), "public class "+observerName, "global with sharing class "+hostName, 1)
					host = strings.Replace(host, "public static void observe() {", signature+" List<String> rows=new List<String>();", 1)
					host = strings.ReplaceAll(host, "System.debug(", "rows.add(")
					end := strings.LastIndex(host, "\n} }")
					if end < 0 || !strings.Contains(host, "rows.add(") {
						t.Fatalf("cannot derive host class for %s", observerName)
					}
					host = host[:end] + "\nreturn String.join(rows,'\\n');\n} }\n"
					hostPath := filepath.Join(root, "force-app/main/default/classes", hostName+".cls")
					if err := os.WriteFile(hostPath, []byte(host), 0600); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(hostPath+"-meta.xml", []byte(fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api)), 0600); err != nil {
						t.Fatal(err)
					}
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				s, err := schema.LoadProject(p)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, s)
				checkRoutes := func(t *testing.T, lines []string) {
					got := map[string]string{}
					for _, line := range lines {
						if strings.HasPrefix(line, "ROUTE|") {
							parts := strings.Split(line, "|")
							if len(parts) != 5 {
								t.Fatalf("invalid route row %q", line)
							}
							key := strings.Join(parts[1:4], "/")
							if _, exists := got[key]; exists {
								t.Fatalf("duplicate route %s", key)
							}
							got[key] = parts[4]
						}
					}
					checked := 0
					for _, row := range rows[1:] {
						if row[0] != api {
							continue
						}
						key := strings.Join(row[1:4], "/")
						want := row[4]
						if namespace != "" && row[3] == "dto" {
							want = namespace + "__" + want
						}
						if got[key] != want {
							t.Errorf("%s native=%s Glade=%s", key, want, got[key])
						}
						checked++
					}
					if len(got) != checked || checked != 117 {
						t.Fatalf("native=%d Glade=%d", checked, len(got))
					}
				}
				newHostMachine := func(t *testing.T) *vm.VM {
					machine := vm.New(nil)
					org := apextest.OrgFromIndex(index)
					org.APIVersion = api
					machine.SetOrg(&org)
					if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
						t.Fatal(err)
					}
					return machine
				}
				t.Run("lwc-host", func(t *testing.T) {
					result, err := newHostMachine(t).InvokeLWCMethod(hosts["lwc"], "observe", nil)
					if err != nil || !result.Success {
						t.Fatalf("lwc host: %v %#v", err, result.Error)
					}
					text, ok := result.ReturnValue.(string)
					if !ok {
						t.Fatalf("lwc host return %#v", result.ReturnValue)
					}
					checkRoutes(t, strings.Split(text, "\n"))
				})
				t.Run("visualforce-host", func(t *testing.T) {
					result, err := newHostMachine(t).InvokeVisualforceAction(hosts["visualforce"], "observe", "/apex/RouteHost", nil)
					if err != nil || !result.Success {
						t.Fatalf("visualforce host: %v %#v", err, result.Error)
					}
					text, ok := result.ReturnValue.(string)
					if !ok {
						t.Fatalf("visualforce host return %#v", result.ReturnValue)
					}
					checkRoutes(t, strings.Split(text, "\n"))
				})
				// Anonymous Apex runs outside the package namespace and cannot see public classes.
				if namespace == "" {
					t.Run("anonymous-named-observer", func(t *testing.T) {
						machine := vm.New(nil)
						org := apextest.OrgFromIndex(index)
						org.APIVersion = api
						machine.SetOrg(&org)
						if err := apextest.RegisterProjectRuntime(machine, index); err != nil {
							t.Fatal(err)
						}
						observer := "RouteObservation"
						if objectName == "DefaultThing__c" {
							observer = "ObjectRouteObservation"
						}
						program, err := vm.CompileAnonymousWithOptions(observer+strings.TrimSuffix(api, ".0")+".observe();", vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						result, err := machine.Execute(program)
						if err != nil {
							t.Fatal(err)
						}
						checkRoutes(t, result.Debug)
					})
				}
				for _, mode := range []struct {
					name        string
					parallelism int
				}{{"isTest-cold-serial", 1}, {"isTest-cached-parallel", 2}} {
					t.Run(mode.name, func(t *testing.T) {
						report := apextest.Run(index, apextest.Options{NoDiskCache: true, Parallelism: mode.parallelism, ParallelMethods: mode.parallelism > 1})
						for _, suite := range report.Suites {
							for _, c := range suite.Cases {
								t.Run(c.MethodName, func(t *testing.T) {
									if c.Problem != nil {
										t.Errorf("%s: %s", c.Problem.Type, c.Problem.Message)
									}
								})
							}
						}
						if summary := report.Summary(); summary.Total != len(metadataDefaultRoutes) || summary.Passed != summary.Total {
							t.Fatalf("metadata route project: %+v", summary)
						}
					})
				}
			})
		}
	}
}
