package apextest

import (
	"crypto/sha256"
	"fmt"
	"math"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type runtimeFingerprintErrorA string

func (err runtimeFingerprintErrorA) Error() string { return string(err) }

type runtimeFingerprintErrorB string

func (err runtimeFingerprintErrorB) Error() string { return string(err) }

func TestRuntimePatchFingerprintWriterRejectsNegativeCountsAndPreservesSignedIntegers(t *testing.T) {
	writer := runtimePatchFingerprintWriter{
		hash: sha256.New(),
		ok:   true,
	}
	writer.count(0x01, -1)
	if writer.ok {
		t.Fatal("negative count remained eligible for fingerprint publication")
	}

	requireDifferentRuntimePayloadFingerprints(
		t,
		runtimeFingerprintValueEntry(vm.Int(-1)),
		runtimeFingerprintValueEntry(vm.Int(1)),
	)
}

func TestRuntimePatchFingerprintIgnoresMapInsertionOrder(t *testing.T) {
	first := runtimeFingerprintFixture(8)
	second := runtimeFingerprintFixture(8)
	firstFingerprint := requireRuntimePayloadFingerprint(t, first)
	if originalSecond := requireRuntimePayloadFingerprint(t, second); firstFingerprint != originalSecond {
		t.Fatalf("independently constructed fixtures differ before map reordering: %s != %s", firstFingerprint, originalSecond)
	}
	second.Methods = reverseRuntimeFingerprintMethods(second.Methods)
	for i := range second.Classes {
		second.Classes[i].Fields = reverseRuntimeFingerprintFields(second.Classes[i].Fields)
		second.Classes[i].StaticFields = reverseRuntimeFingerprintFields(second.Classes[i].StaticFields)
		second.Classes[i].Methods = reverseRuntimeFingerprintMethods(second.Classes[i].Methods)
		for name, field := range second.Classes[i].Fields {
			field.Value.Fields = reverseRuntimeFingerprintValues(field.Value.Fields)
			field.Value.Map = reverseRuntimeFingerprintValues(field.Value.Map)
			field.Value.MapKeys = reverseRuntimeFingerprintValues(field.Value.MapKeys)
			second.Classes[i].Fields[name] = field
		}
	}
	secondFingerprint := requireRuntimePayloadFingerprint(t, second)
	if firstFingerprint != secondFingerprint {
		t.Fatalf("map insertion order changed fingerprint: %s != %s", firstFingerprint, secondFingerprint)
	}
}

func TestRuntimePatchFingerprintPreservesNilEmptyAndOrderingSemantics(t *testing.T) {
	tests := []struct {
		name   string
		first  runtimeCacheEntry
		second runtimeCacheEntry
	}{
		{"methods nil empty", runtimeCacheEntry{}, runtimeCacheEntry{Methods: map[string]vm.Method{}}},
		{"classes nil empty", runtimeCacheEntry{}, runtimeCacheEntry{Classes: []vm.Class{}}},
		{"triggers nil empty", runtimeCacheEntry{}, runtimeCacheEntry{Triggers: []vm.Trigger{}}},
		{"pages nil empty", runtimeCacheEntry{}, runtimeCacheEntry{PageNames: []string{}}},
		{
			"value fields nil empty",
			runtimeFingerprintValueEntry(vm.Value{Kind: vm.ValueObject}),
			runtimeFingerprintValueEntry(vm.Value{Kind: vm.ValueObject, Fields: map[string]vm.Value{}}),
		},
		{
			"value list nil empty",
			runtimeFingerprintValueEntry(vm.Value{Kind: vm.ValueList}),
			runtimeFingerprintValueEntry(vm.Value{Kind: vm.ValueList, List: []vm.Value{}}),
		},
		{
			"value set nil empty",
			runtimeFingerprintValueEntry(vm.Value{Kind: vm.ValueSet}),
			runtimeFingerprintValueEntry(vm.Value{Kind: vm.ValueSet, Set: []vm.Value{}}),
		},
		{
			"value map nil empty",
			runtimeFingerprintValueEntry(vm.Value{Kind: vm.ValueMap}),
			runtimeFingerprintValueEntry(vm.Value{Kind: vm.ValueMap, Map: map[string]vm.Value{}}),
		},
		{
			"value map keys nil empty",
			runtimeFingerprintValueEntry(vm.Value{Kind: vm.ValueMap}),
			runtimeFingerprintValueEntry(vm.Value{Kind: vm.ValueMap, MapKeys: map[string]vm.Value{}}),
		},
		{
			"value map order nil empty",
			runtimeFingerprintValueEntry(vm.Value{Kind: vm.ValueMap}),
			runtimeFingerprintValueEntry(vm.Value{Kind: vm.ValueMap, MapOrder: []string{}}),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			requireDifferentRuntimePayloadFingerprints(t, test.first, test.second)
		})
	}

	base := runtimeFingerprintFixture(4)
	reorderedClasses := cloneRuntimeFingerprintEntry(t, base)
	reorderedClasses.Classes[0], reorderedClasses.Classes[1] = reorderedClasses.Classes[1], reorderedClasses.Classes[0]
	requireDifferentRuntimePayloadFingerprints(t, base, reorderedClasses)

	reorderedTriggers := cloneRuntimeFingerprintEntry(t, base)
	reorderedTriggers.Triggers[0], reorderedTriggers.Triggers[1] = reorderedTriggers.Triggers[1], reorderedTriggers.Triggers[0]
	requireDifferentRuntimePayloadFingerprints(t, base, reorderedTriggers)

	reorderedPages := cloneRuntimeFingerprintEntry(t, base)
	reorderedPages.PageNames[0], reorderedPages.PageNames[1] = reorderedPages.PageNames[1], reorderedPages.PageNames[0]
	requireDifferentRuntimePayloadFingerprints(t, base, reorderedPages)

	reorderedFields := cloneRuntimeFingerprintEntry(t, base)
	reorderedFields.Classes[0].FieldOrder[0], reorderedFields.Classes[0].FieldOrder[1] =
		reorderedFields.Classes[0].FieldOrder[1], reorderedFields.Classes[0].FieldOrder[0]
	requireDifferentRuntimePayloadFingerprints(t, base, reorderedFields)

	reorderedMethods := cloneRuntimeFingerprintEntry(t, base)
	reorderedMethods.Classes[0].Constructors[0], reorderedMethods.Classes[0].Constructors[1] =
		reorderedMethods.Classes[0].Constructors[1], reorderedMethods.Classes[0].Constructors[0]
	requireDifferentRuntimePayloadFingerprints(t, base, reorderedMethods)
}

