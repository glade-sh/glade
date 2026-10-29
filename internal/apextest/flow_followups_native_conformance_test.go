package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// TestFlowFollowupsNativeConformance replays the captured project without
// replacing the native observations with Go expectations. The reused routes
// share the linked runtime or source index, with fresh transaction/static state;
// they do not claim coverage of the on-disk runtime cache.
func TestFlowFollowupsNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_followups"
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	capture := read("RUN-FACTS.txt")
	for _, fact := range []string{"native_complete=true", "source_apis=62.0,67.0"} {
		if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
			t.Fatalf("missing native capture fact: %s", fact)
		}
	}
	rows := strings.Split(strings.TrimSuffix(read("org.tsv"), "\n"), "\n")
	if len(rows) < 2 || rows[0] != "source_api\troute\tid\tobservation" {
		t.Fatal("invalid native observation table")
	}
	want := make(map[string]map[string]any)
	for _, line := range rows[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 || (cells[0] != "62.0" && cells[0] != "67.0") || (cells[1] != "anonymous" && cells[1] != "istest") {
			t.Fatalf("invalid native row: %s", line)
		}
		if !strings.HasPrefix(cells[2], "AO") && !strings.HasPrefix(cells[2], "FV_") && !strings.HasPrefix(cells[2], "CE_") && !strings.HasPrefix(cells[2], "AR_") {
			t.Fatalf("unknown native family: %s", cells[2])
		}
		if cells[1] == "anonymous" && strings.HasPrefix(cells[2], "AR_") {
			t.Fatal("runAs requires the captured named test route")
		}
		key := cells[0] + "/" + cells[1]
		if want[key] == nil {
			want[key] = make(map[string]any)
		}
		var observation any
		if _, duplicate := want[key][cells[2]]; duplicate || json.Unmarshal([]byte(cells[3]), &observation) != nil || observation == nil {
			t.Fatalf("invalid or duplicate native observation: %s/%s", key, cells[2])
		}
		want[key][cells[2]] = observation
	}

	families := []struct {
		name, class, script, marker string
		prefixes                    []string
	}{
		{"action_outputs", "FollowupActionTest", "action.apex", "FF_ACTION|", []string{"AO"}},
		{"variables_and_number_text", "FFOtherProbeTest", "other.apex", "FF_OTHER|", []string{"FV_", "CE_"}},
		{"assignment_context", "FFAssignmentProbeTest", "", "FF_OTHER|", []string{"AR_"}},
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			projectRoot := filepath.Join(fixture, "project")
			files := 0
			if err := filepath.WalkDir(projectRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(projectRoot, path)
				if err != nil {
					return err
				}
				source := strings.ReplaceAll(string(body), "62.0", api)
				pin := fmt.Sprintf("source_sha256:%s:%s=%x\n", api, filepath.ToSlash(rel), sha256.Sum256([]byte(source)))
				if !strings.Contains("\n"+capture, "\n"+pin) {
					return fmt.Errorf("native source changed: %s/%s", api, rel)
				}
				writeFile(t, filepath.Join(root, rel), source)
				files++
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if files == 0 {
				t.Fatal("native project is empty")
			}
			index := loadTestIndex(t, root)
			org, err := orgFromIndexWithError(index)
			if err != nil {
				t.Fatal(err)
			}
			org.APIVersion = api
			// Standard setup records include the administrator and Standard User
			// profile used by the captured runAs source.
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, family := range families {
				t.Run(family.name, func(t *testing.T) {
					selectRows := func(route string) map[string]any {
						t.Helper()
						selected := make(map[string]any)
						for _, prefix := range family.prefixes {
							count := 0
							for id, value := range want[api+"/"+route] {
								if strings.HasPrefix(id, prefix) {
									selected[id] = value
									count++
								}
							}
							if count == 0 {
								t.Fatalf("missing native family: %s/%s/%s", api, route, prefix)
							}
						}
						return selected
					}
					if family.script != "" {
						for _, route := range []string{"anonymous", "anonymous_reused_runtime"} {
							t.Run(route, func(t *testing.T) {
								body, err := os.ReadFile(filepath.Join(root, family.script))
								if err != nil {
									t.Fatal(err)
								}
								program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								_, execErr := runner.execute(program, func(machine *vm.VM) {
									machine.SetCurrentUser(org.Objects["User"].Records[storage.DefaultUserID])
								})
								var runtimeErr *vm.RuntimeError
								if !errors.As(execErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
									t.Fatalf("missing native observation assertion: %v", execErr)
								}
								checkFlowFollowupsPayload(t, runtimeErr.ExceptionMessage(), family.marker, selectRows("anonymous"))
							})
						}
					}
					for _, route := range []string{"project_runner", "project_runner_reused_index"} {
						t.Run(route, func(t *testing.T) {
							// Apex source/metadata versions come from the captured files.
							// The runner's REST schema version is an independent setting.
							result := Run(index, Options{SelectedClasses: []string{family.class}, SelectedMethod: "observe", RuntimeRESTAPIVersion: "67.0", NoDiskCache: true, Parallelism: 1})
							if result.Summary().Total != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 {
								t.Fatalf("unexpected project selection: %+v; %s", result.Summary(), firstRunProblem(result))
							}
							problem := result.Suites[0].Cases[0].Problem
							if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
								t.Fatalf("missing native observation assertion: %+v", problem)
							}
							checkFlowFollowupsPayload(t, problem.Message, family.marker, selectRows("istest"))
						})
					}
					if family.name == "assignment_context" {
						namedOrg := org.Clone()
						// Match the runner's REST schema context; pinned class APIs stay unchanged.
						namedOrg.APIVersion = "67.0"
						namedRunner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &namedOrg})
						for _, route := range []string{"named_vm", "named_vm_reused_runtime"} {
							t.Run(route, func(t *testing.T) {
								machine := namedRunner.newMachine()
								machine.SetCurrentUser(machine.Org.Objects["User"].Records[storage.DefaultUserID])
								machine.EnableTestContext()
								if err := RegisterTestMethodForRequest(machine, index, family.class, "observe"); err != nil {
									t.Fatal(err)
								}
								for _, name := range []string{"FF_ProfileQueue", "FF_UserQueue"} {
									groups, links := flowFollowupsQueueSetup(machine.Org, name)
									if groups != 1 || links != 1 {
										t.Fatalf("captured queue metadata missing: %s groups=%d Lead links=%d", name, groups, links)
									}
								}
								var diagnostic *flowFollowupsAssignmentDiagnostic
								machine.SetDebugHooks(vm.DebugHooks{
									Breakpoints: []vm.DebugBreakpoint{{File: family.class + ".cls", Line: 12}},
									OnPause: func(pause vm.DebugPause) vm.DebugAction {
										lead := pause.Vars["lead"]
										id := storage.ID(lead.Fields["Id"].String())
										observed := flowFollowupsInspectAssignment(machine.Org, pause, id)
										diagnostic = &observed
										if !diagnostic.Stored || diagnostic.Deleted || diagnostic.RawRows != 1 || diagnostic.RawError != "" {
											t.Errorf("successful insert is absent before captured query: %+v", diagnostic)
											return vm.DebugActionStop
										}
										return vm.DebugActionContinue
									},
								})
								program, err := vm.CompileAnonymousWithOptions(family.class+".observe();", vm.CompileOptions{APIVersion: api})
								if err != nil {
									t.Fatal(err)
								}
								_, execErr := machine.ExecuteTestInClass(program, family.class)
								defer func() {
									if t.Failed() {
										t.Logf("last captured pre-query row: %+v", diagnostic)
									}
								}()
								var runtimeErr *vm.RuntimeError
								if !errors.As(execErr, &runtimeErr) || !strings.HasSuffix(runtimeErr.Type, "AssertException") {
									t.Fatalf("missing native observation assertion: %v", execErr)
								}
								if diagnostic == nil {
									t.Fatal("captured insert/query breakpoint was not reached")
								}
								checkFlowFollowupsPayload(t, runtimeErr.ExceptionMessage(), family.marker, selectRows("istest"))
							})
						}
					}
				})
			}
		})
	}
}

