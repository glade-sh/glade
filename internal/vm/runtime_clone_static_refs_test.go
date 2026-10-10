package vm

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestCloneRuntimeFrozenSharedStaticRefsMatchFreshCollection(t *testing.T) {
	for _, mode := range []string{"shared-class-map", "copy-plan", "without-copy-plan"} {
		t.Run(mode, func(t *testing.T) {
			template, target := staticRefsCloneFixture(t)
			if mode == "copy-plan" {
				class := template.Classes["ListRegistry"]
				class.IsTest = true
				template.Classes["ListRegistry"] = class
			}
			template.FreezeClassLookup()
			if mode == "without-copy-plan" {
				template.sharedClassCopyPlan = nil
			}
			clone := template.CloneRuntimeFrozenShared(nil)
			assertStaticRefsShared(t, template, clone)
			assertStaticRefsMatchFresh(t, clone)
			if got := clone.staticValueRefFields[target.Ref].len(); got != 4 {
				t.Fatalf("shared record locations = %d, want 4", got)
			}
			if got := len(clone.staticValueRefFields[target.Ref].many); got < 2 {
				t.Fatalf("shared record many locations = %d, want at least 2", got)
			}
			before := normalizedStaticRefLocations(template.staticValueRefFields)
			if _, ok := clone.ensureMutableClass("pkg.ListRegistry"); !ok {
				t.Fatal("list registry missing")
			}
			assertStaticRefsMatchFresh(t, clone)
			if !reflect.DeepEqual(before, normalizedStaticRefLocations(template.staticValueRefFields)) {
				t.Fatal("detaching clone static fields changed the template index")
			}
		})
	}
}

func TestCloneRuntimeCopiedStaticsCollectOwnRefs(t *testing.T) {
	for _, frozen := range []bool{false, true} {
		t.Run(fmt.Sprintf("frozen=%t", frozen), func(t *testing.T) {
			template, target := staticRefsCloneFixture(t)
			template.staticValueRefs, template.staticValueRefFields = template.collectStaticValueRefs()
			if frozen {
				template.FreezeClassLookup()
			}
			clone := template.CloneRuntime(nil)
			if clone.staticValueRefs != nil || clone.staticValueRefFields != nil {
				t.Fatal("deep-copied statics must collect their newly allocated refs")
			}
			if got := clone.Classes["ListRegistry"].StaticFields["Items"].Value.List[0].Ref; got == target.Ref {
				t.Fatal("copied static record retained the template ref")
			}
			if err := mutateStaticRefsCloneRecord(clone, "copied"); err != nil {
				t.Fatal(err)
			}
			assertStaticRefsMatchFresh(t, clone)
			assertStaticRefsMatchFresh(t, template)
		})
	}
}

func TestCloneRuntimeFrozenSharedStaticRefsDivergentAliases(t *testing.T) {
	template, _ := staticRefsCloneFixture(t)
	short := template.Classes["ListRegistry"]
	record := Object("Account")
	record.Fields["Name"] = String("divergent")
	items := List(record)
	short.StaticFields = map[string]Field{
		"Items": {Name: "Items", Type: "List<Account>", Static: true, Value: items, InitialValue: items},
	}
	template.Classes["ListRegistry"] = short
	template.FreezeClassLookup()
	if template.sharedClassCopyPlan.uniformAliases {
		t.Fatal("fixture must have divergent class aliases")
	}
	clone := template.CloneRuntimeFrozenShared(nil)
	if clone.staticValueRefs != nil || clone.staticValueRefFields != nil {
		t.Fatal("normalized static aliases must collect their own index")
	}
	if !sameMap(clone.Classes["ListRegistry"].StaticFields, clone.Classes["pkg.ListRegistry"].StaticFields) {
		t.Fatal("planned clone did not normalize static aliases")
	}
	if err := mutateStaticRefsCloneRecord(clone, "normalized"); err != nil {
		t.Fatal(err)
	}
	assertStaticRefsMatchFresh(t, clone)
	if got := template.Classes["ListRegistry"].StaticFields["Items"].Value.List[0].Fields["Name"].Text; got != "divergent" {
		t.Fatalf("short source alias record = %q, want divergent", got)
	}
	if got := template.Classes["pkg.ListRegistry"].StaticFields["Items"].Value.List[0].Fields["Name"].Text; got != "seed" {
		t.Fatalf("qualified source alias record = %q, want seed", got)
	}
}

