package vm

import (
	"reflect"
	"testing"
)

type task31MethodReturnFixture struct {
	scope           map[string]Value
	receiverRef     uint64
	parameterRef    uint64
	receiverMapKey  string
	parameterMapKey string
	mapValueOrder   []string
	mapKeyOrder     []string
	receiverSource  Value
	parameterSource Value
}

type task31PreparedMethodCall struct {
	machine   *VM
	method    Method
	receiver  Value
	parameter Value
	fixture   task31MethodReturnFixture
}

type task31ScopeAliasTraversalObserver struct {
	rootVisits uint64
	fallbacks  uint64
}

func (observer *task31ScopeAliasTraversalObserver) scopeAliasRootVisited() {
	observer.rootVisits++
}

func (observer *task31ScopeAliasTraversalObserver) scopeAliasBatchFallback(string) {
	observer.fallbacks++
}

func TestTask31TwoTargetMethodReturnPreservesAliasShapes(t *testing.T) {
	method := newTask31MutatingMethod(t)
	prepared := prepareTask31MutatingMethod(t, method, true, false)
	defer ResetPerfCounters()
	callTask31MutatingMethod(t, prepared)
	assertTask31MethodReturnFixture(t, prepared)
}

func TestTask31TwoTargetMethodReturnVisitsEachCallerRootOnce(t *testing.T) {
	method := newTask31MutatingMethod(t)
	prepared := prepareTask31MutatingMethod(t, method, true, false)
	defer ResetPerfCounters()
	observer := &task31ScopeAliasTraversalObserver{}
	prepared.machine.scopeAliasTraversalObserver = observer

	callTask31MutatingMethod(t, prepared)

	if want := uint64(len(prepared.fixture.scope)); observer.rootVisits != want {
		t.Fatalf("method-return caller roots actually visited = %d, want %d (each root once)", observer.rootVisits, want)
	}
	assertTask31MethodReturnFixture(t, prepared)
}

func TestTask31TwoTargetMethodReturnPerfCountsEachCallerRootOnceWithoutObserver(t *testing.T) {
	t.Cleanup(ResetPerfCounters)
	method := newTask31MutatingMethod(t)
	prepared := prepareTask31MutatingMethod(t, method, true, true)
	if prepared.machine.scopeAliasTraversalObserver != nil {
		t.Fatal("performance-counter acceptance unexpectedly installed a traversal observer")
	}

	callTask31MutatingMethod(t, prepared)

	stats := task31MutatingMethodStats(prepared)
	if want := uint64(len(prepared.fixture.scope)); stats.Roots != want {
		t.Fatalf("method-return performance counter recorded %d caller roots, want %d (each root once)", stats.Roots, want)
	}
	assertTask31MethodReturnFixture(t, prepared)
}

func TestTask31MethodReturnBatchSummaryDoesNotLeakAcrossCalls(t *testing.T) {
	method := newTask31MutatingMethod(t)
	first := prepareTask31MutatingMethod(t, method, true, false)
	defer ResetPerfCounters()
	observer := &task31ScopeAliasTraversalObserver{}
	first.machine.scopeAliasTraversalObserver = observer
	callTask31MutatingMethod(t, first)
	assertTask31MethodReturnFixture(t, first)
	firstAfter := cloneTask31Scope(first.fixture.scope)

	second := newTask31PreparedMethodCall(first.machine, method, true)
	callTask31MutatingMethod(t, second)

	assertTask31MethodReturnFixture(t, second)
	if !reflect.DeepEqual(first.fixture.scope, firstAfter) {
		t.Fatalf("first caller scope changed during disjoint second return:\nfirst=%#v\nafter-first=%#v", first.fixture.scope, firstAfter)
	}
	if first.fixture.receiverRef == second.fixture.receiverRef || first.fixture.parameterRef == second.fixture.parameterRef {
		t.Fatalf("disjoint returns reused target refs: first=%d/%d second=%d/%d", first.fixture.receiverRef, first.fixture.parameterRef, second.fixture.receiverRef, second.fixture.parameterRef)
	}
	if want := uint64(2 * len(first.fixture.scope)); observer.rootVisits != want {
		t.Fatalf("two disjoint returns visited %d caller roots, want %d", observer.rootVisits, want)
	}
}