// These diagnostics observe the pinned source immediately before its Lead
// query. The raw query deliberately bypasses VM sharing to distinguish missing
// storage or binding inputs from a record visibility failure.
type flowFollowupsAssignmentDiagnostic struct {
	Company           string
	Stored, Deleted   bool
	RawRows           int
	RawError          string
	InputOwnerProfile string
	CreatedByProfile  string
	OwnerType, Queue  string
	QueueLinks        int
	QueueMembers      int
	SharingModel      string
}

func flowFollowupsInspectAssignment(org *storage.OrgState, pause vm.DebugPause, id storage.ID) flowFollowupsAssignmentDiagnostic {
	_, record, exists := storage.LookupRecordByID(org.Objects["Lead"].Records, id)
	diagnostic := flowFollowupsAssignmentDiagnostic{
		Company: pause.Vars["company"].String(), Stored: exists, Deleted: record.System.IsDeleted,
		SharingModel: org.Objects["Lead"].Definition.SharingModel,
	}
	profileName := func(userID storage.ID) string {
		_, user, _ := storage.LookupRecordByID(org.Objects["User"].Records, userID)
		_, profile, _ := storage.LookupRecordByID(org.Objects["Profile"].Records, user.Fields["ProfileId"].ID)
		return profile.Fields["Name"].String
	}
	diagnostic.InputOwnerProfile = profileName(storage.ID(pause.Vars["owner"].String()))
	diagnostic.CreatedByProfile = profileName(record.System.CreatedByID)
	_, group, isGroup := storage.LookupRecordByID(org.Objects["Group"].Records, record.System.OwnerID)
	if isGroup {
		diagnostic.OwnerType, diagnostic.Queue = group.Fields["Type"].String, group.Fields["DeveloperName"].String
		_, diagnostic.QueueLinks = flowFollowupsQueueSetup(org, diagnostic.Queue)
		for _, member := range org.Objects["GroupMember"].Records {
			if !member.System.IsDeleted && storage.IDsEqual(member.Fields["GroupId"].ID, record.System.OwnerID) {
				diagnostic.QueueMembers++
			}
		}
	} else if _, _, isUser := storage.LookupRecordByID(org.Objects["User"].Records, record.System.OwnerID); isUser {
		diagnostic.OwnerType = "User"
	}
	raw, err := soql.Execute(*org, soql.Query{Object: "Lead", Fields: []string{"OwnerId"}, Where: &soql.Condition{Field: "Id", Op: "=", Value: storage.IDValue(id)}})
	diagnostic.RawRows = len(raw.Records)
	if err != nil {
		diagnostic.RawError = err.Error()
	}
	return diagnostic
}

