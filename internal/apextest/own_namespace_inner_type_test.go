package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Synthetic. Project source declares namespace ownNsTwin, either in the root
// sfdx-project.json or in the sfdx-project.json of a sub-project whose source
// the root loads. Classes name its inner type as ownNsTwin.OwnNsOuter.OwnNsInner.
// Project source resolves first; a qualifier that resolves nowhere rejects.
const ownNamespaceOuter = `global with sharing class OwnNsOuter {
    global class OwnNsInner {
        global Integer size() { return 2; }
    }
    global static OwnNsInner make() { return new OwnNsInner(); }
}`

func ownNamespaceConsumer(qualifier string) string {
	q := qualifier + ".OwnNsOuter.OwnNsInner"
	return `public with sharing class OwnNsConsumer {
    public ` + q + ` field;
    public static ` + q + ` relay(` + q + ` parameter) {
        ` + q + ` local = new ` + q + `();
        List<` + q + `> values = new List<` + q + `>{ local };
        values.add(new ` + q + `());
        Object raw = values[0];
        ` + q + ` converted = (` + q + `) raw;
        Boolean matches = raw instanceof ` + q + `;
        return parameter == null ? converted : parameter;
    }
    public static Integer size() { return relay(null).size(); }
}`
}

func ownNamespaceConsumerTest(qualifier string) string {
	q := qualifier + ".OwnNsOuter.OwnNsInner"
	return `@IsTest private class OwnNsConsumerTest {
    @IsTest static void resolves() {
        ` + q + ` value = new ` + q + `();
        System.assertEquals(2, value.size());
        System.assertEquals(2, OwnNsConsumer.size());
        System.assertEquals(2, OwnNsOuter.make().size());
    }
}`
}

func ownNamespaceAnonymous(qualifier string) string {
	q := qualifier + ".OwnNsOuter.OwnNsInner"
	return q + ` value = new ` + q + `();
System.assertEquals(2, value.size());`
}

// layout "root": the root project declares the namespace.
// layout "nested": a sub-project under the root's package directory declares
// it; declare=false leaves that sub-project without a namespace.
func ownNamespaceProject(t *testing.T, api, layout, qualifier string, declare bool) typesys.Index {
	t.Helper()
	root := t.TempDir()
	namespace := ""
	if declare {
		namespace = "ownNsTwin"
	}
	meta := "<ApexClass><apiVersion>" + api + "</apiVersion><status>Active</status></ApexClass>"
	write := func(dir, name, source string) {
		path := filepath.Join(dir, "force-app", "main", "default", "classes", name+".cls")
		writeFile(t, path, source)
		writeFile(t, path+"-meta.xml", meta)
	}
	switch layout {
	case "root":
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"`+namespace+`","sourceApiVersion":"`+api+`"}`)
		write(root, "OwnNsOuter", ownNamespaceOuter)
		write(root, "OwnNsConsumer", ownNamespaceConsumer(qualifier))
		write(root, "OwnNsConsumerTest", ownNamespaceConsumerTest(qualifier))
	case "nested":
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"packs","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
		base := filepath.Join(root, "packs", "BaseTwin")
		consumer := filepath.Join(root, "packs", "ConsumerTwin")
		writeFile(t, filepath.Join(base, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"`+namespace+`","sourceApiVersion":"`+api+`"}`)
		writeFile(t, filepath.Join(consumer, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
		write(base, "OwnNsOuter", ownNamespaceOuter)
		write(consumer, "OwnNsConsumer", ownNamespaceConsumer(qualifier))
		write(consumer, "OwnNsConsumerTest", ownNamespaceConsumerTest(qualifier))
	default:
		t.Fatalf("layout %q", layout)
	}
	proj, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[bool]string{true: namespace, false: ""}[layout == "root"]; proj.Namespace != want {
		t.Fatalf("project namespace=%q, want %q", proj.Namespace, want)
	}
	if nested := layout == "nested" && declare; (len(proj.SourceNamespaces) == 1 && proj.SourceNamespaces[0] == "ownNsTwin") != nested || (!nested && len(proj.SourceNamespaces) != 0) {
		t.Fatalf("source namespaces=%q", proj.SourceNamespaces)
	}
	index := typesys.Build(proj, schema.Schema{})
	if index.HasErrors() {
		t.Fatal(index.Diagnostics)
	}
	return index
}

func ownNamespaceErrors(ds []diagnostic.Diagnostic) []string {
	var errors []string
	for _, d := range ds {
		if d.Severity == diagnostic.Error {
			errors = append(errors, d.Code+" "+d.NativeMessage+" | "+d.Message)
		}
	}
	return errors
}