func TestTask31MethodReturnBatchFallsBackForUnsafeShapes(t *testing.T) {
	t.Run("cycle", func(t *testing.T) {
		method := newTask31MutatingMethod(t)
		prepared := prepareTask31MutatingMethod(t, method, false, false)
		t.Cleanup(ResetPerfCounters)
		observer := &task31ScopeAliasTraversalObserver{}
		prepared.machine.scopeAliasTraversalObserver = observer

		root := Object("pkg.CyclicEnvelope")
		root.Fields["Receiver"] = prepared.receiver
		root.Fields["Parameter"] = prepared.parameter
		root.Fields["Control"] = String("cycle-control")
		root.Fields["Self"] = root
		prepared.machine.Globals = map[string]Value{"root": root}
		callTask31MutatingMethod(t, prepared)

		if observer.fallbacks == 0 {
			t.Fatal("cyclic caller graph did not take the fail-open path")
		}
		gotRoot := prepared.machine.Globals["root"]
		assertTask31Alias(t, gotRoot.Fields["Receiver"], prepared.receiver.Ref, "pkg.AliasReceiver", "receiver-updated")
		assertTask31Alias(t, gotRoot.Fields["Parameter"], prepared.parameter.Ref, "pkg.AliasParameter", "parameter-updated")
		assertTask31Control(t, gotRoot.Fields["Control"], "cycle-control")
		self := gotRoot.Fields["Self"]
		if self.Ref != gotRoot.Ref || !sameMapBacking(self.Fields, gotRoot.Fields) {
			t.Fatalf("fallback broke caller cycle: root=%#v self=%#v", gotRoot, self)
		}
	})

	t.Run("unknown caller shape", func(t *testing.T) {
		method := newTask31MutatingMethod(t)
		prepared := prepareTask31MutatingMethod(t, method, false, false)
		t.Cleanup(ResetPerfCounters)
		observer := &task31ScopeAliasTraversalObserver{}
		prepared.machine.scopeAliasTraversalObserver = observer

		unknown := Value{
			Kind: ValueKind("task31-unknown"),
			Ref:  newValueRef(),
			Fields: map[string]Value{
				"Receiver":  prepared.receiver,
				"Parameter": prepared.parameter,
				"Control":   String("unknown-control"),
			},
		}
		prepared.machine.Globals = map[string]Value{"unknown": unknown}
		callTask31MutatingMethod(t, prepared)

		if observer.fallbacks == 0 {
			t.Fatal("unknown caller shape did not take the fail-open path")
		}
		got := prepared.machine.Globals["unknown"]
		if got.Kind != unknown.Kind || got.Ref != unknown.Ref {
			t.Fatalf("fallback changed unknown caller identity: got=%#v want=%#v", got, unknown)
		}
		if got.Fields["Receiver"].Fields["Name"].Kind != "" || got.Fields["Parameter"].Fields["Name"].Kind != "" {
			t.Fatalf("fallback traversed an unknown caller shape: %#v", got)
		}
		assertTask31Control(t, got.Fields["Control"], "unknown-control")
	})

	t.Run("incomplete provenance", func(t *testing.T) {
		method := newTask31MutatingMethod(t)
		prepared := prepareTask31MutatingMethod(t, method, false, false)
		t.Cleanup(ResetPerfCounters)
		observer := &task31ScopeAliasTraversalObserver{}
		prepared.machine.scopeAliasTraversalObserver = observer

		prepared.receiver.Ref = 0
		root := Object("pkg.IncompleteEnvelope")
		root.Fields["Receiver"] = prepared.receiver
		root.Fields["Parameter"] = prepared.parameter
		root.Fields["Control"] = String("incomplete-control")
		prepared.machine.Globals = map[string]Value{"root": root}
		before := cloneTask31Scope(prepared.machine.Globals)
		updatedReceiver := prepared.receiver
		updatedReceiver.Fields = map[string]Value{"Name": String("receiver-updated")}
		updatedParameter := prepared.parameter
		updatedParameter.Fields = map[string]Value{"Name": String("parameter-updated")}
		if prepared.machine.tryPropagateMethodReturnAliasSnapshotMutationsToScope(
			prepared.machine.Globals,
			[]methodReturnAliasMutation{
				{
					previous: snapshotAlias(prepared.receiver),
					original: prepared.receiver,
					updated:  updatedReceiver,
				},
				{
					previous: snapshotAlias(prepared.parameter),
					original: prepared.parameter,
					updated:  updatedParameter,
				},
			},
		) {
			t.Fatal("batch accepted incomplete receiver provenance")
		}
		if observer.fallbacks == 0 {
			t.Fatal("incomplete receiver provenance did not make batch eligibility fail open")
		}
		if !reflect.DeepEqual(prepared.machine.Globals, before) {
			t.Fatalf("incomplete provenance eligibility changed caller:\ngot=%#v\nwant=%#v", prepared.machine.Globals, before)
		}
		observer.fallbacks = 0
		callTask31MutatingMethod(t, prepared)

		if observer.fallbacks != 0 {
			t.Fatal("method return selected a ref-less receiver mutation instead of preserving the legacy valid-snapshot gate")
		}
		gotRoot := prepared.machine.Globals["root"]
		if got := gotRoot.Fields["Receiver"].Fields["Name"]; got.Kind != "" {
			t.Fatalf("ref-less receiver unexpectedly propagated: %#v", gotRoot.Fields["Receiver"])
		}
		assertTask31Alias(t, gotRoot.Fields["Parameter"], prepared.parameter.Ref, "pkg.AliasParameter", "parameter-updated")
		assertTask31Control(t, gotRoot.Fields["Control"], "incomplete-control")
	})
}

