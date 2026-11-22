package vm

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func objectScalarChain(getter string) ir.Expr {
	return ir.Expr{Kind: ir.ExprCall, Callee: getter, Left: &ir.Expr{Kind: ir.ExprCall, Callee: "objectType.getDescribe"}}
}

func TestSObjectDescribeScalarChainMatchesEager(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		machine := describeShareUnprimedFor(namespaced)
		machine.VarTypes["objectType"] = "Schema.SObjectType"
		for _, object := range []string{"Account", "Widget__c", "Missing__c"} {
			machine.Globals["objectType"] = sObjectTypeToken(object)
			for _, getter := range []string{"getName", "getLabel", "getLabelPlural", "getKeyPrefix", "isCustom", "isCustomSetting", "isAccessible", "isCreateable", "isUpdateable", "isDeletable", "isQueryable", "isSearchable", "getRecordTypeInfos"} {
				chain := objectScalarChain(getter)
				eagerTrace, chainTrace := &Result{traceEnabled: true}, &Result{traceEnabled: true}
				describe, eagerErr := machine.eval(*chain.Left, eagerTrace)
				want := Null
				if eagerErr == nil {
					want, _, eagerErr = machine.callValueMember("", describe, getter, nil, eagerTrace)
				}
				got, gotErr := machine.eval(chain, chainTrace)
				// Collection getters allocate separate outer references; compare their contents.
				if getter == "getRecordTypeInfos" {
					want.Ref, got.Ref = 0, 0
				}
				if !reflect.DeepEqual(want, got) || fmt.Sprint(eagerErr) != fmt.Sprint(gotErr) || !reflect.DeepEqual(eagerTrace.Trace, chainTrace.Trace) {
					t.Fatalf("namespace=%v %s.%s: eager=%v/%v chain=%v/%v", namespaced, object, getter, want, eagerErr, got, gotErr)
				}
			}
		}
	}
}

func BenchmarkSObjectDescribeScalarChain64Fields(b *testing.B) {
	machine := New(nil)
	org := describeOverlayOrg(64)
	org.Namespace = ""
	object := org.Objects["pkg__Widget__c"]
	object.Definition.APIName = "Widget__c"
	org.Objects["Widget__c"] = object
	machine.SetOrg(&org)
	machine.VarTypes["objectType"] = "Schema.SObjectType"
	machine.Globals["objectType"] = sObjectTypeToken("Widget__c")
	chain := objectScalarChain("getLabel")
	result := &Result{}
	if _, err := machine.eval(chain, result); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, err := machine.eval(chain, result)
		if err != nil || value.Kind != ValueString {
			b.Fatalf("%v %v", value, err)
		}
	}
}

