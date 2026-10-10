package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Owned observations at both source API endpoints, including the configured
// partition records and builder classes. Neither route needs an org connection.
func TestPlatformCacheOrgConformance(t *testing.T) {
	type familyCase struct {
		ID, Code, Expected      string
		Compile, ExpectedNull   bool
		CarryOwner, CarryReason string
		NativeDiagnostic        string
	}
	type metadataCase struct {
		ID, Expected string
		Metadata     map[string]string
	}
	var data struct {
		APIVersions      []string `json:"apiVersions"`
		Declarations     map[string]string
		Metadata         map[string]string
		Fixture          storage.Fixture
		Cases, Controls  []familyCase
		MetadataControls []metadataCase
		Boundaries       []string
	}
	raw, err := os.ReadFile("testdata/conformance/platform_cache.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cases) != 295 || len(data.Controls) != 21 || len(data.MetadataControls) != 1 || strings.Join(data.APIVersions, ",") != "62.0,67.0" {
		t.Fatalf("oracle rows/versions: %d + %d %v", len(data.Cases), len(data.Controls), data.APIVersions)
	}
	rows := append(append([]familyCase{}, data.Cases...), data.Controls...)
	seen := map[string]bool{}
	for _, tc := range rows {
		if seen[tc.ID] || tc.Expected == "?" || (tc.CarryOwner != "" && !strings.HasPrefix(tc.Expected, "COMPILE_ERROR\t")) {
			t.Fatalf("invalid oracle row: %#v", tc)
		}
		if strings.HasPrefix(tc.Expected, "COMPILE_ERROR\t") && tc.NativeDiagnostic == "" {
			t.Fatalf("compile rejection lacks exact native diagnostic: %#v", tc)
		}
		seen[tc.ID] = true
		if tc.CarryOwner == "A01" || tc.CarryOwner == "A02" || tc.CarryOwner == "A04" || tc.CarryOwner == "A42" {
			t.Fatalf("row carried to Done or own family: %#v", tc)
		}
	}
	helper := `public class P {
 public String expectedText;
 public Boolean expectedNull=false;
 public void out(String id,Object v){
  String observedText=String.valueOf(v);
  if(expectedNull){
   System.assert(v==null,id+' expected raw null actual <'+observedText+'>');
   return;
  }
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
 public void err(String id,Exception e){
  String observedText='EXC|'+e.getTypeName()+'|'+e.getMessage();
  System.assert(expectedText.equals(observedText),id+' expected <'+expectedText+'> actual <'+observedText+'>');
 }
}`
	body := func(tc familyCase) string {
		prefix := "P pq=new P(); pq.expectedText=" + conformanceApexString(tc.Expected) + ";\n"
		if tc.ExpectedNull {
			prefix += "pq.expectedNull=true;\n"
		}
		if tc.Compile {
			return prefix + tc.Code
		}
		return prefix + "try {Object r; " + tc.Code + " pq.out('" + tc.ID + "',r);}catch(Exception e){pq.err('" + tc.ID + "',e);}"
	}
	for _, api := range data.APIVersions {
		t.Run(api, func(t *testing.T) {
			counts := newConformanceCounts()
			defer counts.Log(t, "Platform Cache", api, "")
			root := t.TempDir()
			for path, source := range data.Metadata {
				writeFile(t, filepath.Join(root, path), source)
			}
			declarations := make(map[string]string, len(data.Declarations)+1)
			var names []string
			for name, source := range data.Declarations {
				declarations[name] = source
				names = append(names, name)
			}
			declarations["P"] = helper
			names = append(names, "P")
			sort.Strings(names)
			var paths []string
			for _, name := range names {
				path := filepath.Join(root, name+".cls")
				writeFile(t, path, declarations[name])
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				paths = append(paths, path)
			}
			buildIndex := func(extra string) typesys.Index {
				files := append([]string{}, paths...)
				if extra != "" {
					files = append(files, extra)
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				p.ApexFiles, p.SourceAPIVersion = files, api
				return typesys.Build(p, gladeschema.Schema{})
			}
			index := buildIndex("")
			if index.HasErrors() {
				t.Fatalf("helper parser: %v", index.Diagnostics)
			}
			if a := sema.Analyze(index); a.HasErrors() {
				t.Fatalf("helper semantics: %v", a.Diagnostics)
			}
			org := orgFromIndex(index)
			// The shared project loader, rather than direct record seeding, must
			// produce the exported setup. IDs are local relationships, not native
			// observations; compare captured fields against the corresponding row.
			partitionIDs := map[storage.ID]storage.ID{}
			for _, object := range data.Fixture.Objects {
				for _, expected := range object.Records {
					var observed storage.Record
					for _, record := range org.Objects[object.Name].Records {
						if object.Name == "PlatformCachePartition" {
							if record.Fields["DeveloperName"].String != expected.Fields["DeveloperName"].String || record.Fields["NamespacePrefix"].String != expected.Fields["NamespacePrefix"].String {
								continue
							}
						} else if record.Fields["PlatformCachePartitionId"].ID != partitionIDs[expected.Fields["PlatformCachePartitionId"].ID] || record.Fields["CacheType"].String != expected.Fields["CacheType"].String {
							continue
						}
						observed = record
						break
					}
					if observed.ID == "" {
						t.Fatalf("shared metadata loader missing %s record: %#v", object.Name, expected)
					}
					if object.Name == "PlatformCachePartition" {
						partitionIDs[expected.ID] = observed.ID
					}
					for field, want := range expected.Fields {
						if field == "PlatformCachePartitionId" {
							want = storage.IDValue(partitionIDs[want.ID])
						}
						if !reflect.DeepEqual(observed.Fields[field], want) {
							t.Fatalf("shared metadata loader %s.%s: expected %#v actual %#v", object.Name, field, want, observed.Fields[field])
						}
					}
				}
			}
			for _, boundary := range data.Boundaries {
				t.Logf("Platform Cache boundary: %s", boundary)
			}
			for _, tc := range data.MetadataControls {
				counts.Run(t, "metadata", tc.ID, tc.ID, func(t *testing.T) {
					controlRoot := t.TempDir()
					writeFile(t, filepath.Join(controlRoot, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
					for path, source := range tc.Metadata {
						writeFile(t, filepath.Join(controlRoot, path), source)
					}
					p, err := project.Load(controlRoot)
					if err != nil {
						t.Fatal(err)
					}
					before := org.Clone()
					observedText := "SUCCEEDED"
					if err := resource.ApplyProject(&org, p); err != nil {
						observedText = "DEPLOY_ERROR|" + err.Error()
					}
					if observedText != tc.Expected {
						t.Fatalf("%s expected <%s> actual <%s>", tc.ID, tc.Expected, observedText)
					}
					// D001-D004 run below against this exact post-attempt setup.
					for _, name := range []string{"PlatformCachePartition", "PlatformCachePartitionType"} {
						if !reflect.DeepEqual(org.Objects[name].Records, before.Objects[name].Records) {
							t.Fatalf("%s rejected metadata changed %s", tc.ID, name)
						}
					}
				})
			}

			anonymousRunner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			var namedSource strings.Builder
			namedSource.WriteString("@IsTest(SeeAllData=true) private class PlatformCacheProbe {\n")
			accepted := 0
			for _, tc := range rows {
				if strings.HasPrefix(tc.Expected, "COMPILE_ERROR\t") {
					continue
				}
				accepted++
				fmt.Fprintf(&namedSource, "@IsTest static void observed%s(){\n%s\n}\n", tc.ID, body(tc))
			}
			namedSource.WriteString("}\n")
			namedPath := filepath.Join(root, "PlatformCacheProbe.cls")
			writeFile(t, namedPath, namedSource.String())
			writeFile(t, namedPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			namedIndex := buildIndex(namedPath)
			if namedIndex.HasErrors() {
				t.Fatalf("named parser: %v", namedIndex.Diagnostics)
			}
			if a := sema.Analyze(namedIndex); a.HasErrors() {
				t.Fatalf("named semantics: %v", a.Diagnostics)
			}
			namedCases := Discover(namedIndex, Options{})
			if len(namedCases) != accepted {
				t.Fatalf("named discovery: %d expected %d", len(namedCases), accepted)
			}
			run := Run(namedIndex, Options{NoDiskCache: true, Parallelism: 1})
			namedResults := make(map[string]testreport.Case, accepted)
			for _, suite := range run.Suites {
				for _, result := range suite.Cases {
					id := strings.TrimPrefix(result.MethodName, "observed")
					if result.ClassName != "PlatformCacheProbe" || namedResults[id].MethodName != "" {
						t.Fatalf("unexpected/repeated named result: %#v", result)
					}
					namedResults[id] = result
				}
			}
			if len(namedResults) != accepted {
				t.Fatalf("named results: %d expected %d", len(namedResults), accepted)
			}
			for _, tc := range rows {
				t.Run(tc.ID, func(t *testing.T) {
					kind := "exact"
					if tc.CarryOwner != "" {
						kind = "carried"
					}
					for _, route := range []string{"anonymous", "isTest"} {
						counts.RunKind(t, route, tc.ID, route, kind, func(t *testing.T) {
							if strings.HasPrefix(tc.Expected, "COMPILE_ERROR\t") {
								var diagnostics []diagnostic.Diagnostic
								if route == "anonymous" {
									diagnostics = sema.AnalyzeAnonymous(index, body(tc), api).Diagnostics
								} else {
									path := filepath.Join(root, "PlatformCacheRejected.cls")
									writeFile(t, path, "@IsTest private class PlatformCacheRejected {@IsTest static void observed(){\n"+body(tc)+"\n}}")
									writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
									rejectedIndex := buildIndex(path)
									diagnostics = append(rejectedIndex.Diagnostics, sema.Analyze(rejectedIndex).Diagnostics...)
								}
								var messages []string
								for _, d := range diagnostics {
									if d.Severity == diagnostic.Error {
										messages = append(messages, d.Message)
									}
								}
								observedText := strings.Join(messages, " | ")
								if tc.CarryOwner != "" {
									t.Logf("%s carried to %s: %s Native <%s> Glade <%s>", tc.ID, tc.CarryOwner, tc.CarryReason, tc.NativeDiagnostic, observedText)
								} else if observedText != tc.NativeDiagnostic {
									t.Fatalf("%s expected <%s> actual <%s>", tc.ID, tc.NativeDiagnostic, observedText)
								}
								return
							}
							if route == "isTest" {
								result := namedResults[tc.ID]
								if result.Status != testreport.StatusPass {
									t.Fatalf("named row: status %s problem %v", result.Status, result.Problem)
								}
								return
							}
							source := body(tc)
							if a := sema.AnalyzeAnonymous(index, source, api); a.HasErrors() {
								t.Fatalf("anonymous semantics: %v", a.Diagnostics)
							}
							program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
							if err != nil {
								t.Fatal(err)
							}
							if _, err := anonymousRunner.execute(program); err != nil {
								t.Fatal(err)
							}
						})
					}
				})
			}
		})
	}
}