func TestCloneRuntimeSharedStaticRefsWriteIsolation(t *testing.T) {
	for _, sourceWrites := range []bool{false, true} {
		t.Run(fmt.Sprintf("source-writes=%t", sourceWrites), func(t *testing.T) {
			template, _ := staticRefsCloneFixture(t)
			template.FreezeClassLookup()
			clone := template.CloneRuntimeFrozenShared(nil)
			assertStaticRefsShared(t, template, clone)
			writer, observer := clone, template
			if sourceWrites {
				writer, observer = template, clone
			}
			beforeRefs, beforeFields := observer.collectStaticValueRefs()
			replacement := List(Object("Replacement"))
			class := writer.Classes["pkg.ListRegistry"]
			writer.writeStaticFieldValue("pkg.ListRegistry", "Items", class, class.StaticFields["Items"], replacement)
			assertStaticRefsMatchFresh(t, writer)
			if !writer.staticValueRefs[replacement.Ref] {
				t.Fatal("static write did not index replacement")
			}
			if !reflect.DeepEqual(observer.staticValueRefs, beforeRefs) || !reflect.DeepEqual(normalizedStaticRefLocations(observer.staticValueRefFields), normalizedStaticRefLocations(beforeFields)) {
				t.Fatal("static write changed the other runtime's index")
			}
			assertStaticRefsMatchFresh(t, observer)
			if got := observer.Classes["ListRegistry"].StaticFields["Items"].Value.List[0].Fields["Name"].Text; got != "seed" {
				t.Fatalf("other runtime's static record = %q, want seed", got)
			}
		})
	}
}

func TestCloneRuntimeSharedStaticRefsMutatorsDetach(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*VM, Value, Value, staticFieldRef)
	}{
		{"remember", func(vm *VM, _, next Value, _ staticFieldRef) { vm.rememberStaticValueRefs(next) }},
		{"remember-field", func(vm *VM, _, next Value, location staticFieldRef) {
			vm.rememberStaticValueRefsInField(next, location)
		}},
		{"replace-field", func(vm *VM, previous, next Value, location staticFieldRef) {
			vm.replaceStaticValueRefsInField(previous, next, location)
		}},
		{"forget-field", func(vm *VM, _, _ Value, location staticFieldRef) { vm.forgetStaticValueRefsInField(location) }},
		{"forget-value", func(vm *VM, previous, _ Value, location staticFieldRef) {
			vm.forgetStaticValueRefsFromValue(previous, location)
		}},
		{"forget-ref", func(vm *VM, previous, _ Value, location staticFieldRef) {
			vm.forgetStaticValueRefInField(previous.Ref, location)
		}},
		{"collect-field", func(vm *VM, _, next Value, location staticFieldRef) {
			vm.collectStaticFieldValueRefsInField(next, location)
		}},
		{"collect-additional", func(vm *VM, _, next Value, location staticFieldRef) {
			vm.collectAdditionalStaticFieldValueRefsInField(next, location)
		}},
		{"remember-additional", func(vm *VM, previous, next Value, location staticFieldRef) {
			vm.rememberAdditionalStaticValueRefsInField(previous, next, location)
		}},
		{"alias-update", func(vm *VM, previous, next Value, location staticFieldRef) {
			vm.rememberStaticAliasUpdateRefs(snapshotAlias(previous), next, location)
		}},
	}
	for _, test := range tests {
		for _, sourceWrites := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/source-writes=%t", test.name, sourceWrites), func(t *testing.T) {
				template, target := staticRefsCloneFixture(t)
				template.FreezeClassLookup()
				clone := template.CloneRuntimeFrozenShared(nil)
				assertStaticRefsShared(t, template, clone)
				writer, observer := clone, template
				if sourceWrites {
					writer, observer = template, clone
				}
				beforeRefs, beforeFields := observer.collectStaticValueRefs()
				// Pick an overflow location so removals exercise the nested map,
				// regardless of collection map iteration order.
				var location staticFieldRef
				for candidate := range writer.staticValueRefFields[target.Ref].many {
					location = candidate
					break
				}
				if location.ClassName == "" {
					t.Fatal("fixture has no overflow location")
				}
				next := Object("Replacement")
				next.Fields["Existing"] = target
				test.mutate(writer, target, next, location)
				if sameMap(writer.staticValueRefs, observer.staticValueRefs) || sameMap(writer.staticValueRefFields, observer.staticValueRefFields) {
					t.Fatal("mutator retained a shared index")
				}
				if !reflect.DeepEqual(observer.staticValueRefs, beforeRefs) || !reflect.DeepEqual(normalizedStaticRefLocations(observer.staticValueRefFields), normalizedStaticRefLocations(beforeFields)) {
					t.Fatal("mutator changed the other runtime's index")
				}
			})
		}
	}
}