func TestSObjectDescribeScalarChainFallbackAndAliases(t *testing.T) {
	machine := describeShareUnprimed()
	machine.Globals["objectType"] = sObjectTypeToken("Account")
	machine.VarTypes["objectType"] = "Schema.SObjectType"
	for _, getter := range []string{"getRecordTypeInfos", "getFields", "getSObjectType"} {
		_, handled, err := machine.evalSObjectDescribeScalarChain(&[]ir.Expr{objectScalarChain(getter)}[0], &Result{})
		if handled || err != nil {
			t.Fatalf("%s unexpectedly optimized: %v", getter, err)
		}
	}
	chain := objectScalarChain("getName")
	chain.Left.Args = []ir.Expr{{Kind: ir.ExprLiteral, Value: "DEFERRED"}}
	if _, handled, _ := machine.evalSObjectDescribeScalarChain(&chain, &Result{}); handled {
		t.Fatal("option argument optimized")
	}

	program, err := CompileAnonymous(`
Schema.SObjectType objectType = Account.SObjectType;
Schema.DescribeSObjectResult saved = objectType.getDescribe();
Schema.DescribeSObjectResult alias = saved;
String before = JSON.serialize(saved);
System.assertEquals(before, JSON.serialize(alias));
saved.name = 'Changed';
System.assertEquals('Changed', alias.getName());
System.assertEquals('Account', objectType.getDescribe().getName());
System.assertEquals('Account', objectType.getDescribe().getName());
System.assertEquals(JSON.serialize(saved), JSON.serialize(alias));
System.assert(before != JSON.serialize(saved));
List<Schema.RecordTypeInfo> records = objectType.getDescribe().getRecordTypeInfos();
Integer count = records.size();
records.clear();
System.assertEquals(count, objectType.getDescribe().getRecordTypeInfos().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestSObjectDescribeScalarChainReferenceAndErrors(t *testing.T) {
	machine := describeShareUnprimed()
	machine.VarTypes["objectType"] = "Schema.SObjectType"
	machine.Globals["objectType"] = sObjectTypeToken("Account")
	chain := objectScalarChain("getName")
	if _, handled, err := machine.evalSObjectDescribeScalarChain(&chain, &Result{}); !handled || err != nil {
		t.Fatalf("scalar chain not handled: %v", err)
	}
	before := nextValueRef.Load()
	if _, err := machine.eval(*chain.Left, &Result{}); err != nil {
		t.Fatal(err)
	}
	eager := nextValueRef.Load() - before
	before = nextValueRef.Load()
	if _, err := machine.eval(chain, &Result{}); err != nil {
		t.Fatal(err)
	}
	if got := nextValueRef.Load() - before; got != eager {
		t.Fatalf("reference allocations: eager %d, chain %d", eager, got)
	}
	for _, invalid := range []Value{Null, Int(1)} {
		token := sObjectTypeToken("Account")
		token.Fields["object"] = invalid
		machine.Globals["objectType"] = token
		_, want := machine.eval(*chain.Left, &Result{})
		_, got := machine.eval(chain, &Result{})
		if want == nil || fmt.Sprint(want) != fmt.Sprint(got) {
			t.Fatalf("malformed token: eager %v, chain %v", want, got)
		}
	}
}

func TestSObjectDescribeScalarChainRegisteredGetterKeepsDispatch(t *testing.T) {
	machine := describeShareUnprimed()
	machine.Globals["objectType"] = sObjectTypeToken("Account")
	machine.VarTypes["objectType"] = "Schema.SObjectType"
	program, err := CompileAnonymous("return 'override';")
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Schema.DescribeSObjectResult", Methods: map[string]Method{
		"getName": {Name: "Schema.DescribeSObjectResult.getName", ClassName: "Schema.DescribeSObjectResult", Access: "public", ReturnType: "String", Program: program},
	}}); err != nil {
		t.Fatal(err)
	}
	value, err := machine.eval(objectScalarChain("getName"), &Result{})
	if err != nil || value.Text != "override" {
		t.Fatalf("registered getter: %v %v", value, err)
	}
}

// Each transition isolates one describe-key input after the local-token path
// is warm. The eager result is rebuilt so a stale warm root cannot validate itself.
func TestSObjectDescribeScalarChainWarmContextKeys(t *testing.T) {
	tests := []struct {
		name, object, field string
		setup, change       func(*VM)
	}{
		{"profile", "Widget__c", "ProfileID", nil, func(m *VM) { m.executionUser.Fields["ProfileId"] = String("00e000000000002") }},
		{"user", "Widget__c", "UserID", nil, func(m *VM) { m.executionUser.Fields["Id"] = String("005000000000002") }},
		{"permission sets", "Widget__c", "PermissionSets", func(m *VM) {
			permissionIdentityPut(m, "PermissionSet", "0ps000000000001", nil)
			permissionIdentityPut(m, "PermissionSetAssignment", "0pa000000000001", map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000002"), "PermissionSetId": storage.IDValue("0ps000000000001")})
		}, func(m *VM) {
			permissionIdentityPut(m, "PermissionSetAssignment", "0pa000000000001", map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000001"), "PermissionSetId": storage.IDValue("0ps000000000001")})
		}},
		{"task order", "Task", "TaskFieldOrder", nil, func(m *VM) { m.currentMethod.APIVersion = "65.0" }},
		{"permission field omission", "ObjectPermissions", "OmitPermissionViewAllFields", nil, func(m *VM) { m.currentMethod.APIVersion = "62.0" }},
		{"feature API", "DocumentGenerationProcess", "FeatureAPIVersion", nil, func(m *VM) { m.currentMethod.APIVersion = "66.0" }},
		{"feature named source", "DocumentGenerationProcess", "FeatureNamedSource", nil, func(m *VM) { m.currentClass = "Reader" }},
		{"API 68 shape", "Account", "API68CapturedShape", func(m *VM) {
			storage.ApplyProjectOrgShape(m.Org, []string{project.StandardFieldEditionDeveloper})
			m.currentMethod.APIVersion = "67.0"
		}, func(m *VM) { m.currentMethod.APIVersion = "68.0" }},
		{"API 68 named route", "Account", "API68NamedRoute", func(m *VM) {
			storage.ApplyProjectOrgShape(m.Org, []string{project.StandardFieldEditionDeveloper})
			m.currentMethod.APIVersion = "68.0"
		}, func(m *VM) { m.currentClass = "Reader" }},
		{"anonymous policy", "Widget__c", "AnonymousFieldPolicy", func(m *VM) { m.currentMethod.APIVersion = "62.0"; m.currentClass = "Reader" }, func(m *VM) { m.currentClass = "" }},
		{"plain projection", "Account", "PlainDeveloperProjection", func(m *VM) {
			m.Org.Features = []string{project.PlainDeveloperDescribeShape}
			m.currentMethod.APIVersion = "62.0"
			m.executionUser.Fields["LocaleSidKey"] = String("fr_FR")
		}, func(m *VM) { m.executionUser.Fields["LocaleSidKey"] = String("en_US") }},
		{"plain named", "Account", "PlainDeveloperNamed", func(m *VM) {
			m.Org.Features = []string{project.PlainDeveloperDescribeShape}
			m.currentMethod.APIVersion = "62.0"
		}, func(m *VM) { m.currentClass = "Reader" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			org := describeShareOrg()
			storage.EnsureStandardObject(&org, tc.object)
			machine := New(nil)
			machine.SetOrg(&org)
			machine.executionUser = Object("User")
			machine.executionUser.Fields["Id"] = String("005000000000001")
			machine.executionUser.Fields["ProfileId"] = String("00e000000000001")
			profile := Object("Profile")
			profile.Fields["Name"] = String("System Administrator")
			machine.executionUser.Fields["Profile"] = profile
			machine.currentMethod = Method{APIVersion: "64.0", SourceContextBound: true}
			if err := machine.RegisterClass(Class{Name: "Reader"}); err != nil {
				t.Fatal(err)
			}
			if tc.setup != nil {
				tc.setup(machine)
			}
			machine.Globals["objectType"] = sObjectTypeToken(tc.object)
			machine.VarTypes["objectType"] = "Schema.SObjectType"
			check := func() {
				t.Helper()
				for _, getter := range []string{"getName", "getLabel", "isAccessible"} {
					chain := objectScalarChain(getter)
					if _, handled, err := machine.evalSObjectDescribeScalarChain(&chain, &Result{}); !handled || err != nil {
						t.Fatalf("%s not borrowed: %v", getter, err)
					}
					chainTrace, eagerTrace := &Result{traceEnabled: true}, &Result{traceEnabled: true}
					got, gotErr := machine.eval(chain, chainTrace)
					warmCache := machine.describeCache
					machine.describeCache = make(map[describeCacheKey]Value)
					desc, wantErr := machine.eval(*chain.Left, eagerTrace)
					want := Null
					if wantErr == nil {
						want, _, wantErr = machine.callValueMember("", desc, getter, nil, eagerTrace)
					}
					machine.describeCache = warmCache
					if !reflect.DeepEqual(got, want) || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) || !reflect.DeepEqual(chainTrace.Trace, eagerTrace.Trace) {
						t.Fatalf("%s: chain=%v/%v eager=%v/%v", getter, got, gotErr, want, wantErr)
					}
				}
			}
			check()
			before := machine.describeSObjectCacheKey(tc.object)
			if _, ok := machine.describeCache[before]; !ok {
				t.Fatal("initial context was not warmed")
			}
			tc.change(machine)
			after := machine.describeSObjectCacheKey(tc.object)
			bv, av := reflect.ValueOf(before).FieldByName(tc.field), reflect.ValueOf(after).FieldByName(tc.field)
			if reflect.DeepEqual(bv.Interface(), av.Interface()) {
				t.Fatalf("%s did not change: %+v", tc.field, before)
			}
			changedKey := after
			// No unrelated key component may change in an isolated transition.
			reflect.ValueOf(&after).Elem().FieldByName(tc.field).Set(bv)
			if after != before {
				t.Fatalf("%s transition also changed another key: %+v != %+v", tc.field, after, before)
			}
			check()
			if _, ok := machine.describeCache[before]; !ok {
				t.Fatal("initial warm context was discarded")
			}
			if _, ok := machine.describeCache[changedKey]; !ok {
				t.Fatal("changed context was not warmed")
			}
			// Return to the first context while both roots have been warmed.
			switch tc.name {
			case "profile":
				machine.executionUser.Fields["ProfileId"] = String("00e000000000001")
			case "user":
				machine.executionUser.Fields["Id"] = String("005000000000001")
			case "permission sets":
				delete(machine.Org.Objects["PermissionSetAssignment"].Records, "0pa000000000001")
			case "task order", "permission field omission", "feature API":
				machine.currentMethod.APIVersion = "64.0"
			case "feature named source", "API 68 named route", "plain named":
				machine.currentClass = ""
			case "API 68 shape":
				machine.currentMethod.APIVersion = "67.0"
			case "anonymous policy":
				machine.currentClass = "Reader"
			case "plain projection":
				machine.executionUser.Fields["LocaleSidKey"] = String("fr_FR")
			}
			if got := machine.describeSObjectCacheKey(tc.object); got != before {
				t.Fatalf("context did not restore key: %+v != %+v", got, before)
			}
			check()
		})
	}
}

func TestSObjectDescribeScalarChainWarmConcurrentSiblingClones(t *testing.T) {
	base, template := describeSharePrimedBase(t)
	clones := []*VM{describeShareClone(base, template), describeShareClone(base, template)}
	chain := objectScalarChain("getLabel")
	for _, machine := range clones {
		machine.Globals["objectType"] = sObjectTypeToken("Widget__c")
		machine.VarTypes["objectType"] = "Schema.SObjectType"
		value, handled, err := machine.evalSObjectDescribeScalarChain(&chain, &Result{})
		if !handled || err != nil || value.Text != "Widget" {
			t.Fatalf("warm clone: %v handled=%v err=%v", value, handled, err)
		}
	}
	firstMap := describeShareFieldMapOf(t, clones[0], "Widget__c")
	secondMap := describeShareFieldMapOf(t, clones[1], "Widget__c")
	if mapIdentity(firstMap) != mapIdentity(secondMap) {
		t.Fatal("fixture did not warm shared schema parts")
	}
	definition, ok := storage.EnsureMutableObjectDefinition(clones[0].Org, "Widget__c")
	if !ok {
		t.Fatal("missing fixture definition")
	}
	definition.Label = "Changed"
	state := clones[0].Org.Objects["Widget__c"]
	state.Definition = *definition
	clones[0].Org.Objects["Widget__c"] = state
	results := make(chan error, len(clones))
	for i, machine := range clones {
		want := []string{"Changed", "Widget"}[i]
		go func() {
			for n := 0; n < 10; n++ {
				got, err := machine.eval(chain, &Result{})
				if err != nil || got.Kind != ValueString || got.Text != want {
					results <- fmt.Errorf("got %v/%v, want %s", got, err, want)
					return
				}
			}
			results <- nil
		}()
	}
	for range clones {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if mapIdentity(describeShareFieldMapOf(t, clones[0], "Widget__c")) == mapIdentity(secondMap) {
		t.Fatal("changed generation retained the shared field map")
	}
}

func TestSObjectDescribeScalarChainWarmPermissionOutcomes(t *testing.T) {
	machine := describeShareUnprimed()
	machine.testContext.CurrentUser = Object("User")
	user := machine.testContext.CurrentUser
	user.Fields["Id"] = String("005000000000001")
	user.Fields["ProfileId"] = String("00e000000000001")
	for _, row := range []struct {
		id, parent string
		allowed    bool
	}{
		{"110000000000001", "00e000000000001", false},
		{"110000000000002", "00e000000000002", true},
		{"110000000000003", "0ps000000000001", true},
	} {
		permissionIdentityPut(machine, "ObjectPermissions", row.id, map[string]storage.Value{
			"ParentId": storage.IDValue(storage.ID(row.parent)), "SObjectType": storage.StringValue("Widget__c"), "PermissionsRead": storage.BooleanValue(row.allowed),
		})
	}
	permissionIdentityPut(machine, "PermissionSet", "0ps000000000001", nil)
	permissionIdentityPut(machine, "PermissionSetAssignment", "0pa000000000001", map[string]storage.Value{
		"AssigneeId": storage.IDValue("005000000000002"), "PermissionSetId": storage.IDValue("0ps000000000001"),
	})
	machine.Globals["objectType"] = sObjectTypeToken("Widget__c")
	machine.VarTypes["objectType"] = "Schema.SObjectType"
	chain := objectScalarChain("isAccessible")
	for _, row := range []struct {
		user, profile string
		allowed       bool
	}{
		{"005000000000001", "00e000000000001", false},
		{"005000000000001", "00e000000000002", true},
		{"005000000000001", "00e000000000001", false},
		{"005000000000002", "00e000000000001", true},
		{"005000000000001", "00e000000000001", false},
	} {
		user.Fields["Id"], user.Fields["ProfileId"] = String(row.user), String(row.profile)
		got, handled, err := machine.evalSObjectDescribeScalarChain(&chain, &Result{})
		if !handled || err != nil || got.Kind != ValueBool || got.Bool != row.allowed {
			t.Fatalf("user=%s profile=%s: got %v handled=%v err=%v, want %v", row.user, row.profile, got, handled, err, row.allowed)
		}
	}
	assignment := machine.Org.Objects["PermissionSetAssignment"].Records["0pa000000000001"]
	assignment.Fields["AssigneeId"] = storage.IDValue("005000000000001")
	got, handled, err := machine.evalSObjectDescribeScalarChain(&chain, &Result{})
	if !handled || err != nil || got.Kind != ValueBool || !got.Bool {
		t.Fatalf("same-count reassignment: got %v handled=%v err=%v", got, handled, err)
	}
}

func TestSObjectDescribeScalarChainWarmTargetsAndCallers(t *testing.T) {
	for _, namespaced := range []bool{false, true} {
		machine := describeShareUnprimedFor(namespaced)
		machine.VarTypes["objectType"] = "Schema.SObjectType"
		machine.currentMethod = Method{SourceContextBound: true}
		objects := []string{"Account", "Widget__c", "Account"}
		callers := []string{"", "", ""}
		if namespaced {
			objects = []string{"zzns__Gadget__c", "zzns__Gadget__c", "zzns__Gadget__c"}
			callers = []string{"", "zzns", ""}
		}
		for i, object := range objects {
			machine.Globals["objectType"] = sObjectTypeToken(object)
			machine.currentMethod.Namespace = callers[i]
			chain := objectScalarChain("getName")
			chainTrace, eagerTrace := &Result{traceEnabled: true}, &Result{traceEnabled: true}
			got, gotErr := machine.eval(chain, chainTrace)
			desc, wantErr := machine.eval(*chain.Left, eagerTrace)
			want := Null
			if wantErr == nil {
				want, _, wantErr = machine.callValueMember("", desc, "getName", nil, eagerTrace)
			}
			if !reflect.DeepEqual(got, want) || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) || !reflect.DeepEqual(chainTrace.Trace, eagerTrace.Trace) {
				t.Fatalf("namespaced=%v step=%d: chain=%v/%v eager=%v/%v", namespaced, i, got, gotErr, want, wantErr)
			}
			if !namespaced && (got.Kind != ValueString || got.Text != object) {
				t.Fatalf("target %s: got %v", object, got)
			}
		}
	}
}
