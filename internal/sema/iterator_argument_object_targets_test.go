package sema

import (
	"fmt"
	"testing"
)

func TestIteratorObjectParameterDeclaredArguments(t *testing.T) {
	for _, version := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, elementType, element string
		}{
			{"string", "String", "'Name'"},
			{"field", "Schema.SObjectField", "Account.Name"},
			{"object-control", "Object", "'Name'"},
		} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
					"ArgumentReceiver.cls": iteratorObjectArgumentReceiverSource,
					"ArgumentUse.cls": fmt.Sprintf(`public class ArgumentUse {
  public static void run() {
    Iterator<%s> chained = new List<%s>{%s}.iterator();
    String selected = new ArgumentReceiver().add(chained).value();
    Iterator<%s> staticValues = new List<%s>{%s}.iterator();
    Object staticValue = ArgumentReceiver.staticHead(staticValues);
    Iterator<%s> instanceValues = new List<%s>{%s}.iterator();
    Object instanceValue = new ArgumentReceiver().instanceHead(instanceValues);
  }
}`, tc.elementType, tc.elementType, tc.element, tc.elementType, tc.elementType, tc.element, tc.elementType, tc.elementType, tc.element),
				}, version)
				if result.HasErrors() {
					t.Fatalf("declared iterator argument rejected: %#v", result.Diagnostics)
				}
			})
		}
		t.Run(version+"/neighbor-overload-controls", func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"ArgumentReceiver.cls": iteratorObjectArgumentReceiverSource,
				"ArgumentUse.cls": `public class ArgumentUse {
  public static void run() {
    ArgumentReceiver receiver = new ArgumentReceiver();
    String fromListString = receiver.add(new List<String>{'Name'}).value();
    String fromSetString = receiver.add(new Set<String>{'Name'}).value();
    String fromListField = receiver.add(new List<Schema.SObjectField>{Account.Name}).value();
    String fromSetField = receiver.add(new Set<Schema.SObjectField>{Account.Name}).value();
    Object direct = ArgumentReceiver.staticHead(new List<Object>{'Name'}.iterator());
  }
}`,
			}, version)
			if result.HasErrors() {
				t.Fatalf("existing overload controls rejected: %#v", result.Diagnostics)
			}
		})
	}
}

const iteratorObjectArgumentReceiverSource = `public class ArgumentReceiver {
  public ArgumentReceiver add(List<String> values) { return this; }
  public ArgumentReceiver add(Set<String> values) { return this; }
  public ArgumentReceiver add(List<Schema.SObjectField> values) { return this; }
  public ArgumentReceiver add(Set<Schema.SObjectField> values) { return this; }
  public ArgumentReceiver add(Iterator<Object> values) { return this; }
  public String value() { return 'selected'; }
  public static Object staticHead(Iterator<Object> values) { return values.next(); }
  public Object instanceHead(Iterator<Object> values) { return values.next(); }
}`
