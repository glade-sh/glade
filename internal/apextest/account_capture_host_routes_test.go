package apextest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// The host adapters replay the anonymous capture; the native rows do not
// claim that Salesforce's browser transports were captured.
func TestAccountCaptureHostRoutes(t *testing.T) {
	for _, tc := range []struct{ name, fixture, expression string }{
		{"delete", accountDeleteFixture, "LpDeleteProbe.run()"},
		{"recordType", accountObjectSettingsFixture, "LpRtProbe.run()"},
	} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(tc.name+"/"+api, func(t *testing.T) {
				root := accountCaptureProject(t, tc.fixture, api)
				if tc.name == "delete" {
					accountCaptureExactTailMasks(t, filepath.Join(root, "force-app/main/default/classes/LpDeleteProbe.cls"))
				}
				index := loadTestIndex(t, root)
				org := capturedFixtureOrg(t, index, api)
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
				program, err := vm.CompileAnonymousWithOptions("return JSON.serialize("+tc.expression+");", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				want := map[string]string{}
				for _, row := range accountCaptureNativeRows(t, tc.fixture)[api]["anonymous"] {
					if !strings.HasSuffix(row[0], " compile") && !strings.HasPrefix(row[0], "S") {
						want[row[0]] = row[1]
					}
				}
				for _, route := range []string{"lwc", "vf"} {
					t.Run(route, func(t *testing.T) {
						machine := runner.newMachine()
						class := vm.Class{Name: "CaptureHost", Methods: map[string]vm.Method{
							"lwc": {Name: "CaptureHost.lwc", ClassName: "CaptureHost", APIVersion: api, ReturnType: "String", IsStatic: true, Modifiers: []string{"public", "AuraEnabled"}, Program: program},
							"vf":  {Name: "CaptureHost.vf", ClassName: "CaptureHost", APIVersion: api, ReturnType: "String", Modifiers: []string{"public"}, Program: program},
						}}
						if err := machine.RegisterClass(class); err != nil {
							t.Fatal(err)
						}
						for _, state := range []string{"cold", "warm"} {
							var out vm.UIInvocationResult
							var err error
							if route == "lwc" {
								out, err = machine.InvokeLWCMethod("CaptureHost", "lwc", nil)
							} else {
								out, err = machine.InvokeVisualforceAction("CaptureHost", "vf", "/apex/CaptureHost", nil)
							}
							if err != nil || !out.Success {
								t.Fatalf("%s: host error %v: %+v", state, err, out.Error)
							}
							var rows []string
							if err := json.Unmarshal([]byte(fmt.Sprint(out.ReturnValue)), &rows); err != nil {
								t.Fatalf("%s: decode %v: %#v", state, err, out.ReturnValue)
							}
							got := map[string]string{}
							for _, row := range rows {
								id, observation, ok := strings.Cut(row, "|")
								if ok {
									got[id] = normalizeAccountCapture(observation)
								}
							}
							for id, expected := range want {
								if got[id] != expected {
									t.Errorf("%s %s: got %q, want %q", state, id, got[id], expected)
								}
							}
						}
					})
				}
			})
		}
	}
}

// The P0 REST describe fixtures report EmailMessage.Status as a restricted
// picklist at both API versions; this checks the local UI adapter boundary.
func TestRestrictedPicklistHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			idx := loadTestIndex(t, root)
			org := capturedFixtureOrg(t, idx, api)
			machine := vm.New(nil)
			machine.SetOrg(&org)
			source := `Schema.DescribeFieldResult d = EmailMessage.Status.getDescribe();
System.assertEquals(Schema.DisplayType.PICKLIST, d.getType());
System.assert(d.isRestrictedPicklist());
System.assertEquals(6, d.getPicklistValues().size());
System.assertEquals('Sent', d.getPicklistValues()[3].getLabel());
return 'ok';`
			program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			class := vm.Class{Name: "PicklistHost", Methods: map[string]vm.Method{
				"lwc": {Name: "PicklistHost.lwc", ClassName: "PicklistHost", APIVersion: api, ReturnType: "String", IsStatic: true, Modifiers: []string{"public", "AuraEnabled"}, Program: program},
				"vf":  {Name: "PicklistHost.vf", ClassName: "PicklistHost", APIVersion: api, ReturnType: "String", Modifiers: []string{"public"}, Program: program},
			}}
			if err := machine.RegisterClass(class); err != nil {
				t.Fatal(err)
			}
			for _, route := range []string{"lwc", "vf"} {
				for _, state := range []string{"cold", "warm"} {
					var out vm.UIInvocationResult
					var err error
					if route == "lwc" {
						out, err = machine.InvokeLWCMethod("PicklistHost", "lwc", nil)
					} else {
						out, err = machine.InvokeVisualforceAction("PicklistHost", "vf", "/apex/PicklistHost", nil)
					}
					if err != nil || !out.Success || out.ReturnValue != "ok" {
						t.Errorf("%s/%s: error=%v result=%+v", route, state, err, out)
					}
				}
			}
		})
	}
}

func TestAccountCaptureDiskRuntimeRoutes(t *testing.T) {
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, tc := range []struct{ name, fixture, expression string }{
		{"delete", accountDeleteFixture, "LpDeleteProbe.run()"},
		{"recordType", accountObjectSettingsFixture, "LpRtProbe.run()"},
	} {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(tc.name+"/"+api, func(t *testing.T) {
				root := accountCaptureProject(t, tc.fixture, api)
				if tc.name == "delete" {
					accountCaptureExactTailMasks(t, filepath.Join(root, "force-app/main/default/classes/LpDeleteProbe.cls"))
				}
				index := loadTestIndex(t, root)
				program, err := vm.CompileAnonymousWithOptions("for (String row : "+tc.expression+") System.debug('ROW|' + row);", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				want := map[string]string{}
				for _, row := range accountCaptureNativeRows(t, tc.fixture)[api]["anonymous"] {
					if !strings.HasSuffix(row[0], " compile") && !strings.HasPrefix(row[0], "S") {
						want[row[0]] = row[1]
					}
				}
				for _, route := range []string{"cold", "disk_restore"} {
					InvalidateRuntimeCaches()
					counters := newRunPerfCounters(true)
					_, built, err := runtimeFromIndexWithSourceDigestsAndPerf(index, nil, newSourceCache(), true, counters)
					if err != nil {
						t.Fatal(err)
					}
					if hits := counters.phases.diskCacheHits.Load(); (hits == 1) != (route == "disk_restore") {
						t.Fatalf("%s: disk hits=%d", route, hits)
					}
					org := built.restored.CloneOrg()
					org.APIVersion = api
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
					picklist, err := vm.CompileAnonymousWithOptions(`Schema.DescribeFieldResult d = EmailMessage.Status.getDescribe();
System.assertEquals(Schema.DisplayType.PICKLIST, d.getType());
System.assert(d.isRestrictedPicklist());
System.assertEquals(6, d.getPicklistValues().size());
System.assertEquals('Sent', d.getPicklistValues()[3].getLabel());`, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := runner.execute(picklist); err != nil {
						t.Fatalf("%s picklist: %v", route, err)
					}
					result, err := runner.execute(program)
					if err != nil {
						t.Fatalf("%s: %v", route, err)
					}
					got := accountCaptureDebugRows(result.Debug)
					for id, expected := range want {
						if got[id] != expected {
							t.Errorf("%s %s: got %q, want %q", route, id, got[id], expected)
						}
					}
				}
			})
		}
	}
}
