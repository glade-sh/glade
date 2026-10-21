package vm

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native sharing rows distinguish an omitted test entry from an ordinary
// omitted helper. Explicit keywords and the API67 default retain precedence.
func TestTestEntrySharingUsesResolvedCallerAndKeywords(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, test := range []struct {
			name, keyword, entry string
			helper               bool
			want                 int
		}{
			{name: "omitted entry", want: 1},
			{name: "inherited entry", keyword: "inherited sharing", want: 1},
			{name: "explicit without entry", keyword: "without sharing", want: 2},
			{name: "ordinary helper under without", keyword: "without sharing", helper: true, want: 2},
			{name: "ordinary helper under with", keyword: "with sharing", helper: true, want: 1},
			{name: "existing entry preserved", entry: "without sharing", want: 2},
		} {
			t.Run(api+"/"+test.name, func(t *testing.T) {
				want := test.want
				if api == "67.0" && (test.helper || test.entry != "") {
					want = 1
				}
				machine := New(io.Discard)
				org := privateExecutionPolicyOrg()
				machine.SetOrg(&org)
				machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
				machine.entrySharingMode = test.entry
				machine.EnableTestContext()
				machine.testContext.RunAsDepth = 1
				body := fmt.Sprintf("System.assertEquals(%d, [SELECT Id FROM Widget__c WITH SYSTEM_MODE].size());", want)
				if test.helper {
					program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterClass(Class{Name: "Helper", APIVersion: api}); err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterMethod(Method{Name: "Helper.run", ClassName: "Helper", IsStatic: true, Access: "public", APIVersion: api, SourceContextBound: true, Program: program}); err != nil {
						t.Fatal(err)
					}
					body = "Helper.run();"
				}
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				class := Class{Name: "Entry", APIVersion: api}
				if test.keyword != "" {
					class.Modifiers = []string{test.keyword}
				}
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterMethod(Method{Name: "Entry.run", ClassName: "Entry", IsStatic: true, Access: "public", APIVersion: api, SourceContextBound: true, SharingMode: test.keyword, Program: program}); err != nil {
					t.Fatal(err)
				}
				invoke, err := CompileAnonymous("Entry.run();")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.ExecuteTestInClass(invoke, "Entry"); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Fresh native Queueable R021/R022/R023 at62/67/68 establish callback sharing
// independent of the omitted test entry. Inherited cases preserve existing
// explicit-parent/trigger caller behavior; they are not additional native rows.
func TestQueueableSharingEntryUsesOwnPolicyAndRestoresParent(t *testing.T) {
	for _, bound := range []bool{true, false} {
		for _, api := range []string{"62.0", "67.0", "68.0"} {
			for _, keyword := range []string{"", "with sharing", "without sharing", "inherited sharing"} {
				for _, fail := range []bool{false, true} {
					t.Run(fmt.Sprintf("bound=%t/%s/%s/error=%t", bound, api, keyword, fail), func(t *testing.T) {
						want := 2
						if keyword == "with sharing" || keyword == "" && api != "62.0" || keyword == "inherited sharing" && !fail {
							want = 1
						}
						machine := New(io.Discard)
						org := privateExecutionPolicyOrg()
						machine.SetOrg(&org)
						machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
						machine.EnableTestContext()
						machine.testContext.RunAsDepth = 1
						body := fmt.Sprintf("System.assertEquals(%d, [SELECT Id FROM Widget__c WITH SYSTEM_MODE].size()); System.assertEquals((Id)'005000000000001', UserInfo.getUserId());", want)
						if fail {
							body += " throw new IllegalArgumentException('callback failure');"
						}
						program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						class := Class{Name: "Callback", APIVersion: api}
						if keyword != "" {
							class.Modifiers = []string{keyword}
						}
						if err := machine.RegisterClass(class); err != nil {
							t.Fatal(err)
						}
						method := Method{Name: "Callback.execute", ClassName: "Callback", Access: "public", APIVersion: api, SourceContextBound: bound, Program: program}
						if bound {
							method.SharingMode = keyword
						}
						if err := machine.RegisterMethod(method); err != nil {
							t.Fatal(err)
						}
						machine.entrySharingMode = "with sharing"
						machine.currentClass = "Parent"
						machine.currentMethod = Method{Name: "Parent.run", ClassName: "Parent", APIVersion: "68.0", SourceContextBound: true, SharingMode: "with sharing"}
						machine.currentTrigger = fail
						machine.callStack = []callFrame{{Symbol: "Parent.run", SharingMode: "with sharing", APIVersion: "68.0"}}
						parentMethod, parentStack := machine.currentMethod, append([]callFrame(nil), machine.callStack...)
						parentUser := machine.testContext.CurrentUser
						err = machine.runAsyncJobInTransaction(AsyncJob{ID: "707000000000001", Kind: "Queueable", Object: Object("Callback")}, &Result{})
						if fail {
							if err == nil || !strings.Contains(err.Error(), "callback failure") {
								t.Fatalf("callback error = %v", err)
							}
						} else if err != nil {
							t.Fatal(err)
						}
						if machine.currentClass != "Parent" || !reflect.DeepEqual(machine.currentMethod, parentMethod) ||
							machine.currentTrigger != fail || !reflect.DeepEqual(machine.callStack, parentStack) || machine.entrySharingMode != "with sharing" {
							t.Fatal("async dispatch did not restore parent sharing state")
						}
						if !reflect.DeepEqual(machine.testContext.CurrentUser, parentUser) || machine.testContext.RunAsDepth != 1 {
							t.Fatal("async dispatch changed the restricted principal")
						}
					})
				}
			}
		}
	}
}

// The marker is scoped to the runner's real invocation, including error exits;
// enabling Test APIs alone must retain anonymous execution's sharing default.
func TestTestEntrySharingScopeRestoresOnError(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			machine := New(io.Discard)
			machine.EnableTestContext()
			if machine.entrySharingMode != "" || machine.testExecutionEntry != nil {
				t.Fatal("enabling Test APIs changed anonymous entry sharing")
			}
			body := "System.assertEquals(1, 1);"
			if fail {
				body = "throw new IllegalArgumentException('entry failure');"
			}
			program, err := CompileAnonymous(body)
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.ExecuteTestInClass(program, "Entry")
			if fail && (err == nil || !strings.Contains(err.Error(), "entry failure")) {
				t.Fatalf("entry error = %v", err)
			}
			if !fail && err != nil {
				t.Fatal(err)
			}
			if machine.entrySharingMode != "" || machine.testExecutionEntry != nil {
				t.Fatal("test entry scope was not restored")
			}
		})
	}
}

// Native R038 covers only direct omitted test entries at62/68. Explicit parents
// and other async routes here check preservation of their resolved caller mode.
func TestAsyncEntrySharingOverrideKeepsInheritedCallerBoundary(t *testing.T) {
	for _, bound := range []bool{true, false} {
		for _, api := range []string{"62.0", "68.0"} {
			parentWant, mixedAPI, mixedWant := 2, "68.0", 1
			if api == "68.0" {
				parentWant, mixedAPI, mixedWant = 1, "62.0", 2
			}
			tests := []struct {
				name, parent, parentAPI string
				queueable               bool
				want                    int
			}{
				{"native direct omitted Queueable", "", api, true, 2},
				{"explicit with Queueable", "with sharing", api, true, 1},
				{"explicit without Queueable", "without sharing", api, true, 2},
				{"explicit with other callback", "with sharing", api, false, 1},
				{"explicit without other callback", "without sharing", api, false, 2},
				{"direct omitted other callback", "", api, false, parentWant},
				{"mixed parent API other callback", "", mixedAPI, false, mixedWant},
			}
			for _, test := range tests {
				t.Run(fmt.Sprintf("bound=%t/%s/%s", bound, api, test.name), func(t *testing.T) {
					machine := New(io.Discard)
					org := privateExecutionPolicyOrg()
					machine.SetOrg(&org)
					machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
					machine.EnableTestContext()
					machine.testContext.RunAsDepth = 1
					machine.entrySharingMode = "with sharing"
					machine.testExecutionEntry = &testExecutionEntry{className: "Entry", depth: 1}
					machine.currentClass = "Entry"
					machine.currentMethod = Method{Name: "Entry.run", ClassName: "Entry", APIVersion: test.parentAPI, SourceContextBound: true, SharingMode: test.parent}
					machine.callStack = []callFrame{{Symbol: "Entry.run", SharingMode: "with sharing", APIVersion: test.parentAPI}}
					program, err := CompileAnonymousWithOptions(fmt.Sprintf("System.assertEquals(%d, [SELECT Id FROM Widget__c WITH SYSTEM_MODE].size()); System.assertEquals((Id)'005000000000001', UserInfo.getUserId());", test.want), CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterClass(Class{Name: "Callback", APIVersion: api, Modifiers: []string{"inherited sharing"}}); err != nil {
						t.Fatal(err)
					}
					method := Method{Name: "Callback.execute", ClassName: "Callback", APIVersion: api, SourceContextBound: bound, Program: program}
					if bound {
						method.SharingMode = "inherited sharing"
					}
					parentMethod, parentStack, parentEntry := machine.currentMethod, append([]callFrame(nil), machine.callStack...), machine.testExecutionEntry
					if _, err := machine.callAsyncEntry(method, Object("Callback"), nil, &Result{}, test.queueable); err != nil {
						t.Fatal(err)
					}
					if machine.currentClass != "Entry" || !reflect.DeepEqual(machine.currentMethod, parentMethod) || !reflect.DeepEqual(machine.callStack, parentStack) || machine.testExecutionEntry != parentEntry || machine.entrySharingMode != "with sharing" {
						t.Fatal("callback altered parent execution context")
					}
				})
			}
		}
	}
}

// An async callback's ordinary synchronous helper must inherit the callback,
// even while the existing async-kind marker is active.
func TestAsyncEntrySharingPropagatesToSynchronousHelper(t *testing.T) {
	machine := New(io.Discard)
	org := privateExecutionPolicyOrg()
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
	machine.EnableTestContext()
	machine.testContext.RunAsDepth = 1
	program, err := CompileAnonymousWithOptions("System.assertEquals(1, [SELECT Id FROM Widget__c WITH SYSTEM_MODE].size());", CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Helper", APIVersion: "62.0"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Helper.run", ClassName: "Helper", IsStatic: true, Access: "public", APIVersion: "62.0", SourceContextBound: true, Program: program}); err != nil {
		t.Fatal(err)
	}
	callback, err := CompileAnonymous("Helper.run();")
	if err != nil {
		t.Fatal(err)
	}
	method := Method{Name: "Callback.execute", ClassName: "Callback", APIVersion: "62.0", SourceContextBound: true, SharingMode: "with sharing", Program: callback}
	_, err = machine.withAsyncKind("Queueable", func() (Value, error) { return machine.callAsyncEntry(method, Object("Callback"), nil, &Result{}, true) })
	if err != nil {
		t.Fatal(err)
	}
}

// Older R024 is native at62/65/66/67; API68 here checks preservation of the
// existing secure-default gate, without claiming a fresh Future observation.
func TestFutureOmittedEntryPreservesSourceVersionDefault(t *testing.T) {
	for _, bound := range []bool{true, false} {
		for _, api := range []string{"62.0", "65.0", "66.0", "67.0", "68.0"} {
			t.Run(fmt.Sprintf("bound=%t/%s", bound, api), func(t *testing.T) {
				want := 2
				if api == "67.0" || api == "68.0" {
					want = 1
				}
				machine := New(io.Discard)
				org := privateExecutionPolicyOrg()
				machine.SetOrg(&org)
				machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
				machine.EnableTestContext()
				machine.testContext.RunAsDepth = 1
				machine.entrySharingMode = "with sharing"
				machine.currentClass = "Parent"
				machine.currentMethod = Method{Name: "Parent.run", ClassName: "Parent", SourceContextBound: true, SharingMode: "with sharing", APIVersion: "62.0"}
				program, err := CompileAnonymousWithOptions(fmt.Sprintf("System.assertEquals(%d, [SELECT Id FROM Widget__c WITH SYSTEM_MODE].size()); System.assertEquals((Id)'005000000000001', UserInfo.getUserId());", want), CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{Name: "Callback", APIVersion: api}); err != nil {
					t.Fatal(err)
				}
				method := Method{Name: "Callback.futureRun", ClassName: "Callback", IsStatic: true, SourceContextBound: bound, APIVersion: api, Program: program}
				parent := machine.currentMethod
				if err := machine.runAsyncJobInTransaction(AsyncJob{Kind: "Future", Method: method}, &Result{}); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(machine.currentMethod, parent) || machine.currentClass != "Parent" || machine.entrySharingMode != "with sharing" {
					t.Fatal("Future callback altered parent context")
				}
			})
		}
	}
}
