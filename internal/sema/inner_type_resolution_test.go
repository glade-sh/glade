package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Synthetic. An Outer class with an inner enum and an inner class, and a
// consumer that names them as types and as static receivers. Apex resolves
// Outer.Inner through the project class first, whether the project declares
// no namespace, its own namespace, or an external namespace without source.
const innerTypeOuter = `public with sharing class InnerOuterV036 {
    public enum Level { LOW, HIGH }
    public class Box { public Integer size = 1; }
    public static Integer made = 2;
    public static Level top() { return Level.HIGH; }
}
`

const innerTypeConsumer = `public with sharing class InnerConsumerV036 {
    public InnerOuterV036.Level field = InnerOuterV036.Level.LOW;
    public InnerOuterV036.Box box;
    public List<InnerOuterV036.Box> boxes = new List<InnerOuterV036.Box>();
    public Map<InnerOuterV036.Level, InnerOuterV036.Box> byLevel = new Map<InnerOuterV036.Level, InnerOuterV036.Box>();
    public static InnerOuterV036.Level pick(InnerOuterV036.Level lv) { return lv; }
    public static Integer run(Object o) {
        InnerOuterV036.Level local = InnerOuterV036.Level.HIGH;
        InnerOuterV036.Box b = new InnerOuterV036.Box();
        InnerOuterV036.Box c = (InnerOuterV036.Box) o;
        Boolean is = o instanceof InnerOuterV036.Box;
        List<InnerOuterV036.Level> levels = InnerOuterV036.Level.values();
        Integer n = InnerOuterV036.made;
        if (local == InnerOuterV036.Level.HIGH && is) { n += b.size + levels.size(); }
        return n;
    }
}
`

func innerTypeIndex(t *testing.T, api, namespace string, deps []typesys.DependencyInfo) typesys.Index {
	t.Helper()
	root := t.TempDir()
	var paths []string
	for name, source := range map[string]string{"InnerOuterV036": innerTypeOuter, "InnerConsumerV036": innerTypeConsumer} {
		path := filepath.Join(root, name+".cls")
		writeSemaFile(t, path, source)
		writeSemaFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
		paths = append(paths, path)
	}
	index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
	if index.HasErrors() {
		t.Fatalf("index diagnostics: %v", index.Diagnostics)
	}
	index.Dependencies = deps
	return index
}

func TestInnerTypesResolveThroughOuterClassProject(t *testing.T) {
	cases := map[string]struct {
		namespace string
		deps      []typesys.DependencyInfo
	}{
		"no namespace":          {},
		"own namespace":         {namespace: "ownNs"},
		"external declared":     {deps: []typesys.DependencyInfo{{Namespace: "extNs", Status: "declared"}}},
		"external missing":      {deps: []typesys.DependencyInfo{{Namespace: "extNs", Status: "missing"}}},
		"own plus external dec": {namespace: "ownNs", deps: []typesys.DependencyInfo{{Namespace: "extNs", Status: "declared"}}},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for name, tc := range cases {
			t.Run(api+"/"+name, func(t *testing.T) {
				index := innerTypeIndex(t, api, tc.namespace, tc.deps)
				for _, d := range Analyze(index).Diagnostics {
					t.Errorf("unexpected %s %s %s %s %s", d.Severity, d.Code, filepath.Base(d.File), d.Excerpt, d.Message)
				}
				anonymous := "InnerOuterV036.Level l = InnerOuterV036.Level.HIGH;\nInnerOuterV036.Box b = new InnerOuterV036.Box();\nSystem.debug(l + ' ' + b.size + ' ' + InnerOuterV036.made);\n"
				for _, d := range AnalyzeAnonymous(index, anonymous, api).Diagnostics {
					t.Errorf("unexpected anonymous %s %s %s", d.Severity, d.Code, d.Message)
				}
			})
		}
	}
}