func flowFollowupsQueueSetup(org *storage.OrgState, name string) (groups, links int) {
	for _, group := range org.Objects["Group"].Records {
		if group.System.IsDeleted || group.Fields["Type"].String != "Queue" || group.Fields["DeveloperName"].String != name {
			continue
		}
		groups++
		for _, link := range org.Objects["QueueSobject"].Records {
			if !link.System.IsDeleted && storage.IDsEqual(link.Fields["QueueId"].ID, group.ID) && link.Fields["SobjectType"].String == "Lead" {
				links++
			}
		}
	}
	return groups, links
}

func checkFlowFollowupsPayload(t *testing.T, message, marker string, want map[string]any) {
	t.Helper()
	_, payload, found := strings.Cut(message, marker)
	var got map[string]any
	if !found || json.Unmarshal([]byte(payload), &got) != nil || got == nil {
		t.Fatalf("invalid native observation envelope: %s", message)
	}
	if len(got) != len(want) {
		t.Errorf("observed case count: native=%d; Glade=%d", len(want), len(got))
	}
	ids := make([]string, 0, len(want))
	for id := range want {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			actual, exists := got[id]
			if !exists || !reflect.DeepEqual(actual, want[id]) {
				native, _ := json.Marshal(want[id])
				observed, _ := json.Marshal(actual)
				t.Fatalf("native=%s; Glade=%s; observed=%t", native, observed, exists)
			}
		})
	}
}
