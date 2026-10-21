package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestSOQLStaticInitializerBindScopes(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		reject       bool
	}{
		{"scalar", `class InitializerBindProbe { static { String token='none'; Object rows=[SELECT Id FROM Account WHERE Name=:token]; } }`, false},
		{"collection", `class InitializerBindProbe { static { List<String> tokens=new List<String>{'none'}; for(Account row:[SELECT Id FROM Account WHERE Name IN :tokens]){} } }`, false},
		{"after loop", `class InitializerBindProbe { static { for(Account row:[SELECT Id FROM Account WHERE Id=null]){} List<String> tokens=new List<String>{'none'}; for(Account row:[SELECT Id FROM Account WHERE Name IN :tokens]){} List<String> aliases=new List<String>{'other'}; for(Account row:[SELECT Id FROM Account WHERE Name IN :aliases]){} } }`, false},
		{"nested block", `class InitializerBindProbe { static { { Set<Id> ids=new Set<Id>(); Object rows=[SELECT Id FROM Account WHERE Id IN :ids]; } } }`, false},
		{"static field", `class InitializerBindProbe { static String token='none'; static { Object rows=[SELECT Id FROM Account WHERE Name=:token]; } }`, false},
		{"commented header", `class InitializerBindProbe { static /* scope marker */ { String token='none'; Object rows=[SELECT Id FROM Account WHERE Name=:token]; } }`, false},
		{"later declaration", `class InitializerBindProbe { static { Object rows=[SELECT Id FROM Account WHERE Name=:token]; String token='none'; } }`, true},
		{"closed block", `class InitializerBindProbe { static { { String token='none'; } Object rows=[SELECT Id FROM Account WHERE Name=:token]; } }`, true},
		{"sibling initializer", `class InitializerBindProbe { static { String token='none'; } static { Object rows=[SELECT Id FROM Account WHERE Name=:token]; } }`, true},
		{"instance field", `class InitializerBindProbe { String token='none'; static { Object rows=[SELECT Id FROM Account WHERE Name=:token]; } }`, true},
		{"method local outside initializer", `class InitializerBindProbe { static { Object rows=[SELECT Id FROM Account WHERE Name=:token]; } static void run(){String token='none';} }`, true},
		{"initializer local outside method", `class InitializerBindProbe { static { String token='none'; } static void run(){ Object rows=[SELECT Id FROM Account WHERE Name=:token]; } }`, true},
		{"method control", `class InitializerBindProbe { static void run(){String token='none'; Object rows=[SELECT Id FROM Account WHERE Name=:token];} }`, false},
		{"instance method control", `class InitializerBindProbe { String token='none'; void run(){Object rows=[SELECT Id FROM Account WHERE Name=:token];} }`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("InitializerBindProbe.cls", tc.source)
			if got := hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND"); got != tc.reject {
				t.Fatalf("query bind rejected=%v, want %v: %#v", got, tc.reject, diagnostics)
			}
		})
	}
}
