package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Replays the exact captured Number/Currency decisions through anonymous Apex
// and the project test runner. Repeating Run covers its warm runtime cache.
func TestFlowNullComparisonsNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_null_comparisons"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := "api" + strings.TrimSuffix(api, ".0")
			root := t.TempDir()
			files := map[string]string{}
			for _, name := range []string{"NullCompareProbe", "NullCompareProbeTest"} {
				files["classes/"+name+".cls"] = read("sources/classes/" + name + ".cls")
				files["classes/"+name+".cls-meta.xml"] = "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>\n"
			}
			for _, name := range []string{"NullCompareAuto", "NullCompareTrigger"} {
				files["flows/"+name+".flow-meta.xml"] = read("metadata/" + version + "/flows/" + name + ".flow-meta.xml")
			}
			wants := map[string]map[string]string{}
			for _, context := range []string{"anonymous", "istest"} {
				capture := read("native/" + version + "/" + context + "/RUN-FACTS.txt")
				for _, fact := range []string{"api=" + api, "flow_api=" + api, "context=" + context, "rows=120", "native_complete=true", "deployment_success=true"} {
					if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
						t.Fatalf("missing capture fact %s", fact)
					}
				}
				for path, body := range files {
					if strings.HasSuffix(path, ".cls-meta.xml") {
						continue
					}
					if !strings.Contains(capture, fmt.Sprintf("source_sha256:%s=%x\n", path, sha256.Sum256([]byte(body)))) {
						t.Fatalf("source differs from capture: %s", path)
					}
				}
				anonymous := read("sources/scripts/anonymous.apex")
				if !strings.Contains(capture, fmt.Sprintf("anonymous_source_sha256=%x\n", sha256.Sum256([]byte(anonymous)))) {
					t.Fatal("anonymous source differs from capture")
				}
				rows := strings.Split(strings.TrimSpace(read("native/"+version+"/"+context+"/org.tsv")), "\n")
				if len(rows) != 121 || rows[0] != "id\troute\toutcome" {
					t.Fatal("incomplete native matrix")
				}
				want := map[string]string{}
				for _, line := range rows[1:] {
					row := strings.Split(line, "\t")
					if len(row) != 3 || (row[2] != "match" && row[2] != "other") {
						t.Fatalf("invalid native row: %s", line)
					}
					key := row[1] + "/" + row[0]
					if _, exists := want[key]; exists {
						t.Fatalf("duplicate native row: %s", key)
					}
					want[key] = row[2]
				}
				wants[context] = want
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"sourceApiVersion\":\""+api+"\"}")
			for path, body := range files {
				writeFile(t, filepath.Join(root, "force-app/main/default", path), body)
			}
			index := loadTestIndex(t, root)
			check := func(payload, context string) {
				t.Helper()
				var got map[string]string
				if err := json.Unmarshal([]byte(payload), &got); err != nil {
					t.Fatalf("invalid observation: %v (%s)", err, payload)
				}
				if !reflect.DeepEqual(got, wants[context]) {
					for key, want := range wants[context] {
						if got[key] != want {
							t.Errorf("%s: native %s; Glade %s", key, want, got[key])
						}
					}
					if len(got) != len(wants[context]) {
						t.Errorf("observations=%d; want %d", len(got), len(wants[context]))
					}
				}
			}
			t.Run("anonymous", func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions(read("sources/scripts/anonymous.apex"), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.newMachine().Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, line := range result.Debug {
					if _, payload, ok := strings.Cut(line, "PROBE|"); ok {
						check(payload, "anonymous")
						return
					}
				}
				t.Fatal("missing observation")
			})
			for _, temperature := range []string{"cold", "warm"} {
				t.Run("project/"+temperature, func(t *testing.T) {
					result := Run(index, Options{SelectedClasses: []string{"NullCompareProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
					if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem == nil {
						t.Fatalf("observation selection: %s", firstRunProblem(result))
					}
					_, payload, ok := strings.Cut(result.Suites[0].Cases[0].Problem.Message, "PROBE|")
					if !ok {
						t.Fatalf("runtime failed before observation: %s", firstRunProblem(result))
					}
					check(payload, "istest")
				})
			}
		})
	}
}
