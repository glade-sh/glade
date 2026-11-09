package apextest

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

// (1): a subscriber @IsTest class that spells a package type without
// its namespace is a compile error, not a warning. Native rows
// namespace_subscriber_members S12 and S18 (anonymous and named): "Invalid
// type: V035Provider.GlobalInner" at API 62.0 and 67.0.
func TestNamespaceSubscriberIsTestUnqualifiedInnerIsError(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := namespaceSubscriberProject(t, api, api, map[string]string{
				"UnqualifiedProbeTest": "@IsTest private class UnqualifiedProbeTest {\n    @IsTest static void t() {\n        V035Provider.GlobalInner v = new V035Provider.GlobalInner();\n        System.assertEquals(2, v.size());\n    }\n    @IsTest static void u() {\n        gladens004.V035Provider.GlobalInner v = new gladens004.V035Provider.GlobalInner();\n        System.assertEquals(2, v.size());\n    }\n}\n",
			})
			var errors []diagnostic.Diagnostic
			for _, d := range sema.Analyze(index).Diagnostics {
				if strings.HasSuffix(d.File, "UnqualifiedProbeTest.cls") {
					errors = append(errors, d)
				}
			}
			if len(errors) == 0 {
				t.Fatal("no diagnostics for the unqualified package type")
			}
			for _, d := range errors {
				if d.Severity != diagnostic.Error {
					t.Errorf("%s %s %s is %s, want error", d.Code, nativeDiagnosticText(d), d.File, d.Severity)
				}
				if got := nativeDiagnosticText(d); got != "Invalid type: V035Provider.GlobalInner" {
					t.Errorf("message %q", got)
				}
			}
			run := Run(index, Options{SelectedClasses: []string{"UnqualifiedProbeTest"}, NoDiskCache: true, Parallelism: 1})
			if run.Summary().Passed != 0 {
				t.Errorf("run passed %+v; the class does not compile natively", run.Summary())
			}
		})
	}
}
