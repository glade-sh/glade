package vm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

// These VM unit routes execute the same owned Apex bodies as the native
// assignment_save_routes captures, with methods, metadata and triggers
// registered directly. Project loading and test-runner twins live in apextest.
func TestAssignmentSaveVMAnonymousAndIsTestAPIRoutes(t *testing.T) {
	testAssignmentSaveVMAPIRoutes(t, "assignment_save_routes")
}

func TestAssignmentSaveVMScalarAndFilteredAnonymousAndIsTestAPIRoutes(t *testing.T) {
	testAssignmentSaveVMAPIRoutes(t, "assignment_save_edge_routes")
}

func TestAssignmentSaveVMUserModeAnonymousAndIsTestAPIRoutes(t *testing.T) {
	testAssignmentSaveVMAPIRoutes(t, "assignment_save_user_routes")
}

func testAssignmentSaveVMAPIRoutes(t *testing.T, fixture string, routes ...string) {
	t.Helper()
	if len(routes) == 0 {
		routes = []string{"anonymous", "istest"}
	}
	root := filepath.Join("..", "apextest", "testdata", "conformance", fixture)
	raw, err := os.ReadFile(filepath.Join(root, "project", "force-app", "main", "default", "classes", "AssignmentSaveProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	_, observe, ok := strings.Cut(source, "public static void observe(List<SObject> rows, Map<Id,SObject> oldRows, Boolean beforeEvent) {")
	if !ok {
		t.Fatal("missing native observer")
	}
	observe, run, ok := strings.Cut(observe, "    public static String run(String caseId) {")
	if !ok {
		t.Fatal("missing native entry")
	}
	observe = strings.TrimSuffix(strings.TrimSpace(observe), "}")
	run = strings.TrimSuffix(strings.TrimSpace(run), "}")
	run = strings.TrimSuffix(strings.TrimSpace(run), "}")
	triggerSource, err := os.ReadFile(filepath.Join(root, "project", "force-app", "main", "default", "triggers", "AssignmentSaveLead.trigger"))
	if err != nil {
		t.Fatal(err)
	}
	operations := []string{"update"}
	if strings.Contains(string(triggerSource), "before insert") {
		operations = append(operations, "insert")
	}
	for _, api := range []string{"62.0", "67.0"} {
		compile := func(body string) ir.Program {
			t.Helper()
			program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			return program
		}
		observeProgram, runProgram := compile(observe), compile(run)
		trigger := compile("AssignmentSaveProbe.observe(Trigger.new, Trigger.oldMap, Trigger.isBefore);")
		for _, route := range routes {
			path := filepath.Join(root, "api"+strings.TrimSuffix(api, ".0"), route, "org.tsv")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for line, row := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				cells := strings.SplitN(row, "\t", 4)
				if len(cells) != 4 {
					t.Fatalf("invalid native row %q", row)
				}
				id := cells[0]
				t.Run(api+"/"+route+"/"+id, func(t *testing.T) {
					org := storage.NewOrgState()
					org.APIVersion = api
					storage.EnsureDeterministicPlatformData(&org)
					for _, object := range []string{"Lead", "Case", "Group"} {
						storage.EnsureStandardObject(&org, object)
					}
					for _, object := range []string{"Lead", "Case"} {
						state := org.Objects[object]
						field, value := "Company", "Route"
						if object == "Case" {
							field, value = "Origin", "Web"
						}
						if err := json.Unmarshal([]byte(fmt.Sprintf(`{"assignmentRules":[{"name":"AssignmentSaveRule","active":true,"entries":[{"assignedTo":"AssignmentSaveQueue","assignedToType":"Queue","criteria":[{"field":%q,"operation":"equals","value":%q}]}]}]}`, field, value)), &state.Definition); err != nil {
							t.Fatal(err)
						}
						org.Objects[object] = state
					}
					group := org.Objects["Group"]
					queue := storage.ID("00G000000000001")
					if group.Records == nil {
						group.Records = map[storage.ID]storage.Record{}
					}
					group.Records[queue] = storage.Record{Object: "Group", ID: queue, Fields: map[string]storage.Value{"Type": storage.StringValue("Queue"), "DeveloperName": storage.StringValue("AssignmentSaveQueue")}}
					org.Objects["Group"] = group
					machine := New(nil)
					machine.SetOrg(&org)
					machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
					if err := machine.RegisterClass(Class{Name: "AssignmentSaveProbe", APIVersion: api, Access: "public", StaticFields: map[string]Field{
						"rejectAt": {Name: "rejectAt", Type: "String", Static: true, Access: "public", Value: String(""), InitialValue: String("")},
						"events":   {Name: "events", Type: "List<String>", Static: true, Access: "public", Value: List(), InitialValue: List()},
					}, Methods: map[string]Method{
						"observe": {Name: "AssignmentSaveProbe.observe", ClassName: "AssignmentSaveProbe", IsStatic: true, Access: "public", APIVersion: api, ReturnType: "void", Params: []Param{{Name: "rows", Type: "List<SObject>"}, {Name: "oldRows", Type: "Map<Id,SObject>"}, {Name: "beforeEvent", Type: "Boolean"}}, Program: observeProgram},
						"run":     {Name: "AssignmentSaveProbe.run", ClassName: "AssignmentSaveProbe", IsStatic: true, Access: "public", APIVersion: api, ReturnType: "String", Params: []Param{{Name: "caseId", Type: "String"}}, Program: runProgram},
					}}); err != nil {
						t.Fatal(err)
					}
					for _, object := range []string{"Lead", "Case"} {
						for _, operation := range operations {
							for _, timing := range []string{triggerTimingBefore, triggerTimingAfter} {
								if err := machine.RegisterTrigger(Trigger{Name: "AssignmentSave" + object, Object: object, Timing: timing, Operation: operation, APIVersion: api, Program: trigger}); err != nil {
									t.Fatal(err)
								}
							}
						}
					}
					program := compile("System.assert(false, 'ASSIGN_SAVE|" + id + "|' + AssignmentSaveProbe.run('" + id + "'));")
					if route == "istest" {
						machine.EnableTestContext()
						if err := machine.RegisterMethod(Method{Name: "AssignmentSaveUnit.observe", ClassName: "AssignmentSaveUnit", IsStatic: true, Access: "public", APIVersion: api, Modifiers: []string{"isTest"}, Program: program}); err != nil {
							t.Fatal(err)
						}
						_, err = machine.ExecuteTestInClass(compile("AssignmentSaveUnit.observe();"), "AssignmentSaveUnit")
					} else {
						_, err = machine.Execute(program)
					}
					var fault *RuntimeError
					if !errors.As(err, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("native observation completed; VM=%v", err)
					}
					_, payload, ok := strings.Cut(fault.ExceptionMessage(), "ASSIGN_SAVE|"+id+"|")
					var got, want map[string]any
					if !ok || json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(cells[3]), &want) != nil {
						t.Fatalf("invalid observation: %v", err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("native %s:%d=%v; VM=%v", path, line+2, want, got)
					}
				})
			}
		}
	}
}
