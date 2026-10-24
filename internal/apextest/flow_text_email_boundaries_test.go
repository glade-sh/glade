package apextest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// These synthetic project routes pin rejection/rollback boundaries, not new
// Salesforce parity claims. Raw DoesNotContain was rejected in the adjacent
// flow_text_email capture. Null prefix/suffix are supported and false in
// flow_text_prefix_suffix/api{62,67}/org.tsv startswith_null and endswith_null;
// Email Alert retains its uncaptured boundary.
func TestFlowTextEmailBoundaryProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			files := map[string]string{}
			var methods strings.Builder
			for _, route := range []string{"interview", "interview_subflow", "invocable", "invocable_subflow", "trigger", "trigger_plain", "trigger_subflow"} {
				for _, boundary := range []string{"DoesNotContain", "StartsWith", "EndsWith", "emailAlert_false", "emailAlert_omitted", "emailAlert_true", "negatedContains_null", "negatedContains_match"} {
					// Keep the plain trigger free of prelude nodes: it must enter
					// the decision directly, with the same scalar defaults.
					if route == "trigger_plain" && !strings.HasPrefix(boundary, "negatedContains") {
						continue
					}
					name := route + "_" + boundary
					trigger := strings.HasPrefix(route, "trigger")
					invocable := strings.HasPrefix(route, "invocable")
					emailAdapter := route == "invocable"
					email := strings.HasPrefix(boundary, "emailAlert")
					negation := strings.HasPrefix(boundary, "negatedContains")
					variables := `<variables><name>InputText</name><dataType>String</dataType><isInput>true</isInput><isOutput>false</isOutput><isCollection>false</isCollection>`
					if boundary == "DoesNotContain" || boundary == "negatedContains_match" {
						variables += `<value><stringValue>Hello World</stringValue></value>`
					}
					variables += `</variables><variables><name>Matched</name><dataType>Boolean</dataType><isInput>false</isInput><isOutput>true</isOutput><isCollection>false</isCollection></variables>`
					start := "Choose"
					nodes := `<decisions><name>Choose</name><rules><name>Match</name><conditionLogic>and</conditionLogic><conditions><leftValueReference>InputText</leftValueReference><operator>` + boundary + `</operator><rightValue><stringValue>x</stringValue></rightValue></conditions><connector><targetReference>Mark</targetReference></connector></rules></decisions><assignments><name>Mark</name><assignmentItems><assignToReference>Matched</assignToReference><operator>Assign</operator><value><booleanValue>true</booleanValue></value></assignmentItems></assignments>`
					if negation {
						// EB/EC route twins: NOT Contains must keep its selected and
						// fallthrough paths on every entry, including triggered children.
						comparison := "x"
						if boundary == "negatedContains_match" {
							comparison = "world"
						}
						nodes = `<decisions><name>Choose</name><defaultConnector><targetReference>Skip</targetReference></defaultConnector><rules><name>Match</name><conditionLogic>NOT 1</conditionLogic><conditions><leftValueReference>InputText</leftValueReference><operator>Contains</operator><rightValue><stringValue>` + comparison + `</stringValue></rightValue></conditions><connector><targetReference>Mark</targetReference></connector></rules></decisions><recordCreates><name>Mark</name><object>Task</object><inputAssignments><field>Subject</field><value><stringValue>Negation route marker</stringValue></value></inputAssignments></recordCreates><assignments><name>Skip</name><assignmentItems><assignToReference>Matched</assignToReference><operator>Assign</operator><value><booleanValue>false</booleanValue></value></assignmentItems></assignments>`
					}
					if email {
						start = "Send"
						nodes = `<actionCalls><name>Send</name><actionType>emailAlert</actionType><actionName>Account.Alert</actionName>`
						if boundary != "emailAlert_omitted" {
							value := "false"
							if boundary == "emailAlert_true" {
								value = "true"
							}
							nodes += `<inputParameters><name>logEmailOnSend</name><value><booleanValue>` + value + `</booleanValue></value></inputParameters>`
						}
						nodes += `</actionCalls>`
					}
					wrap := func(start, nodes, variables, triggerXML string) string {
						return `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><processType>AutoLaunchedFlow</processType><status>Active</status><start><connector><targetReference>` + start + `</targetReference></connector>` + triggerXML + `</start>` + nodes + variables + `</Flow>`
					}
					if route == "trigger" && !email {
						// A prelude keeps the triggering flow on the ordered frame
						// interpreter, which evaluates its declared scalar resources.
						files["force-app/main/default/flows/BoundaryPrelude.flow-meta.xml"] = wrap("Prepare", `<assignments><name>Prepare</name><assignmentItems><assignToReference>Marker</assignToReference><operator>Assign</operator><value><stringValue>ready</stringValue></value></assignmentItems></assignments>`, `<variables><name>Marker</name><dataType>String</dataType><isInput>false</isInput><isOutput>true</isOutput><isCollection>false</isCollection></variables>`, "")
						nodes += `<subflows><name>Prepare</name><flowName>BoundaryPrelude</flowName><connector><targetReference>` + start + `</targetReference></connector></subflows>`
						start = "Prepare"
					}
					if strings.HasSuffix(route, "_subflow") {
						files["force-app/main/default/flows/"+name+"Child.flow-meta.xml"] = wrap(start, nodes, variables, "")
						start, nodes = "Call", `<subflows><name>Call</name><flowName>`+name+`Child</flowName></subflows>`
					}
					triggerXML := ""
					if trigger {
						triggerXML = `<object>Account</object><triggerType>RecordAfterSave</triggerType><recordTriggerType>Create</recordTriggerType><filterLogic>and</filterLogic><filters><field>Name</field><operator>EqualTo</operator><value><stringValue>` + name + `</stringValue></value></filters>`
					}
					files["force-app/main/default/flows/"+name+".flow-meta.xml"] = wrap(start, nodes, variables, triggerXML)
					call := `Flow.Interview interview=Flow.Interview.createInterview('` + name + `',new Map<String,Object>()); interview.start();`
					if trigger {
						call = `insert new Account(Name='` + name + `');`
					} else if invocable {
						call = `Invocable.Action action=Invocable.Action.createCustomAction('flow','` + name + `'); action.setInvocations(new List<Map<String,Object>>{new Map<String,Object>()}); Invocable.Action.Result result=action.invoke()[0];`
					}
					body := ""
					if negation {
						want := 1
						if boundary == "negatedContains_match" {
							want = 0
						}
						body = call + fmt.Sprintf(`System.assertEquals(%d,[SELECT count() FROM Task],'NOT Contains route');`, want)
					} else if email {
						want := 0
						if trigger || boundary == "emailAlert_true" && !emailAdapter {
							want = 1
						}
						body = call
						if emailAdapter {
							body += `System.assertEquals(false,result.isSuccess(),'email adapter must reject'); System.assert(JSON.serialize(result.getErrors()).contains('hosted Flow action'));`
						} else {
							body += fmt.Sprintf(`System.assertEquals(%d,[SELECT count() FROM EmailMessage],'Email Alert pre-extension count');`, want)
						}
					} else {
						body = call
						if invocable && boundary == "DoesNotContain" {
							body += `System.assertEquals(false,result.isSuccess(),'unsupported criteria must reject'); System.assert(JSON.serialize(result.getErrors()).contains('unsupported criteria'));`
						} else if invocable {
							body += `System.assertEquals(true,result.isSuccess(),JSON.serialize(result.getErrors()));`
						}
					}
					fmt.Fprintf(&methods, "@IsTest static void %s(){%s}\n", name, body)
				}
			}
			index := flowCoreIndex(t, api, map[string]string{"FlowBoundaryProbe": "@IsTest public class FlowBoundaryProbe {" + methods.String() + "}"}, files)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("synthetic project rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			tests := Discover(index, Options{SelectedClasses: []string{"FlowBoundaryProbe"}})
			if len(tests) != 50 {
				t.Fatalf("test routes=%d; want 50", len(tests))
			}
			compiled, compileErrors := compileTestMethodsForIndex(&index, tests)
			programs, programErrors := compileTestInvokePrograms(tests)
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, context := range []string{"anonymous", "istest"} {
				for _, test := range tests {
					t.Run(context+"/"+test.MethodName, func(t *testing.T) {
						key := testCaseKey(test)
						if compileErrors[key] != nil || programErrors[key] != nil {
							t.Fatalf("compile: %v %v", compileErrors[key], programErrors[key])
						}
						machine := runner.newMachine()
						if err := machine.RegisterMethod(compiled[key]); err != nil {
							t.Fatal(err)
						}
						var err error
						if context == "istest" {
							machine.EnableTestContext()
							_, err = machine.ExecuteTestInClass(programs[key], test.ClassName)
						} else {
							program, compileErr := vm.CompileAnonymousWithOptions("FlowBoundaryProbe."+test.MethodName+"();", vm.CompileOptions{APIVersion: api})
							if compileErr != nil {
								t.Fatal(compileErr)
							}
							_, err = machine.Execute(program)
						}
						wantError := ""
						if strings.Contains(test.MethodName, "DoesNotContain") && !strings.HasPrefix(test.MethodName, "invocable") {
							wantError = "unsupported criteria"
						}
						if wantError != "" {
							if err == nil || !strings.Contains(err.Error(), wantError) {
								t.Fatalf("error=%v; want %s boundary", err, wantError)
							}
						} else if err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		})
	}
}
