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
)

// Native rows (scratch org, API 62.0 and 67.0, anonymous Apex over before-save
// flows): Custom Error merge fields, a constant inside a decision formula, a
// before-save Update Records read of <Relationship>.Id, and a decision formula
// that reads RecordType.DeveloperName from the record and from a Get Records
// output. Glade replays the same probe class through an @IsTest wrapper.
func TestFlowGapsNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_gaps"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	recordTypes := []string{"GapParA", "GapParB", "GapParC", "GapChildX", "GapChildY"}
	flows := []string{"Gap_ConstDecision", "Gap_ErrMerge", "Gap_RelUpdate", "Gap_RtDecision"}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := "api" + strings.TrimSuffix(api, ".0")
			run := read("native/" + version + "/anonymous/RUN-FACTS.txt")
			for name, value := range map[string]string{"api": api, "context": "anonymous", "rows": "13", "native_complete": "true", "deployment_success": "true", "flow_api": api} {
				if !strings.Contains("\n"+run, "\n"+name+"="+value+"\n") {
					t.Fatalf("missing native capture fact: %s", name)
				}
			}
			root := t.TempDir()
			files := map[string]string{
				"classes/GapProbe.cls":              read("sources/classes/GapProbe.cls"),
				"classes/GapProbeTest.cls":          read("sources/classes/GapProbeTest.cls"),
				"classes/GapProbe.cls-meta.xml":     `<ApexClass><apiVersion>` + api + `</apiVersion></ApexClass>`,
				"classes/GapProbeTest.cls-meta.xml": `<ApexClass><apiVersion>` + api + `</apiVersion></ApexClass>`,
			}
			for _, name := range flows {
				files["flows/"+name+".flow-meta.xml"] = read("metadata/" + version + "/flows/" + name + ".flow-meta.xml")
			}
			files["profiles/Admin.profile-meta.xml"] = read("sources/profiles/Admin.profile-meta.xml")
			for _, name := range recordTypes {
				files["objects/Account/recordTypes/"+name+".recordType-meta.xml"] = read("sources/objects/Account/recordTypes/" + name + ".recordType-meta.xml")
			}
			for path, source := range files {
				if strings.HasPrefix(path, "classes/GapProbeTest") || strings.HasSuffix(path, "-meta.xml") && strings.HasPrefix(path, "classes/") {
					continue
				}
				if !strings.Contains(run, fmt.Sprintf("source_sha256:%s=%x\n", path, sha256.Sum256([]byte(source)))) {
					t.Fatalf("capture detached from exact source: %s", path)
				}
			}
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			for path, source := range files {
				writeFile(t, filepath.Join(root, "force-app", "main", "default", path), source)
			}
			want := make(map[string]map[string]any)
			rows := strings.Split(strings.TrimSuffix(read("native/"+version+"/anonymous/org.tsv"), "\n"), "\n")
			if len(rows) != 14 || rows[0] != "id\tgroup\troute\tstatus\tvalue" {
				t.Fatal("incomplete native TSV")
			}
			for _, line := range rows[1:] {
				row := strings.SplitN(line, "\t", 5)
				var cell struct {
					Outcome string
					Value   map[string]any
				}
				if len(row) != 5 || json.Unmarshal([]byte(row[4]), &cell) != nil || cell.Outcome != "runtime" {
					t.Fatalf("invalid native row: %s", line)
				}
				want[row[0]] = cell.Value
			}
			result := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"GapProbeTest"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
			if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("observation selection: %s", firstRunProblem(result))
			}
			_, payload, found := strings.Cut(result.Suites[0].Cases[0].Problem.Message, "PROBE|")
			if !found {
				t.Fatalf("missing observation envelope: %s", result.Suites[0].Cases[0].Problem.Message)
			}
			var got map[string]map[string]any
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatalf("invalid observation: %s (%v)", payload, err)
			}
			for id, expected := range want {
				if !reflect.DeepEqual(got[id], expected) {
					t.Errorf("%s: native %#v; Glade %#v", id, expected, got[id])
				}
			}
		})
	}
}
