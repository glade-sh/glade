package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Synthetic shapes for installed managed-package namespaces with no source in
// the project. v0.3.1 (a recorded revision) compiles them outside local declarations;
// native A04 C055/N010/N011 reject only the local-declaration shape, which
// TestQualifiedLocalTypeNativeRejectionTwins keeps.
const unknownNamespaceSignatureClass = `public with sharing class SignatureTwin {
    public vendorTwin.CoreThing current;
    public static vendorTwin.CoreThing relay(vendorTwin.CoreThing request) { return request; }
    public static void handle(TriggersTwin.Context context, List<vendorTwin.CoreThing> items) {}
    public static Integer count(Map<String, TriggersTwin.Context> byName) { return byName == null ? 0 : byName.size(); }
}`

const unknownNamespaceSignatureTest = `@IsTest private class SignatureTwinTest {
    static vendorTwin.CoreThing cached;
    static Integer measure(TriggersTwin.Context context) { return 1; }
    @IsTest static void compiles() {
        System.assertEquals(null, SignatureTwin.relay(null));
        System.assertEquals(1, measure(null));
        System.assertEquals(0, SignatureTwin.count(null));
    }
}`

const unknownNamespaceSignatureAnonymous = `System.assertEquals(null, SignatureTwin.relay(null));
SignatureTwin.handle(null, null);
System.assertEquals(0, SignatureTwin.count(null));
System.assertEquals(null, new SignatureTwin().current);`

func unknownNamespaceSignatureIndex(t *testing.T, api string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	var paths []string
	for name, source := range map[string]string{"SignatureTwin": unknownNamespaceSignatureClass, "SignatureTwinTest": unknownNamespaceSignatureTest} {
		path := filepath.Join(root, name+".cls")
		writeSemaFile(t, path, source)
		writeSemaFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
		paths = append(paths, path)
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
	if index.HasErrors() {
		t.Fatal(index.Diagnostics)
	}
	return index
}

func TestUnknownNamespaceSignatureTypesCompile(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := unknownNamespaceSignatureIndex(t, api)
			for route, ds := range map[string][]diagnostic.Diagnostic{
				"named/@IsTest": Analyze(index).Diagnostics,
				"anonymous":     AnalyzeAnonymous(index, unknownNamespaceSignatureAnonymous, api).Diagnostics,
			} {
				for _, d := range ds {
					if d.Severity == diagnostic.Error {
						t.Errorf("%s: %s %q | %q", route, d.Code, d.NativeMessage, d.Message)
					}
				}
			}
		})
	}
}

// The local-declaration twin of the same references still rejects natively.
func TestUnknownNamespaceLocalTwinRejected(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		index := unknownNamespaceSignatureIndex(t, api)
		for _, reference := range []string{"vendorTwin.CoreThing", "TriggersTwin.Context"} {
			t.Run(api+"/"+reference, func(t *testing.T) {
				result := AnalyzeAnonymous(index, reference+" local;", api)
				for _, d := range result.Diagnostics {
					if d.Severity == diagnostic.Error && d.NativeMessage != "Invalid type: "+reference {
						t.Errorf("unexpected error: %+v", d)
					}
				}
				if !result.HasErrors() {
					t.Fatal("unresolved local type accepted")
				}
			})
		}
	}
}

// A declared installed package whose source is opaque: v0.3.1 compiles its
// types in every position, including locals.
func TestDeclaredOpaquePackageTypesCompile(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "OpaqueConsumer.cls")
			writeSemaFile(t, path, `public with sharing class OpaqueConsumer {
    public depTwin.Triggers.Context context;
    public static Integer handle(depTwin.Triggers.Context context) { depTwin.Triggers.Context local = context; return local == null ? 0 : 1; }
}`)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			index.Dependencies = []typesys.DependencyInfo{{Namespace: "depTwin", Status: "missing"}}
			for route, ds := range map[string][]diagnostic.Diagnostic{
				"named":     Analyze(index).Diagnostics,
				"anonymous": AnalyzeAnonymous(index, "depTwin.Triggers.Context local = null; System.assertEquals(0, OpaqueConsumer.handle(local));", api).Diagnostics,
			} {
				for _, d := range ds {
					if d.Severity == diagnostic.Error {
						t.Errorf("%s: %s %q | %q", route, d.Code, d.NativeMessage, d.Message)
					}
				}
			}
		})
	}
}
