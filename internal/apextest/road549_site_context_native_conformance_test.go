package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestSiteContextRoutesNativeConformance(t *testing.T) {
	f, err := os.Open("testdata/conformance/road549_site_context/org.tsv")
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
	if len(rows) != 25 {
		t.Fatalf("native rows = %d, want 24", len(rows)-1)
	}
	observations := map[string]string{}
	for _, row := range rows[1:] {
		key := fmt.Sprintf("%s/%s/%s/%s", row[0], row[1], row[2], row[3])
		if _, exists := observations[key]; exists {
			t.Fatalf("duplicate observation %s", key)
		}
		observations[key] = row[4]
	}
	for _, state := range []string{"absent", "present"} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(state+"/"+api, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Sites]\n")
				count := 0
				if state == "present" {
					count = 1
				}
				body := fmt.Sprintf("System.assertEquals(%d, [SELECT COUNT() FROM Site]);\n", count)
				if state == "present" {
					body += "Site captured = [SELECT MasterLabel, SiteType, IsActive, UrlPathPrefix FROM Site LIMIT 1];" +
						"System.assertEquals('Synthetic Site', captured.MasterLabel);" +
						"System.assertEquals('Visualforce', captured.SiteType);" +
						"System.assertEquals(false, captured.IsActive);" +
						"System.assertEquals('synthetic', captured.UrlPathPrefix);\n"
				}
				for _, method := range []string{"getPathPrefix", "getBaseUrl", "getSiteId"} {
					var expected string
					for _, route := range []string{"anonymous", "istest"} {
						key := fmt.Sprintf("%s/%s/%s/%s", state, api, route, method)
						value, ok := observations[key]
						if !ok {
							t.Fatalf("missing native observation %s", key)
						}
						if expected != "" && expected != value {
							t.Fatalf("route mismatch for %s", key)
						}
						expected = value
					}
					switch expected {
					case "VALUE|":
						body += fmt.Sprintf("System.assert(Site.%s() != null); System.assertEquals('', Site.%s());\n", method, method)
					case "NULL":
						body += fmt.Sprintf("System.assertEquals(null, Site.%s());\n", method)
					default:
						t.Fatalf("unsupported native observation %q", expected)
					}
				}
				runSiteContextRoutes(t, root, api, state, body)
			})
		}
	}
}

// Exercise the captured anonymous and @IsTest routes against independent
// clones of the same Sites-enabled org. The project runner otherwise rebuilds
// the seeded Site, so the no-record shape must be supplied explicitly.
func runSiteContextRoutes(t *testing.T, root, api, state, body string) {
	t.Helper()
	const class = "SiteContextEvidenceTest"
	path := filepath.Join(root, "force-app/main/default/classes", class+".cls")
	writeFile(t, path, "@IsTest private class "+class+" { @IsTest static void sample(){System.assertEquals(true, Test.isRunningTest());"+body+"} }")
	writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
	index := loadTestIndex(t, root)
	anonymous := "System.assertEquals(false, Test.isRunningTest());" + body
	if analysis := sema.AnalyzeAnonymous(index, anonymous, api); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("anonymous compilation: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	if analysis := sema.Analyze(index); analysis.HasErrors() {
		t.Fatalf("test compilation: %v", analysis.Diagnostics)
	}
	org := orgFromIndex(index)
	if !org.HasFeature("Sites") {
		t.Fatal("Sites feature is not enabled")
	}
	site := org.Objects["Site"]
	if len(site.Records) != 1 {
		t.Fatalf("seed Site records = %d, want 1", len(site.Records))
	}
	records := map[storage.ID]storage.Record{}
	if state == "present" {
		// The captured CustomSite metadata is inactive and Visualforce, with
		// the label and prefix below (see RUN-FACTS.txt). Do not use the
		// unrelated active ChatterNetwork defaults of the local seed.
		for id := range site.Records {
			records[id] = storage.Record{ID: id, Object: "Site", Fields: map[string]storage.Value{
				"Name":          storage.StringValue("Synthetic"),
				"MasterLabel":   storage.StringValue("Synthetic Site"),
				"SiteType":      storage.StringValue("Visualforce"),
				"IsActive":      storage.BooleanValue(false),
				"UrlPathPrefix": storage.StringValue("synthetic"),
			}}
		}
	}
	site.SetRecords(records)
	site.Indexes = nil
	site.RecordsShared = false
	site.IndexesShared = false
	org.Objects["Site"] = site
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	program, err := vm.CompileAnonymousWithOptions(anonymous, vm.CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	cases := Discover(index, Options{SelectedClasses: []string{class}, SelectedMethod: "sample"})
	if len(cases) != 1 {
		t.Fatalf("discovered %d tests, want 1", len(cases))
	}
	methods, methodErrors := compileTestMethods(cases)
	programs, programErrors := compileTestInvokePrograms(cases)
	tc := cases[0]
	key := testCaseKey(tc)
	if methodErrors[key] != nil || programErrors[key] != nil {
		t.Fatalf("test compile: %v %v", methodErrors[key], programErrors[key])
	}
	for _, temperature := range []string{"cold", "warm"} {
		t.Run(temperature, func(t *testing.T) {
			if _, err := runner.execute(program); err != nil {
				t.Fatalf("anonymous: %v", err)
			}
			machine := runner.newMachine()
			if err := machine.RegisterMethod(methods[key]); err != nil {
				t.Fatal(err)
			}
			machine.EnableTestContext()
			machine.SetTestSeeAllData(tc.SeeAllData)
			if _, err := machine.ExecuteInClass(programs[key], tc.ClassName); err != nil {
				t.Fatalf("@IsTest: %v", err)
			}
		})
	}
}