func TestRuntimePatchFingerprintBindsBehaviorAffectingPayload(t *testing.T) {
	base := runtimeFingerprintFixture(4)
	mutations := []struct {
		name   string
		mutate func(*runtimeCacheEntry)
	}{
		{"method", func(entry *runtimeCacheEntry) {
			method := entry.Methods["Example.run"]
			method.Program.Source = "changed source"
			entry.Methods["Example.run"] = method
		}},
		{"class", func(entry *runtimeCacheEntry) { entry.Classes[0].Name = "ChangedClass" }},
		{"trigger", func(entry *runtimeCacheEntry) { entry.Triggers[0].Timing = "after" }},
		{"field value", func(entry *runtimeCacheEntry) {
			field := entry.Classes[0].StaticFields["State"]
			field.Value = vm.Int(99)
			entry.Classes[0].StaticFields["State"] = field
		}},
		{"field initial value", func(entry *runtimeCacheEntry) {
			field := entry.Classes[0].StaticFields["State"]
			field.InitialValue = vm.Int(99)
			entry.Classes[0].StaticFields["State"] = field
		}},
		{"page", func(entry *runtimeCacheEntry) { entry.PageNames[0] = "ChangedPage" }},
		{"trigger error type", func(entry *runtimeCacheEntry) {
			entry.TriggerErrors[0] = runtimeFingerprintErrorB("trigger failure")
		}},
		{"trigger error text", func(entry *runtimeCacheEntry) {
			entry.TriggerErrors[0] = runtimeFingerprintErrorA("changed failure")
		}},
		{"base error type", func(entry *runtimeCacheEntry) {
			entry.BaseErr = runtimeFingerprintErrorB("base failure")
		}},
		{"base error text", func(entry *runtimeCacheEntry) {
			entry.BaseErr = runtimeFingerprintErrorA("changed failure")
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			candidate := cloneRuntimeFingerprintEntry(t, base)
			mutation.mutate(&candidate)
			requireDifferentRuntimePayloadFingerprints(t, base, candidate)
		})
	}
}

