package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestFlowLoopRelationshipCommittedNative(t *testing.T) {
	const fixture = "testdata/conformance/flow_loop_relationship"
	const capture = "testdata/native-capture/flow_loop_relationship"
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := "api" + strings.TrimSuffix(api, ".0")
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			for _, name := range []string{"RouteProbe", "RouteProbeTest"} {
				p := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, p, read("sources/classes/"+name+".cls"))
				writeFile(t, p+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/flows/RouteLoop.flow-meta.xml"), read("metadata/"+version+"/flows/RouteLoop.flow-meta.xml"))
			row := strings.SplitN(strings.TrimSpace(readCapture(t, filepath.Join(capture, "native", version, "committed_anonymous", "org.tsv"))), "\n", 2)
			if len(row) != 2 || row[0] != "id\tvalue" || !strings.HasPrefix(row[1], "loop\t") {
				t.Fatal("invalid native row")
			}
			var want map[string]any
			if err := json.Unmarshal([]byte(strings.TrimPrefix(row[1], "loop\t")), &want); err != nil {
				t.Fatal(err)
			}
			idx := loadTestIndex(t, root)
			org := orgFromIndex(idx)
			org.APIVersion = api
			runner := newConformanceRunner(t, idx, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			program, err := vm.CompileAnonymousWithOptions(`System.debug('V036OBS|' + JSON.serialize(RouteProbe.observe()));`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, line := range result.Debug {
				if _, payload, ok := strings.Cut(line, "V036OBS|"); ok {
					var got map[string]any
					if err := json.Unmarshal([]byte(payload), &got); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("native %v; Glade %v", want, got)
					}
					found = true
					break
				}
			}
			if !found {
				t.Fatal("missing committed observation")
			}
			controls := strings.TrimSpace(readCapture(t, filepath.Join(capture, "native", version, "committed_anonymous", "followup.tsv")))
			if controls != "id\tvalue\nrollback\tREQUIRED_FIELD_MISSING\nreadback\t4" {
				t.Fatal("invalid native rollback controls")
			}
			// Anonymous hosts run each request on a clone and publish it only on
			// success (server/composite_handlers.go). VM.Execute itself is a
			// lower-level primitive and does not represent that host boundary.
			committed := machine.Org
			next := committed.Clone()
			machine.SetOrg(&next)
			rollback, err := vm.CompileAnonymousWithOptions(`insert new Account(Name='RouteLoopTrigger'); insert new Account();`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := machine.Execute(rollback); err == nil || !strings.Contains(err.Error(), "REQUIRED_FIELD_MISSING") {
				t.Fatalf("native rollback REQUIRED_FIELD_MISSING; Glade=%v", err)
			}
			machine.SetOrg(committed)
			readback, err := vm.CompileAnonymousWithOptions(`System.debug('V036COUNT|' + [SELECT COUNT() FROM Account WHERE Name = 'RouteLoopTrigger']);`, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, err = machine.Execute(readback)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range result.Debug {
				if _, payload, ok := strings.Cut(line, "V036COUNT|"); ok {
					if payload != "4" {
						t.Errorf("native committed count 4; Glade=%s", payload)
					}
					return
				}
			}
			t.Fatal("missing committed readback")
		})
	}
}

func readCapture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
