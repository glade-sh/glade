package vm

import (
	"reflect"
	"testing"
)

func TestTask31MethodReturnBatchRejectsUpdatedTargetContainingAnotherOldTarget(t *testing.T) {
	oldReceiver := task31RegressionObject("pkg.AliasReceiver")
	oldParameter := task31RegressionObject("pkg.AliasParameter")

	updatedReceiver := oldReceiver
	updatedReceiver.Fields = map[string]Value{
		"Parameter": oldParameter,
		"Name":      String("receiver-updated"),
	}
	updatedParameter := oldParameter
	updatedParameter.Fields = map[string]Value{
		"Name": String("parameter-updated"),
	}

	state := methodReturnAliasBatchState{
		targets: map[methodReturnAliasTargetKey]int{
			{ref: oldReceiver.Ref, kind: oldReceiver.Kind}:   0,
			{ref: oldParameter.Ref, kind: oldParameter.Kind}: 1,
		},
		updates:  []Value{updatedReceiver, updatedParameter},
		visiting: make(map[methodReturnAliasTargetKey]bool),
		memo:     make(map[methodReturnAliasTargetKey]methodReturnAliasBatchMemo),
	}
	root := Object("pkg.AliasEnvelope")
	root.Fields["Receiver"] = oldReceiver
	root.Fields["Parameter"] = oldParameter

	scope := map[string]Value{"root": root}
	before := cloneTask31Scope(scope)
	machine := New(nil)
	observer := &task31ScopeAliasTraversalObserver{}
	machine.scopeAliasTraversalObserver = observer
	mutations := []methodReturnAliasMutation{
		{
			previous: snapshotAlias(oldReceiver),
			original: oldReceiver,
			updated:  updatedReceiver,
		},
		{
			previous: snapshotAlias(oldParameter),
			original: oldParameter,
			updated:  updatedParameter,
		},
	}

	if machine.tryPropagateMethodReturnAliasSnapshotMutationsToScope(scope, mutations) {
		t.Fatal("batch accepted an updated target that still contains another old mutation target")
	}
	if observer.fallbacks == 0 {
		t.Fatal("cross-target containment did not select the legacy fallback")
	}
	if !reflect.DeepEqual(scope, before) {
		t.Fatalf("failed batch changed caller before fallback:\ngot=%#v\nwant=%#v", scope, before)
	}

	referenceScope := cloneTask31Scope(before)
	reference := New(nil)
	applyTask31LegacyMethodReturnMutations(reference, referenceScope, cloneTask31RegressionMutations(mutations))

	applyTask31LegacyMethodReturnMutations(machine, scope, mutations)
	gotRoot := scope["root"]
	assertTask31Alias(t, gotRoot.Fields["Parameter"], oldParameter.Ref, "pkg.AliasParameter", "parameter-updated")
	gotReceiver := gotRoot.Fields["Receiver"]
	assertTask31Alias(t, gotReceiver, oldReceiver.Ref, "pkg.AliasReceiver", "receiver-updated")
	assertTask31Alias(t, gotReceiver.Fields["Parameter"], oldParameter.Ref, "pkg.AliasParameter", "parameter-updated")

	if !reflect.DeepEqual(scope, referenceScope) {
		t.Fatalf("fallback result differs from exact legacy propagation:\ngot=%#v\nwant=%#v", scope, referenceScope)
	}

	var recursiveVisits uint64
	if _, _, safe := replaceMethodReturnAliasBatch(root, &state, &recursiveVisits); safe {
		t.Fatal("direct batch replacement accepted cross-target containment")
	}
}