func TestRuntimePatchFingerprintBindsNestedValuesAndReferences(t *testing.T) {
	baseValue := vm.Value{
		Kind:    vm.ValueObject,
		Type:    "Example",
		Static:  "Owner",
		Runtime: "request",
		Ref:     17,
		Fields: map[string]vm.Value{
			"name": vm.String("first"),
		},
		List:     []vm.Value{vm.Int(1), vm.Bool(true)},
		Set:      []vm.Value{vm.String("a"), vm.String("b")},
		Map:      map[string]vm.Value{"first": vm.Decimal(1.25)},
		MapKeys:  map[string]vm.Value{"first": vm.String("logical key")},
		MapOrder: []string{"first"},
	}
	base := runtimeFingerprintValueEntry(baseValue)
	mutations := []struct {
		name   string
		mutate func(*vm.Value)
	}{
		{"kind", func(value *vm.Value) { value.Kind = vm.ValueMap }},
		{"integer", func(value *vm.Value) { value.Int++ }},
		{"decimal", func(value *vm.Value) { value.Decimal++ }},
		{"boolean", func(value *vm.Value) { value.Bool = !value.Bool }},
		{"text", func(value *vm.Value) { value.Text = "changed" }},
		{"type", func(value *vm.Value) { value.Type = "Changed" }},
		{"static", func(value *vm.Value) { value.Static = "Changed" }},
		{"runtime", func(value *vm.Value) { value.Runtime = "changed" }},
		{"reference", func(value *vm.Value) { value.Ref++ }},
		{"field", func(value *vm.Value) { value.Fields["name"] = vm.String("changed") }},
		{"list", func(value *vm.Value) { value.List[0] = vm.Int(2) }},
		{"set", func(value *vm.Value) { value.Set[0], value.Set[1] = value.Set[1], value.Set[0] }},
		{"map", func(value *vm.Value) { value.Map["first"] = vm.Decimal(2.5) }},
		{"map key", func(value *vm.Value) { value.MapKeys["first"] = vm.String("changed") }},
		{"map order", func(value *vm.Value) { value.MapOrder = []string{"changed"} }},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			candidate := cloneRuntimeFingerprintEntry(t, base)
			field := candidate.Classes[0].Fields["Payload"]
			mutation.mutate(&field.Value)
			candidate.Classes[0].Fields["Payload"] = field
			requireDifferentRuntimePayloadFingerprints(t, base, candidate)
		})
	}

	repeated := vm.List(vm.Int(1))
	references := runtimeCacheEntry{Classes: []vm.Class{{Fields: map[string]vm.Field{
		"First":  {Name: "First", Value: repeated},
		"Second": {Name: "Second", Value: repeated},
	}}}}
	changedReference := references
	changedReference.Classes = append([]vm.Class(nil), references.Classes...)
	changedReference.Classes[0].Fields = reverseRuntimeFingerprintFields(references.Classes[0].Fields)
	second := changedReference.Classes[0].Fields["Second"]
	second.Value.Ref++
	changedReference.Classes[0].Fields["Second"] = second
	requireDifferentRuntimePayloadFingerprints(t, references, changedReference)
}

func TestRuntimePatchFingerprintRejectsCyclesAndExcessiveDepth(t *testing.T) {
	cycleMap := make(map[string]vm.Value)
	cycle := vm.Value{Kind: vm.ValueMap, Map: cycleMap}
	cycleMap["self"] = cycle
	if fingerprint, ok := runtimePatchCompiledPayloadFingerprint(runtimeFingerprintValueEntry(cycle)); ok || fingerprint != "" {
		t.Fatalf("cyclic payload fingerprint = %q, ok %t", fingerprint, ok)
	}

	deep := vm.Int(1)
	for i := 0; i < 258; i++ {
		deep = vm.Value{Kind: vm.ValueList, List: []vm.Value{deep}}
	}
	if fingerprint, ok := runtimePatchCompiledPayloadFingerprint(runtimeFingerprintValueEntry(deep)); ok || fingerprint != "" {
		t.Fatalf("excessively deep payload fingerprint = %q, ok %t", fingerprint, ok)
	}

	expressionCycle := ir.Expr{Kind: ir.ExprUnary, Operator: "-"}
	expressionCycle.Left = &expressionCycle
	cyclicProgram := runtimeCacheEntry{Methods: map[string]vm.Method{
		"Cycle.run": {Name: "Cycle.run", Program: ir.Program{
			Instructions: []ir.Instruction{{Op: ir.OpReturn, Expr: expressionCycle}},
		}},
	}}
	if fingerprint, ok := runtimePatchCompiledPayloadFingerprint(cyclicProgram); ok || fingerprint != "" {
		t.Fatalf("cyclic IR fingerprint = %q, ok %t", fingerprint, ok)
	}

	for _, invalidDecimal := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if fingerprint, ok := runtimePatchCompiledPayloadFingerprint(runtimeFingerprintValueEntry(vm.Decimal(invalidDecimal))); ok || fingerprint != "" {
			t.Fatalf("non-finite decimal fingerprint = %q, ok %t", fingerprint, ok)
		}
	}
}

