package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

const enhancedOrdersDir = "testdata/conformance/standard_enhanced_commerce_orders/"

func readEnhancedOrdersRows(t *testing.T, name string) []map[string]string {
	t.Helper()
	file, err := os.Open(enhancedOrdersDir + name)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	table, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]string
	for _, record := range table[1:] {
		row := map[string]string{}
		for i, column := range table[0] {
			row[column] = record[i]
		}
		rows = append(rows, row)
	}
	return rows
}

func enhancedOrdersProject(t *testing.T, api, variant string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	definition, err := os.ReadFile(enhancedOrdersDir + "definitions/" + variant + ".json")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(definition))
	return root
}

func writeEnhancedOrdersClass(t *testing.T, root, api, name, source string) {
	t.Helper()
	path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
	writeFile(t, path, source)
	writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
}

// Native compile rows: only the enhanced commerce orders setting makes the
// fields visible. The OrderSaveBehaviorBoth feature alone does not.
func TestEnhancedCommerceOrdersCompileNativeConformance(t *testing.T) {
	var probes []struct{ ID, Source, Name string }
	raw, err := os.ReadFile(enhancedOrdersDir + "setup/probe-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &probes); err != nil {
		t.Fatal(err)
	}
	rows := readEnhancedOrdersRows(t, "org.tsv")
	for _, api := range []string{"62.0", "67.0"} {
		for _, variant := range []string{"feature-only", "setting-only", "feature-and-setting", "capitalized-settings"} {
			t.Run(api+"/"+variant, func(t *testing.T) {
				root := enhancedOrdersProject(t, api, variant)
				for _, probe := range probes {
					writeEnhancedOrdersClass(t, root, api, probe.Name, probe.Source)
				}
				index := loadTestIndex(t, root)
				analysis := sema.Analyze(index)
				rejected := map[string]bool{}
				for _, diagnostics := range [][]string{diagnosticFiles(index.Diagnostics), diagnosticFiles(analysis.Diagnostics)} {
					for _, file := range diagnostics {
						rejected[strings.TrimSuffix(filepath.Base(file), ".cls")] = true
					}
				}
				seen := 0
				for _, row := range rows {
					if row["api"] != api || row["variant"] != variant || row["route"] != "deploy-compile" {
						continue
					}
					seen++
					var name string
					for _, probe := range probes {
						if probe.ID == row["case"] {
							name = probe.Name
						}
					}
					want := row["outcome"] == "rejected"
					if got := rejected[name]; got != want {
						t.Errorf("%s: native %s, Glade rejected=%t; diagnostics: %v %v", row["case"], row["outcome"], got, index.Diagnostics, analysis.Diagnostics)

					}
				}
				if seen != len(probes) {
					t.Fatalf("native rows=%d, want %d", seen, len(probes))
				}
			})
		}
	}
}

func diagnosticFiles(diagnostics []diagnostic.Diagnostic) []string {
	var files []string
	for _, item := range diagnostics {
		if item.Severity == diagnostic.Error && item.File != "" {
			files = append(files, item.File)
		}
	}
	return files
}

// The granted route: insert and select observations after the running user was
// granted read access to the gated fields. The setting-only and
// feature-and-setting orgs agree at both APIs.
func TestEnhancedCommerceOrdersRuntimeNativeConformance(t *testing.T) {
	source, err := os.ReadFile(enhancedOrdersDir + "setup/RtOrd.cls")
	if err != nil {
		t.Fatal(err)
	}
	rows := readEnhancedOrdersRows(t, "org.tsv")
	for _, api := range []string{"62.0", "67.0"} {
		for _, variant := range []string{"setting-only", "feature-and-setting"} {
			t.Run(api+"/"+variant, func(t *testing.T) {
				root := enhancedOrdersProject(t, api, variant)
				writeEnhancedOrdersClass(t, root, api, "RtOrd", string(source))
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				for _, method := range []string{"dml", "members"} {
					run := Run(index, Options{SelectedClasses: []string{"RtOrd"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("%s: expected one assertion transport: %+v", method, run.Summary())
					}
					_, message, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "Assertion Failed: ")
					got := map[string]string{}
					for _, item := range strings.Split(message, " ;; ") {
						key, value, _ := strings.Cut(item, "=")
						got[method+":"+strings.TrimSpace(key)] = strings.TrimSpace(value)
					}
					seen := 0
					for _, row := range rows {
						if row["api"] != api || row["variant"] != variant || row["route"] != "test-runner-granted" || !strings.HasPrefix(row["case"], method+":") {
							continue
						}
						seen++
						if got[row["case"]] != row["observation"] {
							t.Errorf("%s: native=%q Glade=%q", row["case"], row["observation"], got[row["case"]])
						}
					}
					if seen == 0 {
						t.Fatalf("no native rows for %s", method)
					}
				}
			})
		}
	}
}
