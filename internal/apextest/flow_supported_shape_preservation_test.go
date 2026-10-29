package apextest

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// These synthetic projects preserve existing local behavior. They do not add
// Salesforce parity claims for uncaptured collection IsBlank or child shapes.
func TestFlowLookupCollectionBlankProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			files := map[string]string{}
			methods := map[string]string{}
			for _, operand := range []string{"true", "false"} {
				name := "CollectionBlank_" + operand
				nodes := `<recordLookups><name>Find</name><object>Contact</object><filterLogic>and</filterLogic><filters><field>LastName</field><operator>EqualTo</operator><value><elementReference>key</elementReference></value></filters><getFirstRecordOnly>false</getFirstRecordOnly><storeOutputAutomatically>true</storeOutputAutomatically><connector><targetReference>Choose</targetReference></connector></recordLookups><decisions><name>Choose</name><defaultConnector><targetReference>Miss</targetReference></defaultConnector><rules><name>Matched</name><conditionLogic>and</conditionLogic><conditions><leftValueReference>Find</leftValueReference><operator>IsBlank</operator><rightValue><booleanValue>` + operand + `</booleanValue></rightValue></conditions><connector><targetReference>Match</targetReference></connector></rules></decisions>`
				for _, result := range []string{"Match", "Miss"} {
					nodes += `<assignments><name>` + result + `</name><assignmentItems><assignToReference>outcome</assignToReference><operator>Assign</operator><value><stringValue>` + result + `</stringValue></value></assignmentItems></assignments>`
				}
				variables := flowPreservationVariable("key", "String", true, false, false) + flowPreservationVariable("outcome", "String", false, true, false)
				files["force-app/main/default/flows/"+name+".flow-meta.xml"] = flowPreservationXML(api, "Find", nodes, variables)
				files["force-app/main/default/flows/Parent_"+name+".flow-meta.xml"] = flowPreservationXML(api, "Call", `<subflows><name>Call</name><flowName>`+name+`</flowName><inputAssignments><name>key</name><value><elementReference>key</elementReference></value></inputAssignments><outputAssignments><assignToReference>outcome</assignToReference><name>outcome</name></outputAssignments></subflows>`, variables)
				for _, count := range []int{0, 1, 2} {
					for _, route := range []string{"interview", "parent_interview", "parent_invocable"} {
						body := strings.Repeat(`insert new Contact(LastName='Blank seed');`, count)
						flow := name
						if route != "interview" {
							flow = "Parent_" + name
						}
						want := "Miss"
						if count == 0 {
							want = "Match"
						}
						if route == "parent_invocable" {
							body += `Invocable.Action action=Invocable.Action.createCustomAction('flow','` + flow + `'); action.setInvocationParameter('key','Blank seed'); Invocable.Action.Result result=action.invoke()[0]; System.assert(result.isSuccess(),JSON.serialize(result.getErrors())); System.assertEquals('` + want + `',result.getOutputParameters().get('outcome'));`
						} else {
							body += `Flow.Interview flow=Flow.Interview.createInterview('` + flow + `',new Map<String,Object>{'key'=>'Blank seed'}); flow.start(); System.assertEquals('` + want + `',flow.getVariableValue('outcome'));`
						}
						methods[fmt.Sprintf("%s_%s_%d", route, operand, count)] = body
					}
				}
			}
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}

func TestFlowChildInterpreterShapeProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			files := map[string]string{}
			methods := map[string]string{}
			shapes := map[string]string{"variable_free": ""}
			for _, dataType := range []string{"String", "Boolean", "Number", "Date", "DateTime", "Currency", "SObject"} {
				shapes["unused_"+dataType+"_collection"] = flowPreservationVariable("unused", dataType, true, true, true)
			}
			for _, dataType := range []string{"Date", "DateTime", "Currency"} {
				shapes["unused_"+dataType+"_input"] = flowPreservationVariable("unused", dataType, true, false, false)
				shapes["unused_"+dataType+"_output"] = flowPreservationVariable("unused", dataType, false, true, false)
			}
			for shape, variables := range shapes {
				child := "Child_" + shape
				nodes := `<recordCreates><name>Create</name><object>Account</object><inputAssignments><field>Name</field><value><stringValue>Child marker</stringValue></value></inputAssignments></recordCreates>`
				files["force-app/main/default/flows/"+child+".flow-meta.xml"] = flowPreservationXML(api, "Create", nodes, variables)
				for _, depth := range []string{"child", "grandchild"} {
					target := child
					if depth == "grandchild" {
						target = "Middle_" + shape
						files["force-app/main/default/flows/"+target+".flow-meta.xml"] = flowPreservationXML(api, "Call", `<subflows><name>Call</name><flowName>`+child+`</flowName></subflows>`, "")
					}
					parent := "Parent_" + depth + "_" + shape
					files["force-app/main/default/flows/"+parent+".flow-meta.xml"] = flowPreservationXML(api, "Call", `<subflows><name>Call</name><flowName>`+target+`</flowName></subflows>`, flowPreservationVariable("token", "String", true, false, false))
					for _, route := range []string{"interview", "invocable"} {
						body := `Flow.Interview flow=Flow.Interview.createInterview('` + parent + `',new Map<String,Object>{'token'=>'seed'}); flow.start();`
						if route == "invocable" {
							body = `Invocable.Action action=Invocable.Action.createCustomAction('flow','` + parent + `'); action.setInvocationParameter('token','seed'); Invocable.Action.Result result=action.invoke()[0]; System.assert(result.isSuccess(),JSON.serialize(result.getErrors()));`
						}
						methods[shape+"_"+depth+"_"+route] = body + `System.assertEquals(1,[SELECT count() FROM Account WHERE Name='Child marker']);`
					}
				}
			}
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}

func TestFlowDeleteLookupEqualityProjectRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			files := map[string]string{}
			nodes := `<recordLookups><name>Find</name><object>Contact</object><filterLogic>and</filterLogic><filters><field>LastName</field><operator>EqualTo</operator><value><elementReference>$Record.Name</elementReference></value></filters><getFirstRecordOnly>true</getFirstRecordOnly><storeOutputAutomatically>true</storeOutputAutomatically><connector><targetReference>Choose</targetReference></connector></recordLookups><decisions><name>Choose</name><defaultConnector><targetReference>Miss</targetReference></defaultConnector><rules><name>Match</name><conditionLogic>and</conditionLogic><conditions><leftValueReference>Find.Title</leftValueReference><operator>EqualTo</operator><rightValue><stringValue>Guide</stringValue></rightValue></conditions><connector><targetReference>Create</targetReference></connector></rules></decisions><recordCreates><name>Create</name><object>Contact</object><inputAssignments><field>LastName</field><value><stringValue>Delete marker</stringValue></value></inputAssignments></recordCreates><assignments><name>Miss</name><assignmentItems><assignToReference>outcome</assignToReference><operator>Assign</operator><value><stringValue>miss</stringValue></value></assignmentItems></assignments>`
			flow := flowPreservationXML(api, "Find", nodes, flowPreservationVariable("outcome", "String", false, true, false))
			flow = strings.Replace(flow, "</start>", `<object>Account</object><triggerType>RecordBeforeDelete</triggerType><recordTriggerType>Delete</recordTriggerType><filterLogic>and</filterLogic><filters><field>Name</field><operator>EqualTo</operator><value><stringValue>Delete seed</stringValue></value></filters></start>`, 1)
			files["force-app/main/default/flows/DeleteLookup.flow-meta.xml"] = flow
			// Preserve the existing explicit boundary instead of silently falling through.
			methods := map[string]string{"deleteEquality": `insert new Contact(LastName='Delete seed',Title='Guide'); Account item=new Account(Name='Delete seed'); insert item; Boolean rejected=false; try { delete item; } catch (DmlException error) { rejected=true; System.assert(error.getMessage().contains('unsupported side-effect steps'),error.getMessage()); } System.assert(rejected,'ordered delete lookup must retain its boundary');`}
			runFlowPreservationProjectRoutes(t, api, files, methods)
		})
	}
}

func flowPreservationVariable(name, dataType string, input, output, collection bool) string {
	object := ""
	if dataType == "SObject" {
		object = "<objectType>Account</objectType>"
	}
	return fmt.Sprintf(`<variables><name>%s</name><dataType>%s</dataType>%s<isInput>%t</isInput><isOutput>%t</isOutput><isCollection>%t</isCollection></variables>`, name, dataType, object, input, output, collection)
}

func flowPreservationXML(api, start, nodes, variables string) string {
	return `<Flow xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><processType>AutoLaunchedFlow</processType><status>Active</status><start><connector><targetReference>` + start + `</targetReference></connector></start>` + nodes + variables + `</Flow>`
}

func runFlowPreservationProjectRoutes(t *testing.T, api string, files, methods map[string]string) {
	t.Helper()
	var names []string
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)
	var source strings.Builder
	for _, name := range names {
		fmt.Fprintf(&source, "@IsTest static void %s(){%s}\n", name, methods[name])
	}
	index := flowCoreIndex(t, api, map[string]string{"FlowPreservationProbe": "@IsTest public class FlowPreservationProbe {" + source.String() + "}"}, files)
	if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
		t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
	}
	t.Run("runner", func(t *testing.T) {
		run := Run(index, Options{SelectedClasses: []string{"FlowPreservationProbe"}, NoDiskCache: true, Parallelism: 1})
		if run.Summary().Total != len(methods) || len(run.Suites) != 1 {
			t.Fatalf("route selection: %+v", run.Summary())
		}
		for _, result := range run.Suites[0].Cases {
			t.Run(result.MethodName, func(t *testing.T) {
				if result.Status != "pass" {
					t.Fatalf("%s: %+v", result.Status, result.Problem)
				}
			})
		}
	})
	t.Run("anonymous", func(t *testing.T) {
		org := orgFromIndex(index)
		org.APIVersion = api
		runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
		for _, name := range names {
			t.Run(name, func(t *testing.T) {
				if analysis := sema.AnalyzeAnonymous(index, methods[name], api); analysis.HasErrors() {
					t.Fatal(analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(methods[name], vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := runner.newMachine().Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	})
}