func TestRuntimePatchFingerprintStreamingABIAndCounters(t *testing.T) {
	if runtimePatchABI != "apextest-runtime-patch-v2" {
		t.Fatalf("runtime patch ABI = %q, want apextest-runtime-patch-v2", runtimePatchABI)
	}
	if testRuntimeCacheABI != "apextest-runtime-v6" {
		t.Fatalf("runtime cache ABI = %q, want apextest-runtime-v6", testRuntimeCacheABI)
	}
	counters := newRunPerfCounters(true)
	if _, ok := runtimePatchCompiledPayloadFingerprintWithPerf(runtimeFingerprintFixture(2), counters); !ok {
		t.Fatal("streaming fixture was rejected")
	}
	if got := snapshotPerfCounters(counters).RuntimeFingerprintBytes; got == 0 {
		t.Fatal("streaming fingerprint reported zero canonical bytes")
	}
}

func BenchmarkRuntimePatchCompiledPayloadFingerprint(b *testing.B) {
	entry := runtimeFingerprintFixture(512)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, ok := runtimePatchCompiledPayloadFingerprint(entry); !ok {
			b.Fatal("fingerprint rejected benchmark fixture")
		}
	}
}

func runtimeFingerprintFixture(size int) runtimeCacheEntry {
	payload := vm.Value{
		Kind:     vm.ValueObject,
		Type:     "Payload",
		Static:   "Example.State",
		Runtime:  "fixture",
		Ref:      42,
		Fields:   make(map[string]vm.Value, size),
		List:     make([]vm.Value, 0, size),
		Set:      make([]vm.Value, 0, size),
		Map:      make(map[string]vm.Value, size),
		MapKeys:  make(map[string]vm.Value, size),
		MapOrder: make([]string, 0, size),
	}
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("item-%04d", i)
		payload.Fields[key] = vm.String(key)
		payload.List = append(payload.List, vm.Int(int64(i)))
		payload.Set = append(payload.Set, vm.String(key))
		payload.Map[key] = vm.Decimal(float64(i) + 0.25)
		payload.MapKeys[key] = vm.String("key-" + key)
		payload.MapOrder = append(payload.MapOrder, key)
	}
	program := ir.Program{
		Source: "return 1;",
		Instructions: []ir.Instruction{{
			Op:   ir.OpReturn,
			Type: "Integer",
			Expr: ir.Expr{Kind: ir.ExprLiteral, Value: "1"},
			Pos:  1,
		}},
	}
	getter := vm.Method{Name: "Example.getPayload", ReturnType: "Object", Program: program, ClassName: "Example"}
	field := vm.Field{
		Name: "Payload", Type: "Object", Value: payload, InitialValue: vm.Null,
		Access: "public", Modifiers: []string{"transient"}, Property: true,
		Getter: &getter, HasSetter: true, File: "Example.cls", StorageName: "payload",
	}
	staticField := vm.Field{
		Name: "State", Type: "Integer", Static: true, Value: vm.Int(1), InitialValue: vm.Int(0),
		Access: "private", Modifiers: []string{"static"}, File: "Example.cls",
	}
	method := vm.Method{
		Name: "Example.run", ReturnType: "Integer", Params: []vm.Param{{Name: "input", Type: "String"}},
		Program: program, ClassName: "Example", IsStatic: true, Access: "public",
		Modifiers: []string{"static"}, File: "Example.cls", APIVersion: "65.0", Line: 2, Column: 3,
	}
	classes := []vm.Class{
		{
			Name: "Example", Namespace: "pkg", SuperClass: "Base", Interfaces: []string{"Runnable"},
			Fields: map[string]vm.Field{
				"Payload": field,
				"Other":   {Name: "Other", Type: "String", Value: vm.String("other"), InitialValue: vm.String("initial")},
			},
			StaticFields:     map[string]vm.Field{"State": staticField},
			FieldOrder:       []string{"Payload", "Other"},
			StaticFieldOrder: []string{"State"},
			Methods:          map[string]vm.Method{"Example.run": method},
			Constructors: []vm.Method{
				{Name: "Example", ClassName: "Example", Program: program},
				{Name: "Example.Second", ClassName: "Example", Program: program},
			},
			StaticInitializers: []vm.Method{{Name: "Example.<clinit>", ClassName: "Example", Program: program}},
			InstanceInitializers: []vm.Method{{
				Name: "Example.<init>", ClassName: "Example", Program: program,
			}},
			EnumValues: []string{"First", "Second"}, Access: "public", Modifiers: []string{"virtual"},
			IsAbstract: true, IsTest: true,
		},
		{Name: "Second", Fields: map[string]vm.Field{}, StaticFields: map[string]vm.Field{}, Methods: map[string]vm.Method{}},
	}
	return runtimeCacheEntry{
		Methods: map[string]vm.Method{
			"Example.run": method,
			"Second.run":  {Name: "Second.run", Program: program, ClassName: "Second"},
		},
		Classes: classes,
		Triggers: []vm.Trigger{
			{Name: "FirstTrigger", Namespace: "pkg", Object: "Account", Timing: "before", Operation: "insert", Program: program, File: "First.trigger", Line: 1, Column: 1},
			{Name: "SecondTrigger", Object: "Contact", Timing: "after", Operation: "update", Program: program, File: "Second.trigger", Line: 2, Column: 3},
		},
		TriggerErrors: []error{runtimeFingerprintErrorA("trigger failure")},
		PageNames:     []string{"FirstPage", "SecondPage"},
		BaseErr:       runtimeFingerprintErrorA("base failure"),
		restored:      vm.NewRestoredRuntimeTemplate(storage.NewOrgState(), vm.New(nil)),
	}
}

