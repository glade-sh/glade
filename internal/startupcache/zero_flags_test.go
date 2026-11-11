package startupcache

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Every pointer-to-scalar reachable from the split payload must be covered by
// the zero-flag side table, because gob decodes a pointer to a zero value as
// nil. Today storage.Field holds all of them.
func TestZeroFieldFlagsCoverEveryPointerScalar(t *testing.T) {
	fieldType := reflect.TypeOf(storage.Field{})
	var outside []string
	onStack := map[reflect.Type]bool{}
	var walk func(rt reflect.Type, path string)
	walk = func(rt reflect.Type, path string) {
		switch rt.Kind() {
		case reflect.Pointer:
			if elem := rt.Elem(); elem.Kind() <= reflect.Complex128 || elem.Kind() == reflect.String {
				// Label sources are restored by EmptyLabelSources.
				if !onStack[fieldType] && path != "payload.Org.Metadata.Labels[].TranslationSourceRaw" {
					outside = append(outside, path)
				}
				return
			}
			walk(rt.Elem(), path)
		case reflect.Slice, reflect.Array:
			walk(rt.Elem(), path+"[]")
		case reflect.Map:
			walk(rt.Key(), path+"{key}")
			walk(rt.Elem(), path+"{}")
		case reflect.Struct:
			if onStack[rt] {
				return
			}
			onStack[rt] = true
			for i := 0; i < rt.NumField(); i++ {
				if field := rt.Field(i); field.IsExported() {
					walk(field.Type, path+"."+field.Name)
				}
			}
			onStack[rt] = false
		}
	}
	walk(reflect.TypeOf(testCachePayload{}), "payload")
	if len(outside) != 0 {
		t.Fatalf("pointer scalars outside storage.Field are not restored after gob decode: %v", outside)
	}

	// Each storage.Field pointer scalar has exactly one side-table bit.
	var boolPointers, intPointers []string
	for i := 0; i < fieldType.NumField(); i++ {
		field := fieldType.Field(i)
		if field.Type.Kind() != reflect.Pointer {
			continue
		}
		switch field.Type.Elem().Kind() {
		case reflect.Bool:
			boolPointers = append(boolPointers, field.Name)
		case reflect.Struct, reflect.Slice, reflect.Map:
		default:
			intPointers = append(intPointers, field.Name)
		}
	}
	if len(boolPointers) != len(fieldBoolFlags) || !reflect.DeepEqual(intPointers, []string{"RelationshipOrder"}) {
		t.Fatalf("storage.Field pointer flags bool=%v other=%v; side table covers %d bools and RelationshipOrder", boolPointers, intPointers, len(fieldBoolFlags))
	}
	var probe storage.Field
	seen := map[uintptr]bool{}
	for _, flag := range fieldBoolFlags {
		address := reflect.ValueOf(flag(&probe)).Pointer()
		if seen[address] {
			t.Fatal("fieldBoolFlags lists one field twice")
		}
		seen[address] = true
	}
}

// Pointers to zero values, pointers to non-zero values and nil pointers all
// keep their state through the split test cache.
func TestSplitCachePreservesZeroFieldFlags(t *testing.T) {
	root := t.TempDir()
	writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[],"sourceApiVersion":"62.0"}`)
	proof, err := ValidateInputWithSourceDigests(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	set := func(field *storage.Field, value bool) {
		v := reflect.ValueOf(field).Elem()
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).Kind() != reflect.Pointer {
				continue
			}
			switch elem := v.Field(i).Type().Elem(); elem.Kind() {
			case reflect.Bool:
				p := reflect.New(elem)
				p.Elem().SetBool(value)
				v.Field(i).Set(p)
			case reflect.Int:
				p := reflect.New(elem)
				if value {
					p.Elem().SetInt(1)
				}
				v.Field(i).Set(p)
			}
		}
	}
	zero := storage.Field{APIName: "Zero__c", Type: storage.FieldBoolean, Required: true}
	set(&zero, false)
	nonZero := storage.Field{APIName: "Set__c", Type: storage.FieldBoolean}
	set(&nonZero, true)
	unset := storage.Field{APIName: "Unset__c", Type: storage.FieldBoolean}
	org := storage.NewOrgState()
	org.Objects["Flags__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Flags__c", Fields: map[string]storage.Field{
		"Zero__c": zero, "Set__c": nonZero, "Unset__c": unset,
	}}}
	entry, err := NewEntryWithValidatedInput(proof, org, CompiledRuntime{})
	if err != nil {
		t.Fatal(err)
	}
	entry.RuntimeABI = "zero-flags-test"
	entry.RuntimeKey = "zero-flags-key"
	if _, err := WriteWithStats(&entry, SubdirTest); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFreshRuntimeWithValidatedInput(root, SubdirTest, Version, entry.RuntimeABI, entry.RuntimeKey, proof)
	if err != nil || got == nil {
		t.Fatalf("fresh cache=%v err=%v", got, err)
	}
	fields := got.Org.Objects["Flags__c"].Definition.Fields
	for name, want := range map[string]storage.Field{"Zero__c": zero, "Set__c": nonZero, "Unset__c": unset} {
		if !reflect.DeepEqual(fields[name], want) {
			t.Errorf("%s restored as %#v, want %#v", name, fields[name], want)
		}
	}
	if t.Failed() {
		return
	}
	// Restored flags must not alias: a write through one leaves the others.
	restored := fields["Zero__c"]
	*restored.Nillable = true
	if *restored.Createable || *restored.DefaultedOnCreate {
		t.Fatal("restored zero flags share storage")
	}
}
