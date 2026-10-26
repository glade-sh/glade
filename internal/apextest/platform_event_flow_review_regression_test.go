package apextest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestPlatformEventFlowPublishTimingNativeProjectAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			nativeCases := readPlatformTransactionCases(t)
			anonymous := readPlatformTransactionNative(t, "event_flows", api, "anonymous", nativeCases)
			named := readPlatformTransactionNative(t, "event_flows", api, "istest", nativeCases)
			index := loadTestIndex(t, copyPlatformTransactionProject(t, "event_flows", api))
			var own []TestCase
			for _, row := range Discover(index, Options{SelectedClasses: []string{"CapturePlatformProbeTest"}}) {
				if row.MethodName == "P01" {
					own = append(own, row)
				}
			}
			if len(own) != 1 {
				t.Fatalf("discovered P01 methods=%d", len(own))
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			t.Run("anonymous_publish", func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions("CapturePlatformProbe.emit('P01');", vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = runner.newMachine().Execute(program)
				assertPlatformTransactionObservation(t, "P01", err, anonymous["P01"])
			})
			t.Run("named_istest_publish", func(t *testing.T) {
				methods, methodErrors := compileTestMethodsForIndex(&index, own)
				programs, programErrors := compileTestInvokePrograms(own)
				row := own[0]
				key := testCaseKey(row)
				if methodErrors[key] != nil || programErrors[key] != nil {
					t.Fatalf("compile: %v %v", methodErrors[key], programErrors[key])
				}
				machine := runner.newMachine()
				machine.EnableTestContext()
				if err := machine.RegisterMethod(methods[key]); err != nil {
					t.Fatal(err)
				}
				_, err := machine.ExecuteTestInClass(programs[key], row.ClassName)
				assertPlatformTransactionObservation(t, "P01", err, named["P01"])
			})
			t.Run("project_test_runner_publish", func(t *testing.T) {
				report := RunCasesContext(context.Background(), index, Options{SelectedClasses: []string{"CapturePlatformProbeTest"}, NoDiskCache: true, Parallelism: 1}, own)
				count := 0
				for _, suite := range report.Suites {
					for _, row := range suite.Cases {
						count++
						if row.Problem == nil {
							t.Fatalf("missing observation: %+v", row)
						}
						assertPlatformTransactionPayload(t, "P01", row.Problem.Message, named["P01"])
					}
				}
				if count != 1 {
					t.Fatalf("project results=%d", count)
				}
			})
		})
	}
}

// Unsupported actions remain a local boundary at every nesting level. This
// synthetic variant of the native PE05 loop must never invoke such an action.
func TestPlatformEventFlowLoopAllowlistSyntheticProjectAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"top_level", "loop"} {
			t.Run(api+"/"+shape, func(t *testing.T) {
				root := eventElementsProject(t, api)
				path := filepath.Join(root, "force-app/main/default/flows/ElementSubscriber.flow-meta.xml")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				// Replace the loop's create with an executable but unsupported PE action.
				source := strings.Replace(string(raw), "<targetReference>WriteLoopLog</targetReference>", "<targetReference>SendMail</targetReference>", 1)
				action := `<actionCalls><name>SendMail</name><label>Send mail</label><locationX>0</locationX><locationY>0</locationY><connector><targetReference>Each</targetReference></connector><actionName>emailSimple</actionName><actionType>emailSimple</actionType><inputParameters><name>emailSubject</name><value><stringValue>Loop boundary</stringValue></value></inputParameters><inputParameters><name>emailBody</name><value><stringValue>Boundary body</stringValue></value></inputParameters></actionCalls>`
				if shape == "top_level" {
					source = strings.Replace(source, "<targetReference>GetMany</targetReference>", "<targetReference>SendMail</targetReference>", 1)
					action = strings.Replace(action, "<connector><targetReference>Each</targetReference></connector>", "", 1)
				}
				source = strings.Replace(source, "</Flow>", action+"</Flow>", 1)
				writeFile(t, path, source)
				index := loadTestIndex(t, root)
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				for _, route := range []string{"anonymous_committed", "istest_deliver", "istest_stop"} {
					t.Run(route, func(t *testing.T) {
						machine := runner.newMachine()
						before, after := "", ""
						if route != "anonymous_committed" {
							machine.EnableTestContext()
						}
						if route == "istest_deliver" {
							after = "Test.getEventBus().deliver();"
						}
						if route == "istest_stop" {
							before = "Test.startTest();"
							after = "Test.stopTest();"
						}
						program, err := vm.CompileAnonymousWithOptions(before+"ElementProbe.publish('Loop boundary','loop');"+after, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						result, err := machine.Execute(program)
						if err == nil && route == "anonymous_committed" {
							err = machine.DrainAsync(&result)
						}
						if err == nil || !strings.Contains(err.Error(), "unsupported side-effect steps") {
							t.Fatalf("unsupported PE action bypassed the gate: %v", err)
						}
					})
				}
			})
		}
	}
}