func assertTask31MethodReturnFixture(tb testing.TB, prepared task31PreparedMethodCall) {
	tb.Helper()
	fixture := prepared.fixture

	objectRoot := fixture.scope["object"]
	if len(objectRoot.Fields) != 3 {
		tb.Fatalf("object fields = %#v, want exactly receiver, parameter, and control", objectRoot.Fields)
	}
	assertTask31Alias(tb, objectRoot.Fields["Receiver"], fixture.receiverRef, "pkg.AliasReceiver", "receiver-updated")
	assertTask31Alias(tb, objectRoot.Fields["Parameter"], fixture.parameterRef, "pkg.AliasParameter", "parameter-updated")
	assertTask31Control(tb, objectRoot.Fields["Control"], "object-control")

	listRoot := fixture.scope["list"]
	if len(listRoot.List) != 3 {
		tb.Fatalf("list root = %#v, want receiver, parameter, and control", listRoot)
	}
	assertTask31Alias(tb, listRoot.List[0], fixture.receiverRef, "pkg.AliasReceiver", "receiver-updated")
	assertTask31Alias(tb, listRoot.List[1], fixture.parameterRef, "pkg.AliasParameter", "parameter-updated")
	assertTask31Control(tb, listRoot.List[2], "list-control")

	setRoot := fixture.scope["set"]
	if len(setRoot.Set) != 3 {
		tb.Fatalf("set root = %#v, want exactly receiver, parameter, and control", setRoot)
	}
	assertTask31AliasByRef(tb, setRoot.Set, fixture.receiverRef, "pkg.AliasReceiver", "receiver-updated")
	assertTask31AliasByRef(tb, setRoot.Set, fixture.parameterRef, "pkg.AliasParameter", "parameter-updated")
	assertTask31ControlInSet(tb, setRoot.Set, "set-control")

	mapValues := fixture.scope["mapValues"]
	if len(mapValues.Map) != 3 || len(mapValues.MapKeys) != 3 {
		tb.Fatalf("map-value entries = values:%#v keys:%#v, want exactly three", mapValues.Map, mapValues.MapKeys)
	}
	assertTask31Alias(tb, mapValues.Map[mapKey(String("receiver"))], fixture.receiverRef, "pkg.AliasReceiver", "receiver-updated")
	assertTask31Alias(tb, mapValues.Map[mapKey(String("parameter"))], fixture.parameterRef, "pkg.AliasParameter", "parameter-updated")
	assertTask31Control(tb, mapValues.Map[mapKey(String("control"))], "map-value-control")
	if !reflect.DeepEqual(mapValues.MapOrder, fixture.mapValueOrder) {
		tb.Fatalf("map-value order = %#v, want %#v", mapValues.MapOrder, fixture.mapValueOrder)
	}

	mapKeys := fixture.scope["mapKeys"]
	if len(mapKeys.Map) != 3 || len(mapKeys.MapKeys) != 3 {
		tb.Fatalf("map-key entries = values:%#v keys:%#v, want exactly three", mapKeys.Map, mapKeys.MapKeys)
	}
	assertTask31Alias(tb, mapKeys.MapKeys[fixture.receiverMapKey], fixture.receiverRef, "pkg.AliasReceiver", "receiver-updated")
	assertTask31Alias(tb, mapKeys.MapKeys[fixture.parameterMapKey], fixture.parameterRef, "pkg.AliasParameter", "parameter-updated")
	if mapKeys.Map[fixture.receiverMapKey].Text != "receiver-value" || mapKeys.Map[fixture.parameterMapKey].Text != "parameter-value" {
		tb.Fatalf("map values changed while propagating key aliases: %#v", mapKeys.Map)
	}
	controlMapKey := mapKey(String("control-key"))
	assertTask31Control(tb, mapKeys.Map[controlMapKey], "map-key-control")
	assertTask31Control(tb, mapKeys.MapKeys[controlMapKey], "control-key")
	if !reflect.DeepEqual(mapKeys.MapOrder, fixture.mapKeyOrder) {
		tb.Fatalf("map-key order = %#v, want %#v", mapKeys.MapOrder, fixture.mapKeyOrder)
	}

	sharedLeft := fixture.scope["sharedLeft"]
	sharedRight := fixture.scope["sharedRight"]
	if len(sharedLeft.Fields) != 3 || len(sharedRight.Fields) != 3 {
		tb.Fatalf("shared fields changed shape: left=%#v right=%#v", sharedLeft.Fields, sharedRight.Fields)
	}
	if sharedLeft.Ref == 0 || sharedLeft.Ref != sharedRight.Ref || !sameMapBacking(sharedLeft.Fields, sharedRight.Fields) {
		tb.Fatalf("shared roots lost backing identity: left=%#v right=%#v", sharedLeft, sharedRight)
	}
	assertTask31Alias(tb, sharedLeft.Fields["Receiver"], fixture.receiverRef, "pkg.AliasReceiver", "receiver-updated")
	assertTask31Alias(tb, sharedRight.Fields["Parameter"], fixture.parameterRef, "pkg.AliasParameter", "parameter-updated")
	assertTask31Control(tb, sharedLeft.Fields["Control"], "shared-control")

	if !reflect.DeepEqual(prepared.receiver, fixture.receiverSource) || !reflect.DeepEqual(prepared.parameter, fixture.parameterSource) {
		tb.Fatalf("method return mutated source values: receiver=%#v parameter=%#v", prepared.receiver, prepared.parameter)
	}
}