func TestCloneRuntimeSharedStaticRefsInvalidationIsolation(t *testing.T) {
	for _, forChange := range []bool{false, true} {
		for _, sourceInvalidates := range []bool{false, true} {
			t.Run(fmt.Sprintf("change=%t/source=%t", forChange, sourceInvalidates), func(t *testing.T) {
				template, target := staticRefsCloneFixture(t)
				template.FreezeClassLookup()
				clone := template.CloneRuntimeFrozenShared(nil)
				assertStaticRefsShared(t, template, clone)
				writer, observer := clone, template
				if sourceInvalidates {
					writer, observer = template, clone
				}
				beforeRefs, beforeFields := observer.collectStaticValueRefs()
				if forChange {
					writer.invalidateStaticValueRefsForChange(target, Object("Changed"))
				} else {
					writer.invalidateStaticValueRefs()
				}
				if writer.staticValueRefs != nil || writer.staticValueRefFields != nil {
					t.Fatal("invalidated index is not nil")
				}
				if !reflect.DeepEqual(observer.staticValueRefs, beforeRefs) || !reflect.DeepEqual(normalizedStaticRefLocations(observer.staticValueRefFields), normalizedStaticRefLocations(beforeFields)) {
					t.Fatal("invalidation changed the other runtime's index")
				}
				if err := mutateStaticRefsCloneRecord(writer, "after-invalidation"); err != nil {
					t.Fatal(err)
				}
				assertStaticRefsMatchFresh(t, writer)
				assertStaticRefsMatchFresh(t, observer)
			})
		}
	}
}

func TestCloneRuntimeSharedStaticRefsCloneChain(t *testing.T) {
	template, _ := staticRefsCloneFixture(t)
	template.FreezeClassLookup()
	first := template.CloneRuntimeFrozenShared(nil)
	second := first.CloneRuntimeFrozenShared(nil)
	third := second.CloneRuntimeFrozenShared(nil)
	for _, clone := range []*VM{first, second, third} {
		assertStaticRefsShared(t, template, clone)
		assertStaticRefsMatchFresh(t, clone)
	}
	if err := mutateStaticRefsCloneRecord(second, "second"); err != nil {
		t.Fatal(err)
	}
	afterWrite := second.CloneRuntimeFrozenShared(nil)
	assertStaticRefsShared(t, second, afterWrite)
	if err := mutateStaticRefsCloneRecord(second, "second-again"); err != nil {
		t.Fatal(err)
	}
	if got := afterWrite.Classes["ListRegistry"].StaticFields["Items"].Value.List[0].Fields["Name"].Text; got != "second" {
		t.Fatalf("source write after cloning changed descendant record: %q", got)
	}
	for _, clone := range []*VM{template, first, second, third, afterWrite} {
		assertStaticRefsMatchFresh(t, clone)
	}
	for _, clone := range []*VM{template, first, third} {
		if got := clone.Classes["ListRegistry"].StaticFields["Items"].Value.List[0].Fields["Name"].Text; got != "seed" {
			t.Fatalf("clone chain leaked static record update: %q", got)
		}
	}
}

