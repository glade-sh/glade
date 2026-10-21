package sema

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestSOQLBindLexicalQuotedColons(t *testing.T) {
	// The captured rows and rejection twins 013/015, both routes,
	// same-release APIs 59/62/67/68.
	for _, literal := range []string{
		`'Approve:Thing'`, `'a:%:%:_%:'`, `'a\'b:undefinedVar'`, `'%a\'b:undefinedVar%'`,
	} {
		source := `public class BindProbe { public void run() { List<Account> rows = [SELECT Id FROM Account WHERE Name = ` + literal + `]; } }`
		diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("BindProbe.cls", source)
		if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
			t.Fatalf("literal %s diagnosed as bind: %#v", literal, diagnostics)
		}
	}
	source := `public class BindProbe { public void run() { List<Account> rows = [SELECT Id FROM Account WHERE Name = :undefinedVar]; } }`
	if diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("BindProbe.cls", source); !hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("undefined bind lost its rejection: %#v", diagnostics)
	}
}

func TestSOQLBindLexicalDeclarationShapes(t *testing.T) {
	// The captured rows. LX006/016/026 retain their captured
	// comment/string controls; LX054/056 retain visibility boundaries.
	for _, test := range []struct {
		name, source, binding, want string
	}{
		{"uppercase-field", `public class BindProbe { PRIVATE FINAL STATIC STRING EMAIL='probe@example.com'; public static void run(){ List<User> rows=[SELECT Id FROM User WHERE Email=:EMAIL]; } }`, "email", "STRING"},
		{"compact-generic", `List<ID>ids=new list<id>(); List<Account> rows=[SELECT Id FROM Account WHERE Id IN :ids];`, "ids", "List<ID>"},
		{"block-comment", `/* :undefinedVar the token */ String token='Acme'; List<Account> rows=[SELECT Id FROM Account WHERE Name=:token];`, "token", "String"},
		{"return-parameter", `public class BindProbe { public static List<Account> run(Set<String> names){return names==null?new List<Account>():[SELECT Id FROM Account WHERE Name=:names];} }`, "names", "Set<String>"},
		{"line-comment", "// :undefinedVar and the token\nString token='Acme'; List<Account> rows=[SELECT Id FROM Account WHERE Name=:token];", "token", "String"},
		{"string-control", `String text='Integer token; :undefinedVar'; String token='Acme'; List<Account> rows=[SELECT Id FROM Account WHERE Name=:token];`, "token", "String"},
		{"out-of-scope", `{String token='Acme';} List<Account> rows=[SELECT Id FROM Account WHERE Name=:token];`, "token", ""},
		{"declared-later", `List<Account> rows=[SELECT Id FROM Account WHERE Name=:token]; String token='Acme';`, "token", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			offset := strings.LastIndex(test.source, ":"+test.binding)
			if test.binding == "email" {
				offset = strings.LastIndex(test.source, ":EMAIL")
			}
			bindings := newSemaBindingResolver(test.source, newSemaCodeSpans(test.source)).bindingsAt(offset)
			if got := bindings[test.binding]; got != test.want {
				t.Fatalf("%s binding = %q, want %q; all=%#v", test.binding, got, test.want, bindings)
			}
		})
	}
}

func TestSOQLBindLexicalMultipleDeclarators(t *testing.T) {
	// The captured rows capture these comma-separated String locals on both
	// routes at APIs 62/67/68. The named route previously missed bName.
	for _, wrapper := range []string{"%s", "@IsTest private class BindProbe { @IsTest static void observe(){%s} }"} {
		body := `String aName='Aster',bName='Birch',cName='Cedar'; List<Account> rows=[SELECT Id FROM Account WHERE Name=:aName OR Name=:bName];`
		source := strings.Replace(wrapper, "%s", body, 1)
		offset := strings.Index(source, ":bName")
		bindings := newSemaBindingResolver(source, newSemaCodeSpans(source)).bindingsAt(offset)
		for _, name := range []string{"aname", "bname", "cname"} {
			if got := bindings[name]; got != "String" {
				t.Fatalf("%s binding=%q, want String; all=%#v", name, got, bindings)
			}
		}
	}
}

func TestSOQLBindLexicalInheritedFields(t *testing.T) {
	// The captured rows named captures; the captured rows preserve field/local
	// precedence, and 039/041/043 preserve undefined-bind rejection.
	for _, test := range []struct {
		name, field, body string
	}{
		{"protected-set", "protected Set<Id> ids=new Set<Id>();", `return [SELECT Id FROM Account WHERE Id IN :ids].size();`},
		{"protected-string", "protected String name='missing';", `return [SELECT Id FROM Account WHERE Name=:name].size();`},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"BindBase.cls":  `public virtual class BindBase { ` + test.field + ` }`,
				"BindProbe.cls": `public class BindProbe extends BindBase { public Integer run(){` + test.body + `} }`,
			})
			if hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_BIND") {
				t.Fatalf("inherited field bind rejected: %#v", result.Diagnostics)
			}
		})
	}
	result := analyzeDeclarationProject(t, map[string]string{
		"BindBase.cls":  `public virtual class BindBase { protected String name='missing'; }`,
		"BindProbe.cls": `public class BindProbe extends BindBase { public Integer run(){return [SELECT Id FROM Account WHERE Name=:undefinedVar].size();} }`,
	})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("undefined inherited-context bind accepted: %#v", result.Diagnostics)
	}
}

func TestSOQLBindLexicalStaticContext(t *testing.T) {
	// The captured row rejects instance fields from static methods. Named
	// The captured rows and instance LX052 remain accepted controls.
	for _, test := range []struct {
		name, fieldModifier, methodModifier string
		wantReject                          bool
	}{
		{"instance-from-static", "", "static", true},
		{"static-from-instance", "static", "", false},
		{"static-from-static", "static", "static", false},
		{"instance-from-instance", "", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := `public class BindProbe { ` + test.fieldModifier + ` String name='Acme'; public ` + test.methodModifier + ` Integer run(){return [SELECT Id FROM Account WHERE Name=:name].size();} }`
			diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("BindProbe.cls", source)
			if got := hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND"); got != test.wantReject {
				t.Fatalf("bind rejection=%v, want %v; diagnostics=%#v", got, test.wantReject, diagnostics)
			}
		})
	}
}