func newTask31MutatingMethod(tb testing.TB) Method {
	tb.Helper()
	program, err := CompileAnonymous(`
this.Name = 'receiver-updated';
parameter.Name = 'parameter-updated';
`)
	if err != nil {
		tb.Fatal(err)
	}
	return Method{
		Name:       "pkg.AliasReceiver.mutate",
		ClassName:  "pkg.AliasReceiver",
		ReturnType: "void",
		Params:     []Param{{Name: "parameter", Type: "pkg.AliasParameter"}},
		Program:    program,
	}
}

func prepareTask31MutatingMethod(tb testing.TB, method Method, withCallerAliases, withPerfRecorder bool) task31PreparedMethodCall {
	tb.Helper()
	ResetPerfCounters()
	if withPerfRecorder {
		SetPerfCountersEnabled(true)
	}
	machine := New(nil)
	if !withPerfRecorder && machine.perfRecorder != nil {
		tb.Fatal("default Task31 path unexpectedly captured a performance recorder")
	}
	if err := machine.RegisterClass(Class{
		Name:       "AliasParameter",
		Namespace:  "pkg",
		Dependency: true,
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String", Access: "public"},
		},
	}); err != nil {
		tb.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "AliasReceiver",
		Namespace:  "pkg",
		Dependency: true,
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String", Access: "public"},
		},
	}); err != nil {
		tb.Fatal(err)
	}

	return newTask31PreparedMethodCall(machine, method, withCallerAliases)
}

func newTask31PreparedMethodCall(machine *VM, method Method, withCallerAliases bool) task31PreparedMethodCall {
	receiver := Value{
		Kind:    ValueObject,
		Type:    "pkg.AliasReceiver",
		Static:  "pkg.AliasReceiver",
		Runtime: "pkg.AliasReceiver",
		Ref:     newValueRef(),
	}
	parameter := Value{
		Kind:    ValueObject,
		Type:    "pkg.AliasParameter",
		Static:  "pkg.AliasParameter",
		Runtime: "pkg.AliasParameter",
		Ref:     newValueRef(),
	}
	fixture := task31MethodReturnFixture{
		scope:           map[string]Value{"primitive": String("control")},
		receiverRef:     receiver.Ref,
		parameterRef:    parameter.Ref,
		receiverSource:  cloneValuePreserveRefs(receiver),
		parameterSource: cloneValuePreserveRefs(parameter),
	}
	if withCallerAliases {
		fixture = newTask31CallerScope(receiver, parameter)
	}
	machine.Globals = fixture.scope
	return task31PreparedMethodCall{
		machine:   machine,
		method:    method,
		receiver:  receiver,
		parameter: parameter,
		fixture:   fixture,
	}
}

func callTask31MutatingMethod(tb testing.TB, prepared task31PreparedMethodCall) {
	tb.Helper()
	if _, err := prepared.machine.callMethodWithReceiver(prepared.method, prepared.receiver, []Value{prepared.parameter}, &Result{}); err != nil {
		tb.Fatal(err)
	}
}

