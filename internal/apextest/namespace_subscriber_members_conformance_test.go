package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Native rows: testdata/conformance/namespace_subscriber_members/org.tsv.
// A subscriber org with a 1GP managed beta installed (provider namespace
// gladens004, package sources under provider/) is asked about the provider's
// global, public and private nested types and about the provider's global
// interface over package-owned custom objects. The loaded dependency project
// stands in for the installed package.
const namespaceSubscriberMembersDir = "testdata/conformance/namespace_subscriber_members"

type namespaceSubscriberRow struct {
	Case, ConsumerAPI, ProviderAPI, Route, Compile, Observed, Source string
}

func readNamespaceSubscriberRows(t *testing.T) []namespaceSubscriberRow {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(namespaceSubscriberMembersDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) < 2 || lines[0] != "case_id\tconsumer_api\tprovider_api\tcontext\troute\tcompile_outcome\truntime_outcome\tobserved_value_or_diagnostic\tsource" {
		t.Fatalf("org.tsv header: %q", lines[0])
	}
	var rows []namespaceSubscriberRow
	for _, line := range lines[1:] {
		c := strings.Split(line, "\t")
		if len(c) != 9 {
			t.Fatalf("bad row %q", line)
		}
		rows = append(rows, namespaceSubscriberRow{c[0], c[1], c[2], c[4], c[5], c[7], c[8]})
	}
	return rows
}

