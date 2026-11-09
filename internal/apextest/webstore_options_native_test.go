package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

const webStoreOptionsDir = "testdata/conformance/standard_webstore_options/"

func readWebStoreOptionRows(t *testing.T) []map[string]string {
	t.Helper()
	file, err := os.Open(webStoreOptionsDir + "org.tsv")
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

func webStoreOptionsProject(t *testing.T, api string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	definition, err := os.ReadFile(webStoreOptionsDir + "definition.json")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(definition))
	return root
}

func writeWebStoreOptionsClass(t *testing.T, root, api, name, source string) {
	t.Helper()
	path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
	writeFile(t, path, source)
	writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
}

func TestWebStoreOptionsCompileNativeConformance(t *testing.T) {
	var probes []struct{ ID, Source, Name string }
	raw, err := os.ReadFile(webStoreOptionsDir + "setup/probe-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &probes); err != nil {
		t.Fatal(err)
	}
	rows := readWebStoreOptionRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := webStoreOptionsProject(t, api)
			for _, probe := range probes {
				writeWebStoreOptionsClass(t, root, api, probe.Name, probe.Source)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			rejected := map[string]bool{}
			for _, files := range [][]string{diagnosticFiles(index.Diagnostics), diagnosticFiles(analysis.Diagnostics)} {
				for _, file := range files {
					rejected[strings.TrimSuffix(filepath.Base(file), ".cls")] = true
				}
			}
			seen := 0
			for _, row := range rows {
				if row["api"] != api || row["route"] != "deploy-compile" {
					continue
				}
				seen++
				for _, probe := range probes {
					if probe.ID == row["case"] {
						if got, want := rejected[probe.Name], row["outcome"] == "rejected"; got != want {
							t.Errorf("%s: native %s, Glade rejected=%t; diagnostics: %v %v", row["case"], row["outcome"], got, index.Diagnostics, analysis.Diagnostics)
						}
					}
				}
			}
			if seen != len(probes) {
				t.Fatalf("native rows=%d, want %d", seen, len(probes))
			}
		})
	}
}

// Observations of the fourteen Boolean Options* fields: describe facets,
// dynamic put, insert and select. Each @IsTest method reports its answers in
// an assertion message.
func TestWebStoreOptionsRuntimeNativeConformance(t *testing.T) {
	source, err := os.ReadFile(webStoreOptionsDir + "setup/RtWs.cls")
	if err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(webStoreOptionsDir + "setup/RtWs2.cls")
	if err != nil {
		t.Fatal(err)
	}
	entitlement, err := os.ReadFile(webStoreOptionsDir + "setup/RtCe.cls")
	if err != nil {
		t.Fatal(err)
	}
	rows := readWebStoreOptionRows(t)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := webStoreOptionsProject(t, api)
			writeWebStoreOptionsClass(t, root, api, "RtCe", string(entitlement))
			writeWebStoreOptionsClass(t, root, api, "RtWs", string(source))
			writeWebStoreOptionsClass(t, root, api, "RtWs2", string(second))
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			for _, method := range []string{"members", "names_only", "dyn_put_all_false", "dyn_put_mixed", "dyn_put_missing_one", "put_unknown", "preserve", "ce_members", "ce_chain", "ce_policy_name_only"} {
				class := "RtWs"
				if method == "preserve" {
					class = "RtWs2"
				} else if strings.HasPrefix(method, "ce_") {
					class = "RtCe"
				}
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
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
					if row["api"] != api || row["route"] != "test-runner" || !strings.HasPrefix(row["case"], method+":") {
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
