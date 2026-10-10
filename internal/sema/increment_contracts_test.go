package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func requireIncrementDiagnostic(t *testing.T, diagnostics []diagnostic.Diagnostic, code, message string) {
	t.Helper()
	for _, d := range diagnostics {
		if d.Code == code && strings.Contains(d.Message, message) {
			return
		}
	}
	t.Fatalf("want %s %q, got %#v", code, message, diagnostics)
}

func requireNoIncrementErrors(t *testing.T, result Result) {
	t.Helper()
	if result.HasErrors() {
		t.Fatalf("want no errors, got %#v", result.Diagnostics)
	}
}

func TestIncrementFixPreservesNativeNumericMethodRejections(t *testing.T) {
	// Double/Decimal F04, E05, K10, K11, captured at API 62 and 67.
	for _, body := range []string{
		`Double value = 1.0; value.abs();`,
		`Double value = 1.0; Decimal other = 1.0; value.compareTo(other);`,
		`Double value = 1.0; value.setScale(2);`,
		`Double value = 1.0; value.doubleValue();`,
	} {
		result := analyzeDeclarationProject(t, map[string]string{
			"Widget.cls": "public class Widget { public void run() {" + body + "} }",
		})
		requireIncrementDiagnostic(t, result.Diagnostics, "GLADESEMA023", "invalid collection call")
	}
}

func TestIncrementUncapturedReceiversKeepCanonicalDiagnostics(t *testing.T) {
	// These are local preservation checks, not Salesforce observations.
	for name, body := range map[string]string{
		"property receiver":      `Box b = new Box(); Decimal old = b.values[0]++;`,
		"method receiver":        `Box b = new Box(); Decimal old = b.getValues()[0]++;`,
		"nested list":            `List<List<Decimal>> l = new List<List<Decimal>>{new List<Decimal>{2}}; Decimal old = l[0][0]++;`,
		"literal":                `Decimal old = 2.0++;`,
		"own property":           `Decimal old = values[0]++;`,
		"own scalar property":    `Decimal old = count++;`,
		"static property":        `Decimal old = Box.shared[0]++;`,
		"static scalar property": `Decimal old = Box.sharedCount++;`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"Box.cls": `public class Box {
 public List<Decimal> values { get { return new List<Decimal>{2}; } }
 public List<Decimal> getValues() { return new List<Decimal>{2}; }
 public static List<Decimal> shared { get { return new List<Decimal>{2}; } }
 public static Decimal sharedCount { get { return 2; } set; }
}`,
				"Widget.cls": "public class Widget { public List<Decimal> values { get { return new List<Decimal>{2}; } } public Decimal count { get { return 2; } set; } public void run() {" + body + "} }",
			})
			requireIncrementDiagnostic(t, result.Diagnostics, "GLADESEMA023", `invalid collection call "__postfix:++"`)
		})
	}
}