func TestTask31MethodReturnBatchFallsBackForCallerRootSharedWithStatic(t *testing.T) {
	oldReceiver := task31RegressionObject("pkg.AliasReceiver")
	oldParameter := task31RegressionObject("pkg.AliasParameter")
	updatedReceiver := oldReceiver
	updatedReceiver.Fields = map[string]Value{"Name": String("receiver-updated")}
	updatedParameter := oldParameter
	updatedParameter.Fields = map[string]Value{"Name": String("parameter-updated")}

	sharedRoot := Object("pkg.AliasEnvelope")
	sharedRoot.Fields["Receiver"] = oldReceiver
	sharedRoot.Fields["Parameter"] = oldParameter
	scope := map[string]Value{"root": sharedRoot}
	machine := New(nil)
	machine.Classes["pkg.StaticHolder"] = Class{
		Name: "pkg.StaticHolder",
		StaticFields: map[string]Field{
			"Envelope": {
				Name:  "Envelope",
				Type:  "pkg.AliasEnvelope",
				Value: sharedRoot,
			},
		},
	}
	observer := &task31ScopeAliasTraversalObserver{}
	machine.scopeAliasTraversalObserver = observer
	mutations := []methodReturnAliasMutation{
		{
			previous: snapshotAlias(oldReceiver),
			original: oldReceiver,
			updated:  updatedReceiver,
		},
		{
			previous: snapshotAlias(oldParameter),
			original: oldParameter,
			updated:  updatedParameter,
		},
	}
	beforeCaller := cloneTask31Scope(scope)
	beforeStatic := cloneValuePreserveRefs(machine.Classes["pkg.StaticHolder"].StaticFields["Envelope"].Value)

	if machine.tryPropagateMethodReturnAliasSnapshotMutationsToScope(scope, mutations) {
		t.Fatal("batch accepted a caller root whose backing is shared with a static field")
	}
	if observer.fallbacks == 0 {
		t.Fatal("caller/static shared backing did not select the legacy fallback")
	}
	if !reflect.DeepEqual(scope, beforeCaller) {
		t.Fatalf("failed batch changed caller before fallback:\ngot=%#v\nwant=%#v", scope, beforeCaller)
	}
	if got := machine.Classes["pkg.StaticHolder"].StaticFields["Envelope"].Value; !reflect.DeepEqual(got, beforeStatic) {
		t.Fatalf("failed batch changed static before fallback:\ngot=%#v\nwant=%#v", got, beforeStatic)
	}

	applyTask31LegacyMethodReturnMutations(machine, scope, mutations)
	callerRoot := scope["root"]
	staticRoot := machine.Classes["pkg.StaticHolder"].StaticFields["Envelope"].Value
	if !sameMapBacking(callerRoot.Fields, staticRoot.Fields) {
		t.Fatalf("legacy fallback split caller/static backing:\ncaller=%#v\nstatic=%#v", callerRoot, staticRoot)
	}
	assertTask31Alias(t, callerRoot.Fields["Receiver"], oldReceiver.Ref, "pkg.AliasReceiver", "receiver-updated")
	assertTask31Alias(t, staticRoot.Fields["Parameter"], oldParameter.Ref, "pkg.AliasParameter", "parameter-updated")

	callerRoot.Fields["Future"] = String("future-mutation")
	if got := staticRoot.Fields["Future"]; got.Kind != ValueString || got.Text != "future-mutation" {
		t.Fatalf("future caller mutation did not reach shared static backing: %#v", staticRoot)
	}
}

