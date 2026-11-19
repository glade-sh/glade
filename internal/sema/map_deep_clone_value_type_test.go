package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// review_round11 dc_typed_self, dc_typed_put, dc_typed_remove, dc_typed_clear,
// dc_chained_self, dc_chained_put, dc_chained_remove, dc_chained_clear,
// dc_int, dc_object, dc_list_value, dc_nested_map,
// dc_nested_sobject_list_map and dc_fieldmap (route anonymous, APIs 62.0 and
// 67.0, compiled=false) refuse Map.deepClone at compile with the captured
// message on every map whose declared value type is not an SObject, while
// dc_sobject and dc_account compile and run. Glade raised the refusal only
// when the call ran (vm.mapDeepCloneValueTypeError); sema now rejects the
// statically typed call at compile, and the run-time check keeps receivers
// whose static value type is unknown.
func TestMapDeepCloneSObjectValueMapCompileRefusal(t *testing.T) {
	refusals := []struct{ source, message string }{
		{
			"Map<String,Integer> m = new Map<String,Integer>{'a' => 1}; Map<String,Integer> d = m.deepClone();",
			"Operation only applies to SObject value map types: Map<String,Integer>",
		},
		{
			"Map<String,Object> m = new Map<String,Object>{'a' => 1}; Map<String,Object> d = m.deepClone();",
			"Operation only applies to SObject value map types: Map<String,Object>",
		},
		{
			"Map<String,Exception> m = new Map<String,Exception>(); Map<String,Exception> d = m.deepClone();",
			"Operation only applies to SObject value map types: Map<String,Exception>",
		},
		{
			"Map<String,List<Account>> m = new Map<String,List<Account>>{'a' => new List<Account>()}; Map<String,List<Account>> d = m.deepClone();",
			"Operation only applies to SObject value map types: Map<String,List<Account>>",
		},
		{
			"Map<String,Map<String,Account>> m = new Map<String,Map<String,Account>>{'a' => new Map<String,Account>()}; Map<String,Map<String,Account>> d = m.deepClone();",
			"Operation only applies to SObject value map types: Map<String,Map<String,Account>>",
		},
		{
			"Map<String,Map<Id,SObject>> m = new Map<String,Map<Id,SObject>>{'a' => new Map<Id,SObject>()}; Map<String,Map<Id,SObject>> d = m.deepClone();",
			"Operation only applies to SObject value map types: Map<String,Map<Id,SObject>>",
		},
		{
			"Map<String,Schema.SObjectType> gd = Schema.getGlobalDescribe(); Integer n = gd.size(); Map<String,Schema.SObjectType> d = gd.deepClone();",
			"Operation only applies to SObject value map types: Map<String,Schema.SObjectType>",
		},
		{
			"Integer n = Schema.getGlobalDescribe().size(); Map<String,Schema.SObjectType> d = Schema.getGlobalDescribe().deepClone();",
			"Operation only applies to SObject value map types: Map<String,Schema.SObjectType>",
		},
		{
			"Map<String,Schema.SObjectField> m = Schema.SObjectType.Account.fields.getMap(); Map<String,Schema.SObjectField> d = m.deepClone();",
			"Operation only applies to SObject value map types: Map<String,Schema.SObjectField>",
		},
		// A cast makes the value type statically known the same way a
		// declaration does.
		{
			"Object o = new Map<String,Integer>(); Map<String,Integer> d = ((Map<String,Integer>)o).deepClone();",
			"Operation only applies to SObject value map types: Map<String,Integer>",
		},
		// The declared spelling keeps the captured no-space type rendering.
		{
			"Map<String, Integer> m = new Map<String, Integer>(); Map<String, Integer> d = m.deepClone();",
			"Operation only applies to SObject value map types: Map<String,Integer>",
		},
	}
	accepts := []struct{ label, source string }{
		{"generic SObject", "Map<String,SObject> m = new Map<String,SObject>{'a' => new Account(Name = 'x')}; Map<String,SObject> d = m.deepClone();"},
		{"Id keyed SObject", "Map<Id,SObject> m = new Map<Id,SObject>(); Map<Id,SObject> d = m.deepClone();"},
		{"standard object", "Map<Id,Account> m = new Map<Id,Account>(); Map<Id,Account> d = m.deepClone();"},
		{"string keyed standard object", "Map<String,Account> m = new Map<String,Account>(); Map<String,Account> d = m.deepClone();"},
		{"org custom object", "Map<Id,Widget__c> m = new Map<Id,Widget__c>(); Map<Id,Widget__c> d = m.deepClone();"},
		{"namespaced custom object", "Map<Id,ns__Parcel__c> m = new Map<Id,ns__Parcel__c>(); Map<Id,ns__Parcel__c> d = m.deepClone();"},
		{"Schema prefixed object", "Map<String,Schema.Account> m = new Map<String,Schema.Account>(); Map<String,Schema.Account> d = m.deepClone();"},
		{"trigger map", "Map<Id,SObject> m = Trigger.newMap; Map<Id,SObject> d = m.deepClone();"},
		// A receiver without a declared value type is not statically known;
		// its refusal stays with the run-time check.
		{"raw map", "Map m = new Map(); Object d = m.deepClone();"},
	}
	shape := schema.Schema{Objects: []schema.Object{{Name: "Widget__c"}, {Name: "ns__Parcel__c"}}}
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Build(project.Project{Root: t.TempDir(), SourceAPIVersion: api}, shape)
		for _, refusal := range refusals {
			t.Run(api+" refuse "+refusal.message, func(t *testing.T) {
				result := AnalyzeAnonymous(index, refusal.source, api)
				if len(result.Diagnostics) != 1 || result.Diagnostics[0].Message != refusal.message {
					t.Fatalf("diagnostics=%+v, want only %q", result.Diagnostics, refusal.message)
				}
			})
		}
		for _, accept := range accepts {
			t.Run(api+" accept "+accept.label, func(t *testing.T) {
				if result := AnalyzeAnonymous(index, accept.source, api); result.HasErrors() {
					t.Fatalf("SObject value map rejected: %+v", result.Diagnostics)
				}
			})
		}
	}
}

// The named-class route runs the IR and the text call checkers over the same
// call, and a project class is not an SObject value either: each refusal must
// appear once, not once per checker.
func TestMapDeepCloneSObjectValueMapClassRouteRefusedOnce(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "DeepCloneProbe.cls"), `
public class DeepCloneProbe {
  public void run() {
    Map<String,Integer> m = new Map<String,Integer>{'a' => 1};
    Map<String,Integer> d = m.deepClone();
    Map<String,DeepCloneProbe> p = new Map<String,DeepCloneProbe>();
    Map<String,DeepCloneProbe> q = p.deepClone();
  }
}
`)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{filepath.Join(root, "DeepCloneProbe.cls")}, SourceAPIVersion: "62.0"}, schema.Schema{})
	result := Analyze(index)
	for _, message := range []string{
		"Operation only applies to SObject value map types: Map<String,Integer>",
		"Operation only applies to SObject value map types: Map<String,DeepCloneProbe>",
	} {
		count := 0
		for _, d := range result.Diagnostics {
			if d.Message == message {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("%q appears %d times in %+v", message, count, result.Diagnostics)
		}
	}
}