func runtimeFingerprintValueEntry(value vm.Value) runtimeCacheEntry {
	return runtimeCacheEntry{Classes: []vm.Class{{Fields: map[string]vm.Field{
		"Payload": {Name: "Payload", Value: value},
	}}}}
}

func requireRuntimePayloadFingerprint(t *testing.T, entry runtimeCacheEntry) string {
	t.Helper()
	fingerprint, ok := runtimePatchCompiledPayloadFingerprint(entry)
	if !ok || fingerprint == "" {
		t.Fatalf("fingerprint rejected entry: %q, ok %t", fingerprint, ok)
	}
	return fingerprint
}

func requireDifferentRuntimePayloadFingerprints(t *testing.T, first, second runtimeCacheEntry) {
	t.Helper()
	firstFingerprint := requireRuntimePayloadFingerprint(t, first)
	secondFingerprint := requireRuntimePayloadFingerprint(t, second)
	if firstFingerprint == secondFingerprint {
		t.Fatalf("behavior-affecting payloads share fingerprint %s", firstFingerprint)
	}
}

func cloneRuntimeFingerprintEntry(t *testing.T, entry runtimeCacheEntry) runtimeCacheEntry {
	t.Helper()
	cloned, ok := cloneRuntimeCacheEntryChecked(entry)
	if !ok {
		t.Fatal("fingerprint fixture clone failed")
	}
	return cloned
}

func reverseRuntimeFingerprintMethods(values map[string]vm.Method) map[string]vm.Method {
	if values == nil {
		return nil
	}
	out := make(map[string]vm.Method, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for i := len(keys) - 1; i >= 0; i-- {
		out[keys[i]] = values[keys[i]]
	}
	return out
}

func reverseRuntimeFingerprintFields(values map[string]vm.Field) map[string]vm.Field {
	if values == nil {
		return nil
	}
	out := make(map[string]vm.Field, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for i := len(keys) - 1; i >= 0; i-- {
		out[keys[i]] = values[keys[i]]
	}
	return out
}

func reverseRuntimeFingerprintValues(values map[string]vm.Value) map[string]vm.Value {
	if values == nil {
		return nil
	}
	out := make(map[string]vm.Value, len(values))
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	for i := len(keys) - 1; i >= 0; i-- {
		out[keys[i]] = values[keys[i]]
	}
	return out
}

var _ error = runtimeFingerprintErrorA("")
var _ error = runtimeFingerprintErrorB("")