func readSubscriberFixture(t *testing.T, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(namespaceSubscriberMembersDir, rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// namespaceSubscriberProject writes the provider (installed-package stand-in)
// and a subscriber that holds the given classes.
func namespaceSubscriberProject(t *testing.T, consumerAPI, providerAPI string, consumer map[string]string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	provider, subscriber := filepath.Join(root, "provider"), filepath.Join(root, "subscriber")
	meta := func(api string) string {
		return "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
	}
	writeFile(t, filepath.Join(provider, "sfdx-project.json"), `{"namespace":"gladens004","sourceApiVersion":"`+providerAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	entries, err := os.ReadDir(filepath.Join(namespaceSubscriberMembersDir, "provider/classes"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".cls") || strings.HasSuffix(name, "Test.cls") {
			continue
		}
		writeFile(t, filepath.Join(provider, "force-app/main/default/classes", name), readSubscriberFixture(t, "provider/classes/"+name))
		writeFile(t, filepath.Join(provider, "force-app/main/default/classes", name+"-meta.xml"), meta(providerAPI))
	}
	for _, object := range []string{"V035Env__c", "V035Sliver__c"} {
		writeFile(t, filepath.Join(provider, "force-app/main/default/objects", object, object+".object-meta.xml"), readSubscriberFixture(t, "provider/objects/"+object+".object"))
	}
	writeFile(t, filepath.Join(subscriber, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"`+consumerAPI+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(subscriber, "glade.yml"), "project:\n  managedPackageDependencies: [\"gladens004:../provider:1.0\"]\n")
	for name, code := range consumer {
		writeFile(t, filepath.Join(subscriber, "force-app/main/default/classes", name+".cls"), code)
		writeFile(t, filepath.Join(subscriber, "force-app/main/default/classes", name+".cls-meta.xml"), meta(consumerAPI))
	}
	return loadTestIndex(t, subscriber)
}

func nativeDiagnosticText(d diagnostic.Diagnostic) string {
	if d.NativeMessage != "" {
		return d.NativeMessage
	}
	return d.Message
}

var subscriberDiagnosticPattern = regexp.MustCompile(`^line (\d+) col (\d+): (.*)$`)

// subscriberErrors returns the error texts in order.
func subscriberErrors(ds []diagnostic.Diagnostic) []string {
	var out []string
	for _, d := range ds {
		if d.Severity == diagnostic.Error {
			out = append(out, nativeDiagnosticText(d))
		}
	}
	return out
}

// subscriberNonErrors names diagnostics that are not errors, for failure text.
func subscriberNonErrors(ds []diagnostic.Diagnostic) string {
	var out []string
	for _, d := range ds {
		if d.Severity != diagnostic.Error {
			out = append(out, fmt.Sprintf("%s %s %s", d.Severity, d.Code, nativeDiagnosticText(d)))
		}
	}
	return strings.Join(out, "; ")
}

func assertSubscriberRejection(t *testing.T, row namespaceSubscriberRow, got []string, others string) {
	t.Helper()
	m := subscriberDiagnosticPattern.FindStringSubmatch(row.Observed)
	if m == nil {
		t.Fatalf("row %s/%s: unparsable native diagnostic %q", row.Case, row.Route, row.Observed)
	}
	if len(got) == 0 {
		t.Errorf("%s %s %s: Glade accepted (non-errors: %s); native rejects with %q", row.Case, row.Route, row.ConsumerAPI, others, m[3])
		return
	}
	if got[0] != m[3] {
		t.Errorf("%s %s %s: Glade says %q; native says %q", row.Case, row.Route, row.ConsumerAPI, got[0], m[3])
	}
}

// namespaceSubscriberCarried rows come from the real installed package. A
// local partial-source project cannot prove a type absent from the package
// (design 1.2, 2.7), so Glade keeps those spellings open; the native answer is
// Invalid type.
var namespaceSubscriberCarried = map[string]string{
	"S13": "missing nested under a present global outer: loaded source is not known to be the whole package",
	"S14": "missing package type: loaded source is not known to be the whole package",
}

func TestNamespaceSubscriberMembersNativeConformance(t *testing.T) {
	carried := 0
	rows := readNamespaceSubscriberRows(t)
	if len(rows) != 112 {
		t.Fatalf("org.tsv rows=%d, want 112", len(rows))
	}
	localClasses := map[string]string{
		"V035Local": readSubscriberFixture(t, "local/V035Local.cls"),
		"Leaf":      readSubscriberFixture(t, "local/Leaf.cls"),
	}
	type group struct{ consumer, provider string }
	groups := map[group][]namespaceSubscriberRow{}
	var order []group
	for _, row := range rows {
		g := group{row.ConsumerAPI, row.ProviderAPI}
		if _, ok := groups[g]; !ok {
			order = append(order, g)
		}
		groups[g] = append(groups[g], row)
	}
	for _, g := range order {
		g := g
		t.Run(fmt.Sprintf("consumer%s_provider%s", g.consumer, g.provider), func(t *testing.T) {
			classes := map[string]string{}
			for k, v := range localClasses {
				classes[k] = v
			}
			for _, row := range groups[g] {
				switch {
				case row.Route == "istest" && strings.HasPrefix(row.Case, "S"):
					classes["R_"+row.Case] = readSubscriberFixture(t, row.Source)
				case row.Route == "istest":
					for _, name := range []string{"ToneRelay", "QuietTone", "ToneChannelProof"} {
						classes[name] = readSubscriberFixture(t, "xns_classes/"+name+".cls")
					}
				}
			}
			index := namespaceSubscriberProject(t, g.consumer, g.provider, classes)
			if errs := subscriberErrors(sema.Analyze(index).Diagnostics); len(errs) > 0 {
				t.Errorf("loaded subscriber project reports errors: %q", errs)
			}
			org := orgFromIndex(index)
			org.APIVersion = g.consumer
			for _, row := range groups[g] {
				row := row
				if reason, ok := namespaceSubscriberCarried[row.Case]; ok {
					carried++
					t.Logf("carried %s %s %s: %s; native %q", row.Case, row.Route, g.consumer, reason, row.Observed)
					continue
				}
				switch row.Route {
				case "anonymous":
					source := readSubscriberFixture(t, row.Source)
					analysis := sema.AnalyzeAnonymous(index, source, g.consumer)
					errs := subscriberErrors(analysis.Diagnostics)
					if row.Compile == "COMPILE_ERROR" {
						assertSubscriberRejection(t, row, errs, subscriberNonErrors(analysis.Diagnostics))
						continue
					}
					if len(errs) > 0 {
						t.Errorf("%s anonymous %s: Glade rejects %q; native compiles", row.Case, g.consumer, errs[0])
						continue
					}
					program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: g.consumer})
					if err != nil {
						t.Errorf("%s anonymous %s: %v", row.Case, g.consumer, err)
						continue
					}
					runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
					result, err := runner.execute(program)
					if err != nil {
						t.Errorf("%s anonymous %s: execution error %v; native %q", row.Case, g.consumer, err, row.Observed)
						continue
					}
					want := "OBS|" + row.Case + "|" + row.Observed
					found := false
					for _, line := range result.Debug {
						if strings.HasSuffix(line, want) {
							found = true
						}
					}
					if !found {
						t.Errorf("%s anonymous %s: debug %q lacks %q", row.Case, g.consumer, result.Debug, want)
					}
				case "named":
					name := strings.TrimSuffix(filepath.Base(row.Source), ".cls")
					one := map[string]string{"V035Local": localClasses["V035Local"], "Leaf": localClasses["Leaf"], name: readSubscriberFixture(t, row.Source)}
					single := namespaceSubscriberProject(t, g.consumer, g.provider, one)
					namedResult := sema.Analyze(single)
					assertSubscriberRejection(t, row, subscriberErrors(namedResult.Diagnostics), subscriberNonErrors(namedResult.Diagnostics))
				case "istest":
					selected := "R_" + row.Case
					if !strings.HasPrefix(row.Case, "S") {
						selected = "ToneChannelProof"
					}
					run := Run(index, Options{SelectedClasses: []string{selected}, NoDiskCache: true, Parallelism: 1})
					found := false
					for _, suite := range run.Suites {
						for _, c := range suite.Cases {
							if c.Problem == nil {
								continue
							}
							if _, value, ok := strings.Cut(c.Problem.Message, "OBS|"+row.Case+"|"); ok {
								found = true
								if value != row.Observed {
									t.Errorf("%s istest %s: Glade %q; native %q", row.Case, g.consumer, value, row.Observed)
								}
							}
						}
					}
					if !found {
						t.Errorf("%s istest %s: no observation (summary %+v problem %s)", row.Case, g.consumer, run.Summary(), firstRunProblem(run))
					}
				default:
					t.Fatalf("route %q", row.Route)
				}
			}
		})
	}
}

// A vendored partial copy of a package: types it does not declare stay open,
// a nested type it declares non-global is still not visible.
func TestNamespaceSubscriberPartialSourceStaysOpen(t *testing.T) {
	root := t.TempDir()
	provider, subscriber := filepath.Join(root, "provider"), filepath.Join(root, "subscriber")
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>"
	writeFile(t, filepath.Join(provider, "sfdx-project.json"), `{"namespace":"pkg","sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(provider, "force-app/main/default/classes/Helper.cls"), "global class Helper { global class Inner {} public class StubPublic {} }")
	writeFile(t, filepath.Join(provider, "force-app/main/default/classes/Helper.cls-meta.xml"), meta)
	writeFile(t, filepath.Join(subscriber, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(subscriber, "glade.yml"), "project:\n  managedPackageDependencies: [\"pkg:../provider:1.0\"]\n")
	cases := map[string]string{
		"Missing": "pkg.Widget w;",
		"Other":   "pkg.Helper.Other o;",
		"Literal": "Type t = pkg.Helper.Other.class;",
		"Present": "pkg.Helper.Inner i;",
		"Stub":    "pkg.Helper.StubPublic s;",
		"StubLit": "Type t = pkg.Helper.StubPublic.class;",
	}
	for name, body := range cases {
		writeFile(t, filepath.Join(subscriber, "force-app/main/default/classes/N_"+name+".cls"), "public class N_"+name+" { public static void m() { "+body+" } }")
		writeFile(t, filepath.Join(subscriber, "force-app/main/default/classes/N_"+name+".cls-meta.xml"), meta)
	}
	index := loadTestIndex(t, subscriber)
	named := map[string][]string{}
	for _, d := range sema.Analyze(index).Diagnostics {
		if d.Severity == diagnostic.Error {
			named[strings.TrimSuffix(filepath.Base(d.File), ".cls")] = append(named[strings.TrimSuffix(filepath.Base(d.File), ".cls")], d.Code+" "+nativeDiagnosticText(d))
		}
	}
	for name, body := range cases {
		want := ""
		if strings.HasPrefix(name, "Stub") {
			want = "GLADESEMA035 Type is not visible: pkg.Helper.StubPublic"
		}
		anon := subscriberErrors(sema.AnalyzeAnonymous(index, body, "62.0").Diagnostics)
		var anonCodes []string
		for _, d := range sema.AnalyzeAnonymous(index, body, "62.0").Diagnostics {
			if d.Severity == diagnostic.Error {
				anonCodes = append(anonCodes, d.Code+" "+nativeDiagnosticText(d))
			}
		}
		_ = anon
		for route, got := range map[string][]string{"anonymous": anonCodes, "named": named["N_"+name]} {
			if want == "" && len(got) != 0 || want != "" && (len(got) == 0 || got[0] != want) {
				t.Errorf("%s %s: got %q, want %q", name, route, got, want)
			}
		}
	}
}