func task31MutatingMethodStats(prepared task31PreparedMethodCall) ScopeAliasPerfCounters {
	return prepared.machine.perfRecorder.Snapshot().ScopeAlias
}

func newTask31CallerScope(receiver, parameter Value) task31MethodReturnFixture {
	objectRoot := Object("pkg.AliasEnvelope")
	objectRoot.Fields["Receiver"] = receiver
	objectRoot.Fields["Parameter"] = parameter
	objectRoot.Fields["Control"] = String("object-control")

	mapValues := Map()
	mapValues.Type = "Map<String,Object>"
	receiverValueKey := mapKey(String("receiver"))
	parameterValueKey := mapKey(String("parameter"))
	mapValues.Map[receiverValueKey] = receiver
	mapValues.MapKeys[receiverValueKey] = String("receiver")
	mapValues.Map[parameterValueKey] = parameter
	mapValues.MapKeys[parameterValueKey] = String("parameter")
	controlValueKey := mapKey(String("control"))
	mapValues.Map[controlValueKey] = String("map-value-control")
	mapValues.MapKeys[controlValueKey] = String("control")
	mapValues.MapOrder = []string{receiverValueKey, parameterValueKey, controlValueKey}

	mapKeys := Map()
	mapKeys.Type = "Map<Object,String>"
	receiverMapKey := mapKey(receiver)
	parameterMapKey := mapKey(parameter)
	mapKeys.Map[receiverMapKey] = String("receiver-value")
	mapKeys.MapKeys[receiverMapKey] = receiver
	mapKeys.Map[parameterMapKey] = String("parameter-value")
	mapKeys.MapKeys[parameterMapKey] = parameter
	controlMapKey := mapKey(String("control-key"))
	mapKeys.Map[controlMapKey] = String("map-key-control")
	mapKeys.MapKeys[controlMapKey] = String("control-key")
	mapKeys.MapOrder = []string{receiverMapKey, parameterMapKey, controlMapKey}

	shared := Object("pkg.SharedAliasEnvelope")
	shared.Fields["Receiver"] = receiver
	shared.Fields["Parameter"] = parameter
	shared.Fields["Control"] = String("shared-control")

	return task31MethodReturnFixture{
		scope: map[string]Value{
			"object":      objectRoot,
			"list":        List(receiver, parameter, String("list-control")),
			"set":         Set(receiver, parameter, String("set-control")),
			"mapValues":   mapValues,
			"mapKeys":     mapKeys,
			"sharedLeft":  shared,
			"sharedRight": shared,
		},
		receiverRef:     receiver.Ref,
		parameterRef:    parameter.Ref,
		receiverMapKey:  receiverMapKey,
		parameterMapKey: parameterMapKey,
		mapValueOrder:   append([]string(nil), mapValues.MapOrder...),
		mapKeyOrder:     append([]string(nil), mapKeys.MapOrder...),
		receiverSource:  cloneValuePreserveRefs(receiver),
		parameterSource: cloneValuePreserveRefs(parameter),
	}
}

func cloneTask31Scope(scope map[string]Value) map[string]Value {
	cloned := make(map[string]Value, len(scope))
	for name, value := range scope {
		cloned[name] = cloneValuePreserveRefs(value)
	}
	return cloned
}

func assertTask31Alias(tb testing.TB, value Value, ref uint64, typeName, name string) {
	tb.Helper()
	if value.Ref != ref || value.Type != typeName || value.Static != typeName || value.Runtime != typeName {
		tb.Fatalf("alias identity = %#v, want ref=%d type/static/runtime=%q", value, ref, typeName)
	}
	if got := value.Fields["Name"]; got.Kind != ValueString || got.Text != name {
		tb.Fatalf("alias Name = %#v, want %q", got, name)
	}
}

func assertTask31AliasByRef(tb testing.TB, values []Value, ref uint64, typeName, name string) {
	tb.Helper()
	for _, value := range values {
		if value.Ref == ref {
			assertTask31Alias(tb, value, ref, typeName, name)
			return
		}
	}
	tb.Fatalf("alias ref %d missing from %#v", ref, values)
}

func assertTask31Control(tb testing.TB, value Value, text string) {
	tb.Helper()
	if value.Kind != ValueString || value.Text != text {
		tb.Fatalf("control = %#v, want %q", value, text)
	}
}

func assertTask31ControlInSet(tb testing.TB, values []Value, text string) {
	tb.Helper()
	for _, value := range values {
		if value.Kind == ValueString && value.Text == text {
			return
		}
	}
	tb.Fatalf("control %q missing from set %#v", text, values)
}