func TestTask31MethodReturnBatchFallsBackForNestedContainerSharedWithStatic(t *testing.T) {
	oldReceiver := task31RegressionObject("pkg.AliasReceiver")
	oldParameter := task31RegressionObject("pkg.AliasParameter")
	updatedReceiver := oldReceiver
	updatedReceiver.Fields = map[string]Value{"Name": String("receiver-updated")}
	updatedParameter := oldParameter
	updatedParameter.Fields = map[string]Value{"Name": String("parameter-updated")}

	sharedNested := Object("pkg.SharedAliasEnvelope")
	sharedNested.Fields["Receiver"] = oldReceiver
	sharedNested.Fields["Parameter"] = oldParameter
	outer := Object("pkg.OuterAliasEnvelope")
	outer.Fields["Nested"] = sharedNested
	scope := map[string]Value{"outer": outer}
	machine := New(nil)
	machine.Classes["pkg.StaticHolder"] = Class{
		Name: "pkg.StaticHolder",
		StaticFields: map[string]Field{
			"Nested": {
				Name:  "Nested",
				Type:  "pkg.SharedAliasEnvelope",
				Value: sharedNested,
			},
		},
	}
	observer := &task31ScopeAliasTraversalObserver{}
	machine.scopeAliasTraversalObserver = observer
	mutations := []methodReturnAliasMutation{
		{
			previous: snapshotAlias(oldReceiver),
			original: oldReceiver,
			updated:  updatedReceiver,
		},
		{
			previous: snapshotAlias(oldParameter),
			original: oldParameter,
			updated:  updatedParameter,
		},
	}
	beforeCaller := cloneTask31Scope(scope)
	beforeStatic := cloneValuePreserveRefs(machine.Classes["pkg.StaticHolder"].StaticFields["Nested"].Value)

	if machine.tryPropagateMethodReturnAliasSnapshotMutationsToScope(scope, mutations) {
		t.Fatal("batch accepted a nested caller container whose backing is shared with a static field")
	}
	if observer.fallbacks == 0 {
		t.Fatal("nested caller/static shared backing did not select the legacy fallback")
	}
	if !reflect.DeepEqual(scope, beforeCaller) {
		t.Fatalf("failed batch changed caller before fallback:\ngot=%#v\nwant=%#v", scope, beforeCaller)
	}
	if got := machine.Classes["pkg.StaticHolder"].StaticFields["Nested"].Value; !reflect.DeepEqual(got, beforeStatic) {
		t.Fatalf("failed batch changed static before fallback:\ngot=%#v\nwant=%#v", got, beforeStatic)
	}

	applyTask31LegacyMethodReturnMutations(machine, scope, mutations)
	callerNested := scope["outer"].Fields["Nested"]
	staticNested := machine.Classes["pkg.StaticHolder"].StaticFields["Nested"].Value
	if callerNested.Ref != sharedNested.Ref || staticNested.Ref != sharedNested.Ref ||
		!sameMapBacking(callerNested.Fields, staticNested.Fields) {
		t.Fatalf("legacy fallback split nested caller/static backing:\ncaller=%#v\nstatic=%#v", callerNested, staticNested)
	}
	assertTask31Alias(t, callerNested.Fields["Receiver"], oldReceiver.Ref, "pkg.AliasReceiver", "receiver-updated")
	assertTask31Alias(t, staticNested.Fields["Parameter"], oldParameter.Ref, "pkg.AliasParameter", "parameter-updated")

	callerNested.Fields["FromCaller"] = String("caller-mutation")
	if got := staticNested.Fields["FromCaller"]; got.Kind != ValueString || got.Text != "caller-mutation" {
		t.Fatalf("future caller mutation did not reach nested static backing: %#v", staticNested)
	}
	staticNested.Fields["FromStatic"] = String("static-mutation")
	if got := callerNested.Fields["FromStatic"]; got.Kind != ValueString || got.Text != "static-mutation" {
		t.Fatalf("future static mutation did not reach nested caller backing: %#v", callerNested)
	}
}

func TestTask31MethodReturnBatchFallsBackForNestedContainerSharedWithAncestorScope(t *testing.T) {
	oldReceiver := task31RegressionObject("pkg.AliasReceiver")
	oldParameter := task31RegressionObject("pkg.AliasParameter")
	updatedReceiver := oldReceiver
	updatedReceiver.Fields = map[string]Value{"Name": String("receiver-updated")}
	updatedParameter := oldParameter
	updatedParameter.Fields = map[string]Value{"Name": String("parameter-updated")}

	sharedNested := Object("pkg.SharedAliasEnvelope")
	sharedNested.Fields["Receiver"] = oldReceiver
	sharedNested.Fields["Parameter"] = oldParameter
	outer := Object("pkg.OuterAliasEnvelope")
	outer.Fields["Nested"] = sharedNested
	scope := map[string]Value{"outer": outer}
	ancestor := map[string]Value{"shared": sharedNested}
	machine := New(nil)
	machine.scopeStack = []map[string]Value{scope, ancestor}
	observer := &task31ScopeAliasTraversalObserver{}
	machine.scopeAliasTraversalObserver = observer
	mutations := []methodReturnAliasMutation{
		{
			previous: snapshotAlias(oldReceiver),
			original: oldReceiver,
			updated:  updatedReceiver,
		},
		{
			previous: snapshotAlias(oldParameter),
			original: oldParameter,
			updated:  updatedParameter,
		},
	}
	beforeCaller := cloneTask31Scope(scope)
	beforeAncestor := cloneTask31Scope(ancestor)

	if machine.tryPropagateMethodReturnAliasSnapshotMutationsToScope(scope, mutations) {
		t.Fatal("batch accepted a nested caller container whose backing is shared with an ancestor scope")
	}
	if observer.fallbacks == 0 {
		t.Fatal("nested caller/ancestor shared backing did not select the legacy fallback")
	}
	if !reflect.DeepEqual(scope, beforeCaller) {
		t.Fatalf("failed batch changed caller before fallback:\ngot=%#v\nwant=%#v", scope, beforeCaller)
	}
	if !reflect.DeepEqual(ancestor, beforeAncestor) {
		t.Fatalf("failed batch changed ancestor before fallback:\ngot=%#v\nwant=%#v", ancestor, beforeAncestor)
	}

	applyTask31LegacyMethodReturnMutations(machine, scope, mutations)
	callerNested := scope["outer"].Fields["Nested"]
	ancestorNested := ancestor["shared"]
	if callerNested.Ref != sharedNested.Ref || ancestorNested.Ref != sharedNested.Ref ||
		!sameMapBacking(callerNested.Fields, ancestorNested.Fields) {
		t.Fatalf("legacy fallback split nested caller/ancestor backing:\ncaller=%#v\nancestor=%#v", callerNested, ancestorNested)
	}
	assertTask31Alias(t, callerNested.Fields["Receiver"], oldReceiver.Ref, "pkg.AliasReceiver", "receiver-updated")
	assertTask31Alias(t, ancestorNested.Fields["Parameter"], oldParameter.Ref, "pkg.AliasParameter", "parameter-updated")

	callerNested.Fields["FromCaller"] = String("caller-mutation")
	if got := ancestorNested.Fields["FromCaller"]; got.Kind != ValueString || got.Text != "caller-mutation" {
		t.Fatalf("future caller mutation did not reach ancestor backing: %#v", ancestorNested)
	}
	ancestorNested.Fields["FromAncestor"] = String("ancestor-mutation")
	if got := callerNested.Fields["FromAncestor"]; got.Kind != ValueString || got.Text != "ancestor-mutation" {
		t.Fatalf("future ancestor mutation did not reach caller backing: %#v", callerNested)
	}
}