// An Outer class that is absent keeps native rejection (namespace_external_positions).
func TestInnerTypeSpellingStillRejectedWhenOuterAbsent(t *testing.T) {
	index := innerTypeIndex(t, "67.0", "", nil)
	anonymous := "InnerMissingV036.Level l;\n"
	found := false
	for _, d := range AnalyzeAnonymous(index, anonymous, "67.0").Diagnostics {
		if strings.Contains(d.Message, "Invalid type: InnerMissingV036.Level") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected Invalid type for absent outer")
	}
}

// Synthetic unknown roots cannot be attributed to a missing package alias.
// The installed-package capture separately proves indexed members resolve.
const unsourcedRootConsumer = `public with sharing class UnsourcedConsumerV036 {
    public UnlockedOuterV036.Level field;
    public List<UnlockedOuterV036.Box> boxes;
    public static UnlockedOuterV036.Level pick(UnlockedOuterV036.Level lv) { return lv; }
    public static Object run(Object o) {
        Boolean is = o instanceof UnlockedOuterV036.Box;
        Object c = (UnlockedOuterV036.Box) o;
        return UnlockedOuterV036.Level.HIGH;
    }
}
`

func unsourcedRootDiagnostics(t *testing.T, api string, deps []typesys.DependencyInfo) []string {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "UnsourcedConsumerV036.cls")
	writeSemaFile(t, path, unsourcedRootConsumer)
	writeSemaFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
	if index.HasErrors() {
		t.Fatalf("index diagnostics: %v", index.Diagnostics)
	}
	index.Dependencies = deps
	var got []string
	for _, d := range Analyze(index).Diagnostics {
		got = append(got, d.Code+" "+d.Message)
	}
	for _, d := range AnalyzeAnonymous(index, "Object o = UnlockedOuterV036.Level.HIGH;\n", api).Diagnostics {
		got = append(got, "anonymous "+d.Code+" "+d.Message)
	}
	return got
}

func TestUnsourcedPackageDoesNotOpenUnprovenInnerTypeRoots(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			unlocked := []typesys.DependencyInfo{{Namespace: "unlocked-package-alias", Status: "missing"}}
			got := strings.Join(unsourcedRootDiagnostics(t, api, unlocked), "\n")
			for _, want := range []string{"Invalid type: UnlockedOuterV036.Level", "Invalid type: UnlockedOuterV036.Box", "anonymous GLADESEMA013"} {
				if !strings.Contains(got, want) {
					t.Errorf("unsourced alias missing diagnostic %q: %s", want, got)
				}
			}
			// A namespace-only declaration for another root does not cover it.
			declared := []typesys.DependencyInfo{{Namespace: "otherNs", Status: "declared"}}
			if got := unsourcedRootDiagnostics(t, api, declared); len(got) == 0 {
				t.Fatal("a declared unrelated namespace must not open an absent root")
			}
			// A configured namespaced package whose source is missing qualifies
			// its types, so it does not open unqualified roots.
			configured := []typesys.DependencyInfo{{Namespace: "pkgns", Status: "missing", SourceRoot: "/absent/source"}}
			if got := unsourcedRootDiagnostics(t, api, configured); len(got) == 0 {
				t.Fatal("a configured namespaced dependency without source must not open unqualified roots")
			}
			// Nothing declared: an org without the class rejects every position.
			if got := unsourcedRootDiagnostics(t, api, nil); len(got) == 0 {
				t.Fatal("no declaration: absent root must stay rejected")
			}
		})
	}
}

// A three-part spelling cannot be a class with an inner type (Apex nests one
// level), so an unsourced package alias does not open it.
func TestUnsourcedPackageDoesNotOpenThreePartSpelling(t *testing.T) {
	const api = "67.0"
	root := t.TempDir()
	path := filepath.Join(root, "ThreePartV036.cls")
	writeSemaFile(t, path, "public with sharing class ThreePartV036 {\n    public MistypedNs.Outer.Inner field;\n}\n")
	writeSemaFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
	index.Dependencies = []typesys.DependencyInfo{{Namespace: "unlocked-package-alias", Status: "missing"}}
	if len(Analyze(index).Diagnostics) == 0 {
		t.Fatal("three-part spelling under an unknown root must stay rejected")
	}
}
