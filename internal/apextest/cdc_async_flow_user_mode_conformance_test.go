package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestCDCAsyncFlowUserModeNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "native-capture", "cdc_async_flow_user_mode")
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
		f := strings.SplitN(line, "\t", 3)
		if len(f) != 3 || f[1] != "committed_cdc_async" {
			t.Fatal("invalid row")
		}
		var v map[string]any
		if err := json.Unmarshal([]byte(f[2]), &v); err != nil {
			t.Fatal(err)
		}
		rows[f[0]] = v
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.Walk(filepath.Join(dir, "project"), func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(filepath.Join(dir, "project"), path)
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				s := string(data)
				if api == "62.0" {
					s = strings.ReplaceAll(s, "67.0", "62.0")
					s = strings.ReplaceAll(s, "asyncap67", "asyncap")
				}
				writeFile(t, filepath.Join(root, rel), s)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "glade.yml"), "test:\n  permissionSets: [Observe]\n")
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			org.APIVersion = api
			if err := applyConfiguredTestPermissions(&org, loadProjectForTest(t, root)); err != nil {
				t.Fatal(err)
			}
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			// CDC delivery after commit is hosted. Inject its observed user at
			// the record-save boundary, then drain the local async Flow. CDC
			// trigger delivery itself is covered by TestBatchEventAsyncNativeConformance.
			foundUser := false
			for _, user := range org.Objects["User"].Records {
				if user.Fields["UserType"].String == "AutomatedProcess" {
					machine.SetCurrentUser(user)
					foundUser = true
					break
				}
			}
			if !foundUser {
				t.Fatal("missing seeded Automated Process user")
			}
			p, err := vm.CompileAnonymousWithOptions("APProbe.enqueueFlow();", vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(p)
			if err != nil {
				t.Fatal(err)
			}
			if err = machine.DrainAsync(&result); err != nil {
				t.Fatal(err)
			}
			name := "asyncap67"
			if api == "62.0" {
				name = "asyncap"
			}
			found := false
			for _, rec := range machine.Org.Objects["Obs__c"].Records {
				if rec.Fields["Name"].String != name {
					continue
				}
				found = true
				var got map[string]any
				if err := json.Unmarshal([]byte(rec.Fields["Data__c"].String), &got); err != nil {
					t.Fatal(err)
				}
				if d := observationDiffs("", got, rows[api]); len(d) > 0 {
					t.Errorf("native differences: %v", d)
				}
			}
			if !found {
				t.Error("missing committed CDC async Flow observation")
			}
		})
	}
}