func TestCloneRuntimeSharedStaticRefsRegistrationIsolation(t *testing.T) {
	for _, sourceRegisters := range []bool{false, true} {
		t.Run(fmt.Sprintf("source-registers=%t", sourceRegisters), func(t *testing.T) {
			template, _ := staticRefsCloneFixture(t)
			template.FreezeClassLookup()
			clone := template.CloneRuntimeFrozenShared(nil)
			assertStaticRefsShared(t, template, clone)
			writer, observer := clone, template
			if sourceRegisters {
				writer, observer = template, clone
			}
			beforeRefs, beforeFields := observer.collectStaticValueRefs()
			value := Object("Account")
			value.Fields["Name"] = String("new")
			if err := writer.RegisterClass(Class{Name: "LateRegistry", StaticFields: map[string]Field{
				"Record": {Name: "Record", Type: "Account", Static: true, Value: value, InitialValue: value},
			}}); err != nil {
				t.Fatal(err)
			}
			class, ok := writer.ensureMutableClass("LateRegistry")
			if !ok {
				t.Fatal("registered class missing")
			}
			previous := class.StaticFields["Record"].Value
			updated := Object("Account")
			updated.Ref = previous.Ref
			updated.Fields["Name"] = String("updated")
			writer.propagateAliasSnapshotToStatics(snapshotAlias(previous), updated)
			if got := writer.Classes["LateRegistry"].StaticFields["Record"].Value.Fields["Name"].Text; got != "updated" {
				t.Fatalf("registered record Name = %q, want updated", got)
			}
			assertStaticRefsMatchFresh(t, writer)
			if _, ok := observer.Classes["LateRegistry"]; ok {
				t.Fatal("registration changed other runtime's classes")
			}
			if !reflect.DeepEqual(observer.staticValueRefs, beforeRefs) || !reflect.DeepEqual(normalizedStaticRefLocations(observer.staticValueRefFields), normalizedStaticRefLocations(beforeFields)) {
				t.Fatal("registration changed other runtime's static index")
			}
		})
	}
}

func TestCloneRuntimeSharedStaticRefsRegistrationResetsValues(t *testing.T) {
	for _, sourceRegisters := range []bool{false, true} {
		t.Run(fmt.Sprintf("source-registers=%t", sourceRegisters), func(t *testing.T) {
			template, _ := staticRefsCloneFixture(t)
			template.FreezeClassLookup()
			clone := template.CloneRuntimeFrozenShared(nil)
			assertStaticRefsShared(t, template, clone)
			writer, observer := clone, template
			if sourceRegisters {
				writer, observer = template, clone
			}
			beforeRefs, beforeFields := observer.collectStaticValueRefs()
			// Detach before passing the runtime's own field map back through
			// registration, which resets Value from InitialValue in place.
			class, ok := writer.ensureMutableClass("pkg.RecordRegistry")
			if !ok {
				t.Fatal("record registry missing")
			}
			oldRef := class.StaticFields["Record"].Value.Ref
			if err := writer.RegisterClass(class); err != nil {
				t.Fatal(err)
			}
			previous := writer.Classes["pkg.RecordRegistry"].StaticFields["Record"].Value
			if previous.Ref == oldRef {
				t.Fatal("registration did not reset the static record ref")
			}
			updated := Object("Account")
			updated.Ref = previous.Ref
			updated.Fields["Name"] = String("registered-again")
			writer.propagateAliasSnapshotToStatics(snapshotAlias(previous), updated)
			if got := writer.Classes["pkg.RecordRegistry"].StaticFields["Record"].Value.Fields["Name"].Text; got != "registered-again" {
				t.Fatalf("registered record Name = %q, want registered-again", got)
			}
			assertStaticRefsMatchFresh(t, writer)
			if !reflect.DeepEqual(observer.staticValueRefs, beforeRefs) || !reflect.DeepEqual(normalizedStaticRefLocations(observer.staticValueRefFields), normalizedStaticRefLocations(beforeFields)) {
				t.Fatal("registration reset changed other runtime's static index")
			}
			assertStaticRefsMatchFresh(t, observer)
		})
	}
}