func TestTask31RefLessReceiverDoesNotEnrichUnrelatedCallerAlias(t *testing.T) {
	program, err := CompileAnonymous(`this.Child.Name = 'rich';`)
	if err != nil {
		t.Fatal(err)
	}
	method := Method{
		Name:       "pkg.RefLessReceiver.touchChild",
		ClassName:  "pkg.RefLessReceiver",
		ReturnType: "void",
		Program:    program,
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "AliasChild",
		Namespace:  "pkg",
		Dependency: true,
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String", Access: "public"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{
		Name:       "RefLessReceiver",
		Namespace:  "pkg",
		Dependency: true,
		Fields: map[string]Field{
			"Child": {Name: "Child", Type: "pkg.AliasChild", Access: "public"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	child := task31RegressionObject("pkg.AliasChild")
	child.Fields = map[string]Value{}
	unrelated := child
	unrelated.Fields = map[string]Value{}
	receiver := task31RegressionObject("pkg.RefLessReceiver")
	receiver.Ref = 0
	receiver.Fields["Child"] = child
	machine.Globals = map[string]Value{"unrelated": unrelated}

	if _, err := machine.callMethodWithReceiver(method, receiver, nil, &Result{}); err != nil {
		t.Fatal(err)
	}
	got := machine.Globals["unrelated"]
	if name := got.Fields["Name"]; name.Kind != "" {
		t.Fatalf("ref-less receiver enriched unrelated caller alias: %#v", got)
	}
}

func applyTask31LegacyMethodReturnMutations(machine *VM, scope map[string]Value, mutations []methodReturnAliasMutation) {
	for _, mutation := range mutations {
		if !machine.propagateMethodReturnAliasSnapshotMutationToScope(
			scope,
			mutation.previous,
			mutation.original,
			mutation.updated,
			mutation.refreshNestedCollections,
		) {
			continue
		}
		machine.propagateAliasSnapshotToStatics(mutation.previous, mutation.updated)
		machine.propagateUpdatedValueAliases(scope, mutation.updated)
	}
}

func task31RegressionObject(typeName string) Value {
	value := Object(typeName)
	value.Static = typeName
	value.Runtime = typeName
	return value
}

func cloneTask31RegressionMutations(mutations []methodReturnAliasMutation) []methodReturnAliasMutation {
	cloned := make([]methodReturnAliasMutation, len(mutations))
	for i, mutation := range mutations {
		cloned[i] = mutation
		cloned[i].original = cloneValuePreserveRefs(mutation.original)
		cloned[i].updated = cloneValuePreserveRefs(mutation.updated)
	}
	return cloned
}
