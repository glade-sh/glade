package sema

import "testing"

// Apex Properties and Extending a Class define inherited getter visibility:
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_apex_developer_guide.pdf
func TestSOQLBindInheritedProperties(t *testing.T) {
	for _, test := range []struct {
		name, property, body string
		constructor          bool
	}{
		{
			name:     "public property with protected setter",
			property: "public SObject selected { get; protected set; }",
			body:     "List<Account> rows = [SELECT Id FROM Account WHERE Id = :selected.Id];",
		},
		{
			name:     "protected property",
			property: "protected String queryName { get; set; }",
			body:     "List<Account> rows = [SELECT Id FROM Account WHERE Name = :queryName];",
		},
		{
			name:     "protected getter",
			property: "public String queryName { protected get; set; }",
			body:     "List<Account> rows = [SELECT Id FROM Account WHERE Name = :queryName];",
		},
		{
			name:        "constructor property bind",
			property:    "public SObject selected { get; protected set; }",
			body:        "List<Account> rows = [SELECT Id FROM Account WHERE Id = :selected.Id];",
			constructor: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			header := "public void inspect()"
			if test.constructor {
				header = "public PropertyConsumer()"
			}
			result := analyzeDeclarationProject(t, map[string]string{
				"PropertyBase.cls":     "public virtual class PropertyBase { " + test.property + " }",
				"PropertyConsumer.cls": "public class PropertyConsumer extends PropertyBase { " + header + " { " + test.body + " } }",
			})
			if result.HasErrors() {
				t.Fatalf("inherited property bind rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestSOQLBindInheritedPropertyControls(t *testing.T) {
	for _, test := range []struct {
		name, property, methodModifier string
		wantReject                     bool
	}{
		{"private property", "private String queryName { get; set; }", "", true},
		{"private getter", "public String queryName { private get; set; }", "", true},
		{"setter only", "public String queryName { set; }", "", true},
		{"instance property in static context", "public String queryName { get; set; }", "static", true},
		{"static field in static context", "public static String queryName;", "static", false},
		{"inherited field", "protected String queryName;", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"PropertyBase.cls": "public virtual class PropertyBase { " + test.property + " }",
				"PropertyConsumer.cls": "public class PropertyConsumer extends PropertyBase { public " + test.methodModifier +
					" void inspect() { List<Account> rows = [SELECT Id FROM Account WHERE Name = :queryName]; } }",
			})
			if got := hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_BIND"); got != test.wantReject {
				t.Fatalf("bind rejection=%v, want %v: %#v", got, test.wantReject, result.Diagnostics)
			}
		})
	}
}
