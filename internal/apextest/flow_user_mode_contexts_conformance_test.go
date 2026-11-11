package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowUserModeContextsNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "native-capture", "flow_user_mode_contexts")
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		f := strings.SplitN(line, "\t", 3)
		if len(f) != 3 {
			t.Fatal("malformed native row")
		}
		if rows[f[0]] == nil {
			rows[f[0]] = map[string]map[string]any{}
		}
		var v map[string]any
		if json.Unmarshal([]byte(f[2]), &v) != nil {
			t.Fatal("malformed native observation")
		}
		rows[f[0]][f[1]] = v
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			if len(rows[api]) != 3 {
				t.Fatal("incomplete native rows")
			}
			root := t.TempDir()
			err := filepath.Walk(filepath.Join(dir, "project"), func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(filepath.Join(dir, "project"), path)
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				s := string(content)
				if api == "62.0" {
					s = strings.ReplaceAll(s, "67.0", "62.0")
					s = strings.ReplaceAll(s, "async67", "async")
					s = strings.ReplaceAll(s, "scheduled67", "scheduled62")
				}
				writeFile(t, filepath.Join(root, rel), s)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [Observe]\n")
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"FlowAccessTest"}, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("missing test observation: %+v", run.Summary())
			}
			msg := run.Suites[0].Cases[0].Problem.Message
			_, payload, ok := strings.Cut(msg, "OBS|")
			var got map[string]any
			if !ok || json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatalf("invalid test observation: %s", msg)
			}
			if d := observationDiffs("", got, rows[api]["istest"]); len(d) > 0 {
				t.Fatal(d)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			if err := applyConfiguredTestPermissions(&org, loadProjectForTest(t, root)); err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			program, err := vm.CompileAnonymousWithOptions("APProbe.enqueueFlow();", vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			if err = machine.DrainAsync(&result); err != nil {
				t.Fatal(err)
			}
			name := "async67"
			if api == "62.0" {
				name = "async"
			}
			found := false
			for _, rec := range machine.Org.Objects["Obs__c"].Records {
				if rec.Fields["Name"].String != name {
					continue
				}
				found = true
				var actual map[string]any
				if err := json.Unmarshal([]byte(rec.Fields["Data__c"].String), &actual); err != nil {
					t.Fatal(err)
				}
				if d := observationDiffs("", actual, rows[api]["async"]); len(d) > 0 {
					t.Errorf("async differences: %v", d)
				}
			}
			if !found {
				t.Error("missing committed async observation")
			}
			// Scheduled delivery is hosted; retain and validate its native shape.
			if d := observationDiffs("", rows[api]["scheduled"], rows[api]["async"]); len(d) > 0 {
				t.Errorf("scheduled native differences: %v", d)
			}
		})
	}
}