// TestIncrementUncapturedShapesKeepCanonicalTwins pins the existing behavior
// for shapes outside the captured collection-increment rows, each beside its
// captured twin.
// These are local preservation checks, not Salesforce observations.
func TestIncrementUncapturedShapesKeepCanonicalTwins(t *testing.T) {
	const invalidCall = `invalid collection call "__postfix:++"`
	const list = "List<Decimal> l = new List<Decimal>{2};\n"
	const nested = "List<List<Decimal>> l = new List<List<Decimal>>{new List<Decimal>{2}};\n"
	for _, api := range []string{"62.0", "67.0"} {
		named := func(t *testing.T, files map[string]string) Result {
			t.Helper()
			return analyzeDeclarationProjectWithAPIVersion(t, files, api)
		}
		check := func(t *testing.T, body string) Result {
			t.Helper()
			return named(t, map[string]string{"Probe.cls": "public class Probe {\n public static void check() {\n" + body + " }\n}\n"})
		}
		anonymous := func(body string) Result {
			return AnalyzeAnonymous(typesys.Index{}, body, api)
		}
		t.Run(api+"/anonymous consumed list element", func(t *testing.T) {
			// No anonymous compile row: the consumed result keeps the canonical rejection.
			requireIncrementDiagnostic(t, anonymous(list+"Decimal observed = l[0]++;").Diagnostics, "GLADESEMA023", invalidCall)
			// R017: the anonymous statement form compiles.
			requireNoIncrementErrors(t, anonymous(list+"l[0]++;"))
			// C017: the consumed result compiles in a named method.
			requireNoIncrementErrors(t, check(t, list+"Decimal observed = l[0]++;"))
		})
		t.Run(api+"/own field", func(t *testing.T) {
			for _, operand := range []string{"f", "this.f"} {
				result := named(t, map[string]string{
					"Box.cls": "public class Box {\n public Decimal f = 2;\n public void run() {\n Decimal observed = " + operand + "++;\n }\n}\n",
				})
				requireIncrementDiagnostic(t, result.Diagnostics, "GLADESEMA023", invalidCall)
			}
			// C105: the same field through a local receiver compiles.
			requireNoIncrementErrors(t, named(t, map[string]string{
				"Box.cls":   "public class Box {\n public Decimal f = 2;\n}\n",
				"Probe.cls": "public class Probe {\n public static void check() {\n Box a = new Box();\n Decimal observed = a.f++;\n }\n}\n",
			}))
		})
		t.Run(api+"/prefix statement parse", func(t *testing.T) {
			// A nested index is not a captured statement: canonical leaves the
			// named body unlowered and rejects the anonymous body.
			result := check(t, nested+"++l[0][0];\n")
			requireNoIncrementErrors(t, result)
			if !hasDiagnosticCode(result.Diagnostics, runtimeLoweringDiagnosticCode) {
				t.Fatalf("want %s, got %#v", runtimeLoweringDiagnosticCode, result.Diagnostics)
			}
			requireIncrementDiagnostic(t, anonymous(nested+"++l[0][0];").Diagnostics, "GLADESEMA_ANONYMOUS_PARSE", "")
			// C171 and R018: ++l[0]; parses on both routes.
			result = check(t, list+"++l[0];\n")
			requireNoIncrementErrors(t, result)
			if hasDiagnosticCode(result.Diagnostics, runtimeLoweringDiagnosticCode) {
				t.Fatalf("captured statement was not lowered: %#v", result.Diagnostics)
			}
			requireNoIncrementErrors(t, anonymous(list+"++l[0];"))
		})
		t.Run(api+"/uncaptured fallback statement", func(t *testing.T) {
			// The VM lowers these spellings, but no row captures the element
			// type or the named operator: each body keeps its name-path result.
			const dates = "List<Date> l = new List<Date>{Date.today()};\n"
			const accounts = "Map<String,Account> m = new Map<String,Account>{'one' => new Account(AnnualRevenue = 2)};\n"
			for _, body := range []string{dates + "++l[0];\n", accounts + "--m.get('one').AnnualRevenue;\n"} {
				result := check(t, body)
				requireNoIncrementErrors(t, result)
				if !hasDiagnosticCode(result.Diagnostics, runtimeLoweringDiagnosticCode) || len(result.Diagnostics) != 1 {
					t.Fatalf("want only %s, got %#v", runtimeLoweringDiagnosticCode, result.Diagnostics)
				}
			}
			requireIncrementDiagnostic(t, anonymous(dates+"++l[0];").Diagnostics, "GLADESEMA_ANONYMOUS_PARSE", "expected ; at byte")
			// R4-04: the anonymous form of the named statement compiles.
			requireNoIncrementErrors(t, anonymous(accounts+"--m.get('one').AnnualRevenue;"))
			// R4-16 and R4-01: the captured named operators compile.
			result := check(t, accounts+"++m.get('one').AnnualRevenue;\nm.get('one').AnnualRevenue++;\n")
			requireNoIncrementErrors(t, result)
			if hasDiagnosticCode(result.Diagnostics, runtimeLoweringDiagnosticCode) {
				t.Fatalf("captured statement was not lowered: %#v", result.Diagnostics)
			}
			// No named row holds a literal-key postfix decrement: it keeps the
			// canonical call diagnostic.
			requireIncrementDiagnostic(t, check(t, accounts+"m.get('one').AnnualRevenue--;\n").Diagnostics, "GLADESEMA023", `invalid collection call "__postfix:--"`)
			// No @IsTest row holds a local-key pre-increment or a field-path
			// key prefix statement: the named body is not lowered.
			for _, body := range []string{"String key = 'one';\n++m.get(key).AnnualRevenue;\n", "Account acc = new Account(Name = 'one');\n++m.get(acc.Name).AnnualRevenue;\n"} {
				result := check(t, accounts+body)
				if !hasDiagnosticCode(result.Diagnostics, runtimeLoweringDiagnosticCode) || len(result.Diagnostics) != 1 {
					t.Fatalf("want only %s, got %#v", runtimeLoweringDiagnosticCode, result.Diagnostics)
				}
			}
		})
		t.Run(api+"/comments after the prefix operator", func(t *testing.T) {
			// Comments between tokens change neither the lowered spellings nor
			// the result an uncaptured body keeps.
			const dates = "List<Date> l = new List<Date>{Date.today()};\n"
			const accounts = "Map<String,Account> m = new Map<String,Account>{'one' => new Account(AnnualRevenue = 2)};\n"
			for _, body := range []string{
				accounts + "--/*c*/m.get('one').AnnualRevenue;\n",
				dates + "++/*c*/l[0];\n",
				dates + "++ // c\n l[0];\n",
			} {
				result := check(t, body)
				if !hasDiagnosticCode(result.Diagnostics, runtimeLoweringDiagnosticCode) || len(result.Diagnostics) != 1 {
					t.Fatalf("want only %s, got %#v", runtimeLoweringDiagnosticCode, result.Diagnostics)
				}
			}
			requireIncrementDiagnostic(t, anonymous(dates+"--/* c */l[0];").Diagnostics, "GLADESEMA_ANONYMOUS_PARSE", "expected ; at byte")
			// C171 and R4-04: the captured spellings still lower.
			result := check(t, list+"++/*c*/l[0];\n")
			requireNoIncrementErrors(t, result)
			if hasDiagnosticCode(result.Diagnostics, runtimeLoweringDiagnosticCode) {
				t.Fatalf("captured statement was not lowered: %#v", result.Diagnostics)
			}
			requireNoIncrementErrors(t, anonymous(accounts+"--/*c*/m.get('one').AnnualRevenue;"))
		})
		t.Run(api+"/map key read through a property", func(t *testing.T) {
			const box = "public class Box {\n public String key { get { return 'one'; } }\n public String plain = 'one';\n}\n"
			const accounts = "Map<String,Account> m = new Map<String,Account>{'one' => new Account(AnnualRevenue = 2)};\n"
			const ids = "Map<Id,Account> m = new Map<Id,Account>{'001000000000001' => new Account(AnnualRevenue = 2)};\n"
			probe := func(body string) Result {
				return named(t, map[string]string{"Box.cls": box, "Probe.cls": "public class Probe {\n public static void check() {\n" + body + " }\n}\n"})
			}
			// R4-22: a key read from a field of a local record compiles.
			requireNoIncrementErrors(t, probe(ids+"Account acc = new Account(Id = '001000000000001');\nm.get(acc.Id).AnnualRevenue++;\n"))
			// No row reads the key through a property or a class field.
			for _, key := range []string{"b.key", "b.plain"} {
				requireIncrementDiagnostic(t, probe(accounts+"Box b = new Box();\nm.get("+key+").AnnualRevenue++;\n").Diagnostics, "GLADESEMA023", invalidCall)
			}
			root := t.TempDir()
			path := filepath.Join(root, "Box.cls")
			writeSemaFile(t, path, box)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			// R4-11 compiles; a property key keeps the name-path rejection.
			requireNoIncrementErrors(t, AnalyzeAnonymous(index, accounts+"Account acc = new Account(Name = 'one');\n++m.get(acc.Name).AnnualRevenue;", api))
			requireIncrementDiagnostic(t, AnalyzeAnonymous(index, accounts+"Box b = new Box();\n++m.get(b.key).AnnualRevenue;", api).Diagnostics, "GLADESEMA_ANONYMOUS_PARSE", "expected ; at byte")
		})
	}
}