func TestCloneRuntimeSharedStaticRefsCollectOnce(t *testing.T) {
	template, _ := staticRefsCloneFixture(t)
	template.FreezeClassLookup()
	recorder := NewPerfRecorder()
	template.SetPerfRecorder(recorder)
	const clones = 12
	for i := 0; i < clones; i++ {
		clone := template.CloneRuntimeFrozenShared(nil)
		if err := mutateStaticRefsCloneRecord(clone, fmt.Sprintf("clone-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	perf := recorder.Snapshot().StaticAlias
	t.Logf("clones=%d staticAlias.collectCalls=%d staticAlias.collectNs=%d", clones, perf.CollectCalls, perf.CollectNS)
	if perf.CollectCalls != 1 {
		t.Fatalf("static collections = %d, want one per template", perf.CollectCalls)
	}
}

func TestCloneRuntimeSharedStaticRefsConcurrentClones(t *testing.T) {
	template, _ := staticRefsCloneFixture(t)
	template.FreezeClassLookup()
	recorder := NewPerfRecorder()
	template.SetPerfRecorder(recorder)
	const workers = 8
	clones := make([]*VM, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range clones {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			clones[i] = template.CloneRuntimeFrozenShared(nil)
			errs[i] = mutateStaticRefsCloneRecord(clones[i], fmt.Sprintf("worker-%d", i))
		}(i)
	}
	wg.Wait()
	if got := recorder.Snapshot().StaticAlias.CollectCalls; got != 1 {
		t.Fatalf("concurrent static collections = %d, want one", got)
	}
	for i, clone := range clones {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		assertStaticRefsMatchFresh(t, clone)
		if got, want := clone.Classes["ListRegistry"].StaticFields["Items"].Value.List[0].Fields["Name"].Text, fmt.Sprintf("worker-%d", i); got != want {
			t.Fatalf("clone %d record = %q, want %q", i, got, want)
		}
	}
	assertStaticRefsMatchFresh(t, template)
	if got := template.Classes["ListRegistry"].StaticFields["Items"].Value.List[0].Fields["Name"].Text; got != "seed" {
		t.Fatalf("template record = %q, want seed", got)
	}
}

func staticRefsCloneFixture(t *testing.T) (*VM, Value) {
	t.Helper()
	template := New(nil)
	record := Object("Account")
	record.Fields["Name"] = String("seed")
	values := typedMap("Map<String,Account>")
	key := mapKey(String("record"))
	values.Map[key] = record
	values.MapKeys[key] = String("record")
	values.MapOrder = append(values.MapOrder, key)
	for _, entry := range []struct {
		name   string
		fields map[string]Value
	}{
		{"ListRegistry", map[string]Value{"Items": List(record), "OtherItems": List(record)}},
		{"MapRegistry", map[string]Value{"Entries": values}},
		{"RecordRegistry", map[string]Value{"Record": record}},
	} {
		fields := make(map[string]Field, len(entry.fields))
		for name, value := range entry.fields {
			fields[name] = Field{Name: name, Type: value.Type, Static: true, Value: value, InitialValue: value}
		}
		if err := template.RegisterClass(Class{Name: entry.name, Namespace: "pkg", StaticFields: fields}); err != nil {
			t.Fatal(err)
		}
		// Registration copies field values. Restore one record shared by
		// four static locations to exercise both single and overflow sets.
		class := template.Classes[entry.name]
		for name, value := range entry.fields {
			field := class.StaticFields[name]
			field.Value = value
			class.StaticFields[name] = field
		}
	}
	return template, record
}

func mutateStaticRefsCloneRecord(machine *VM, name string) error {
	class, ok := machine.ensureMutableClass("pkg.ListRegistry")
	if !ok {
		return fmt.Errorf("list registry missing")
	}
	previous := class.StaticFields["Items"].Value.List[0]
	updated := Object("Account")
	updated.Ref = previous.Ref
	updated.Fields["Name"] = String(name)
	machine.propagateAliasSnapshotToStatics(snapshotAlias(previous), updated)
	if got := machine.Classes["ListRegistry"].StaticFields["Items"].Value.List[0].Fields["Name"].Text; got != name {
		return fmt.Errorf("static record Name = %q, want %q", got, name)
	}
	return nil
}

func assertStaticRefsShared(t *testing.T, source, clone *VM) {
	t.Helper()
	if source.staticValueRefs == nil || source.staticValueRefFields == nil || clone.staticValueRefs == nil || clone.staticValueRefFields == nil {
		t.Fatal("source and clone must have collected static refs")
	}
	if !sameMap(source.staticValueRefs, clone.staticValueRefs) || !sameMap(source.staticValueRefFields, clone.staticValueRefFields) {
		t.Fatal("source and clone static index maps are not shared")
	}
}

func assertStaticRefsMatchFresh(t *testing.T, machine *VM) {
	t.Helper()
	refs, fields := machine.collectStaticValueRefs()
	if !reflect.DeepEqual(machine.staticValueRefs, refs) {
		t.Fatalf("cached static refs = %v, fresh = %v", machine.staticValueRefs, refs)
	}
	if got, want := normalizedStaticRefLocations(machine.staticValueRefFields), normalizedStaticRefLocations(fields); !reflect.DeepEqual(got, want) {
		t.Fatalf("cached static locations = %v, fresh = %v", got, want)
	}
}

func normalizedStaticRefLocations(fields map[uint64]staticFieldRefSet) map[uint64]map[staticFieldRef]bool {
	result := make(map[uint64]map[staticFieldRef]bool, len(fields))
	for ref, locations := range fields {
		set := make(map[staticFieldRef]bool, locations.len())
		locations.forEach(func(location staticFieldRef) { set[location] = true })
		result[ref] = set
	}
	return result
}