func TestOwnNamespaceInnerTypeResolvesOnEveryRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, layout := range []string{"root", "nested"} {
			for _, qualifier := range []string{"ownNsTwin", "OWNNSTWIN"} {
				t.Run(api+"/"+layout+"/"+qualifier, func(t *testing.T) {
					index := ownNamespaceProject(t, api, layout, qualifier, true)
					if errors := ownNamespaceErrors(sema.Analyze(index).Diagnostics); len(errors) != 0 {
						t.Fatalf("named/@IsTest class errors: %q", errors)
					}
					if errors := ownNamespaceErrors(sema.AnalyzeAnonymous(index, ownNamespaceAnonymous(qualifier), api).Diagnostics); len(errors) != 0 {
						t.Fatalf("anonymous errors: %q", errors)
					}
					for _, warm := range []bool{false, true} {
						run := Run(index, Options{NoDiskCache: !warm, Parallelism: 1, SelectedClasses: []string{"OwnNsConsumerTest"}})
						s := run.Summary()
						if s.Total != 1 || s.CompileErrors != 0 || len(run.ExcludedClasses) != 0 {
							t.Fatalf("warm=%v summary=%+v excluded=%+v problem=%s", warm, s, run.ExcludedClasses, firstRunProblem(run))
						}
						// The runtime binds classes by their indexed namespace. A
						// sub-project namespace qualifier compiles here; dispatch
						// through it is a separate runtime boundary.
						if layout == "root" && (s.Passed != 1 || s.RuntimeErrors != 0) {
							t.Fatalf("warm=%v summary=%+v problem=%s", warm, s, firstRunProblem(run))
						}
					}
				})
			}
		}
	}
}

// Twins: the same source types named through a namespace that neither the
// project nor any loaded sub-project declares, and no dependency supplies.
func TestForeignNamespaceInnerTypeRejectedOnEveryRoute(t *testing.T) {
	cases := []struct {
		layout, qualifier string
		declare           bool
	}{
		{"root", "foreignNsTwin", true},
		{"nested", "foreignNsTwin", true},
		{"nested", "ownNsTwin", false},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range cases {
			reference := tc.qualifier + ".OwnNsOuter.OwnNsInner"
			want := "Invalid type: " + reference
			t.Run(api+"/"+tc.layout+"/"+tc.qualifier, func(t *testing.T) {
				index := ownNamespaceProject(t, api, tc.layout, tc.qualifier, tc.declare)
				rejected := map[string]bool{}
				for _, d := range sema.Analyze(index).Diagnostics {
					if d.Severity != diagnostic.Error {
						continue
					}
					file := strings.TrimSuffix(filepath.Base(d.File), ".cls")
					if file != "OwnNsConsumer" && file != "OwnNsConsumerTest" {
						t.Errorf("error outside the consumers: %+v", d)
					}
					if d.NativeMessage != "" && d.NativeMessage != want {
						t.Errorf("unexpected native error: %+v", d)
					}
					rejected[file] = rejected[file] || d.NativeMessage == want
				}
				if !rejected["OwnNsConsumer"] || !rejected["OwnNsConsumerTest"] {
					t.Fatalf("foreign namespace type accepted in named/@IsTest class: %v", rejected)
				}
				found := false
				for _, d := range sema.AnalyzeAnonymous(index, ownNamespaceAnonymous(tc.qualifier), api).Diagnostics {
					if d.Severity != diagnostic.Error {
						continue
					}
					if d.NativeMessage != "" && d.NativeMessage != want {
						t.Errorf("unexpected anonymous error: %+v", d)
					}
					found = found || d.NativeMessage == want
				}
				if !found {
					t.Fatal("foreign namespace type accepted in anonymous Apex")
				}
				for _, warm := range []bool{false, true} {
					run := Run(index, Options{NoDiskCache: !warm, Parallelism: 1, SelectedClasses: []string{"OwnNsConsumerTest"}})
					s := run.Summary()
					if s.Passed != 0 || s.CompileErrors != 1 || len(run.ExcludedClasses) != 2 {
						t.Fatalf("warm=%v summary=%+v excluded=%+v", warm, s, run.ExcludedClasses)
					}
					for _, entry := range run.ExcludedClasses {
						if (entry.ClassName != "OwnNsConsumer" && entry.ClassName != "OwnNsConsumerTest") || !strings.Contains(entry.Message, want) {
							t.Fatalf("unexpected exclusion: %+v", entry)
						}
					}
				}
			})
		}
	}
}
