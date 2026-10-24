package vm

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestAccessLevelCloneAndDetachedEnvelopeDirectVMNative(t *testing.T) {
	for _, mode := range []string{"USER_MODE", "SYSTEM_MODE"} {
		t.Run(mode, func(t *testing.T) {
			original := accessLevelValue(mode)
			copy := accessLevelClone(original)
			if original.Equal(copy) || valueIdentityEqual(original, copy) {
				t.Error("native clone has a distinct equality and reference identity")
			}
			if !original.Equal(original) || !valueIdentityEqual(original, original) || !original.Equal(accessLevelValue(mode)) {
				t.Error("alias and constant reads must retain identity")
			}
			envelope := Object("AccessEnvelope")
			envelope.Fields["direct"] = original
			envelope.Fields["nested"] = List(original, original)
			detached := cloneValueDetachedPreserveRefs(envelope)
			direct := detached.Fields["direct"]
			nested := detached.Fields["nested"].List
			if original.Equal(direct) || valueIdentityEqual(original, direct) {
				t.Error("native Queueable transport has a distinct identity from the constant")
			}
			if !valueIdentityEqual(direct, nested[0]) || !valueIdentityEqual(nested[0], nested[1]) {
				t.Error("transport must preserve aliases within the detached envelope")
			}
			if direct.Ref == original.Ref {
				t.Error("detached transport retained caller reference")
			}
		})
	}
}

func TestQueueableChainRejectionStopTestAndAnonymousDrainDirectVMNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymousDrain", "isTestStop"} {
			for _, chain := range []bool{false, true} {
				name := "terminal"
				if chain {
					name = "chain"
				}
				t.Run(api+"/"+route+"/"+name, func(t *testing.T) {
					machine := New(nil)
					if route == "isTestStop" {
						machine.EnableTestContext()
					}
					register := func(name, body string) {
						t.Helper()
						program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						err = machine.RegisterClass(Class{Name: name, APIVersion: api, Interfaces: []string{"Queueable"}, Methods: map[string]Method{"execute": {Name: name + ".execute", ClassName: name, APIVersion: api, Params: []Param{{Name: "context", Type: "QueueableContext"}}, Program: program}}})
						if err != nil {
							t.Fatal(err)
						}
					}
					register("TerminalWorker", "System.debug('terminal');")
					register("RootWorker", "System.debug('root');System.enqueueJob(new TerminalWorker());")
					source := "System.enqueueJob(new TerminalWorker());"
					if chain {
						source = "System.enqueueJob(new RootWorker());"
					}
					if route == "isTestStop" {
						source = "Test.startTest();" + source + "try {Test.stopTest();} catch(Exception issue) {System.debug(issue.getTypeName());}"
					}
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					result, err := machine.Execute(program)
					if err != nil {
						t.Fatal(err)
					}
					if route == "anonymousDrain" {
						if err := machine.DrainAsync(&result); err != nil {
							t.Fatal(err)
						}
					}
					want := []string{"terminal"}
					if chain {
						want = []string{"root", "terminal"}
						if route == "isTestStop" {
							want = []string{"root", "System.AsyncException"}
						}
					}
					if machine.HasPendingAsyncWork() {
						t.Error("completed drain retained pending work after rejection or terminal execution")
					}
					if !reflect.DeepEqual(result.Debug, want) {
						t.Fatalf("native=%v; Glade=%v", want, result.Debug)
					}
				})
			}
		}
	}
}

func TestAccessLevelApexOperatorsAnonymousAndIsTestDirectVMNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				machine := New(nil)
				if route == "isTest" {
					machine.EnableTestContext()
				}
				program, err := CompileAnonymousWithOptions(`
     for(AccessLevel value:new List<AccessLevel>{AccessLevel.USER_MODE,AccessLevel.SYSTEM_MODE}) {
      AccessLevel copy=(AccessLevel)value.clone();AccessLevel alias=value;
      System.assertEquals(false,value==copy,'clone equality');
      System.assertEquals(false,value.equals(copy),'clone equals method');
      System.assertEquals(false,value===copy,'clone reference');
      System.assertEquals(true,value==alias,'alias equality');
      System.assertEquals(true,value===alias,'alias reference');
     }
    `, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestAccessLevelDistinctCustomScopesDetachedEnvelopeControl(t *testing.T) {
	for _, route := range []string{"staticFactory", "instanceFactory"} {
		t.Run(route, func(t *testing.T) {
			machine := New(nil)
			org := storage.OrgState{Features: []string{"ApexUserModeWithPermset"}}
			machine.SetOrg(&org)
			makeScope := func(id string) Value {
				t.Helper()
				args := []Value{String(id)}
				if route == "staticFactory" {
					value, err := machine.call("AccessLevel.withPermissionSetId", args, nil, &Result{})
					if err != nil {
						t.Fatal(err)
					}
					return value
				}
				value, handled, err := machine.callEnumMember(accessLevelValue("USER_MODE"), "withPermissionSetId", args)
				if err != nil || !handled {
					t.Fatalf("instance factory: handled=%t error=%v", handled, err)
				}
				return value
			}
			first, second := makeScope("0PS000000000001"), makeScope("0PS000000000002")
			if first.Ref == accessLevelValue("USER_MODE").Ref || second.Ref == accessLevelValue("USER_MODE").Ref {
				t.Fatal("custom factory reused the canonical constant reference")
			}
			envelope := Object("AccessEnvelope")
			envelope.Fields["levels"] = List(first, second, first)
			detached := cloneValueDetachedPreserveRefs(envelope).Fields["levels"].List
			if detached[0].Fields["permissionSetId"].Text != "0PS000000000001" || detached[1].Fields["permissionSetId"].Text != "0PS000000000002" {
				t.Fatal("transport collapsed distinct permission-set payloads")
			}
			if detached[0].Ref != detached[2].Ref || detached[0].Ref == detached[1].Ref {
				t.Fatal("transport changed the payload alias graph")
			}
		})
	}
}

func TestAccessLevelJSONSerializeAnonymousAndIsTestDirectVMNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, shape := range []struct{ name, expression string }{
				{"directUser", "AccessLevel.USER_MODE"},
				{"directSystem", "AccessLevel.SYSTEM_MODE"},
				{"nestedUser", "new List<AccessLevel>{AccessLevel.USER_MODE}"},
				{"nestedSystem", "new List<AccessLevel>{AccessLevel.SYSTEM_MODE}"},
			} {
				t.Run(api+"/"+route+"/"+shape.name, func(t *testing.T) {
					machine := New(nil)
					if route == "isTest" {
						machine.EnableTestContext()
					}
					source := "String observed;try {JSON.serialize(" + shape.expression + ");} catch(Exception issue) {observed=issue.getTypeName();}System.assertEquals('System.JSONException',observed);"
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
