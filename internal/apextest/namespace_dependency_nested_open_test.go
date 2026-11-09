package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

// A loaded dependency source is not known to be the whole package: a nested
// type missing under a present global outer stays open in every position, on
// the anonymous and named routes. Own-namespace nested misses stay rejected.
func TestDependencySourceNestedMissStaysOpen(t *testing.T) {
	for _, withLocal := range []bool{false, true} {
		t.Run(map[bool]string{false: "no_local_class", true: "same_name_local_class"}[withLocal], func(t *testing.T) { dependencySourceNestedMiss(t, withLocal) })
	}
}

func dependencySourceNestedMiss(t *testing.T, withLocal bool) {
	root := t.TempDir()
	provider, subscriber := filepath.Join(root, "provider"), filepath.Join(root, "subscriber")
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>"
	writeFile(t, filepath.Join(provider, "sfdx-project.json"), `{"namespace":"pkg","sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(provider, "force-app/main/default/classes/Shell.cls"), "global class Shell { global class Present {} global static Integer v() { return 1; } }")
	writeFile(t, filepath.Join(provider, "force-app/main/default/classes/Shell.cls-meta.xml"), meta)
	writeFile(t, filepath.Join(subscriber, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(subscriber, "glade.yml"), "project:\n  managedPackageDependencies: [\"pkg:../provider:1.0\"]\n")
	if withLocal {
		writeFile(t, filepath.Join(subscriber, "force-app/main/default/classes/Shell.cls"), "public class Shell { public class MissingNested {} }")
		writeFile(t, filepath.Join(subscriber, "force-app/main/default/classes/Shell.cls-meta.xml"), meta)
	}
	classes := map[string]string{
		"Field":  "public class Field { pkg.Shell.MissingNested f; }",
		"Param":  "public class Param { public void m(pkg.Shell.MissingNested p) {} }",
		"Return": "public class ReturnU { public pkg.Shell.MissingNested m() { return null; } }",
		"Local":  "public class Local { public void m() { pkg.Shell.MissingNested l; } }",
		"New":    "public class NewU { public void m() { Object o = new pkg.Shell.MissingNested(); } }",
		"Cast":   "public class CastU { public void m(Object o) { Object c = (pkg.Shell.MissingNested)o; } }",
		"List":   "public class Lst { public void m() { List<pkg.Shell.MissingNested> l; } }",
		"Lit":    "public class Lit { public void m() { Type t = pkg.Shell.MissingNested.class; } }",
		"Ext":    "public class Ext extends pkg.Shell.MissingNested {}",
		"Pres":   "public class Pres { public void m() { pkg.Shell.Present p = new pkg.Shell.Present(); } }",
	}
	for name, code := range classes {
		cls := strings.Fields(strings.SplitN(code, "{", 2)[0])
		writeFile(t, filepath.Join(subscriber, "force-app/main/default/classes", cls[len(cls)-1]+".cls"), code)
		writeFile(t, filepath.Join(subscriber, "force-app/main/default/classes", cls[len(cls)-1]+".cls-meta.xml"), meta)
		_ = name
	}
	index := loadTestIndex(t, subscriber)
	for _, d := range sema.Analyze(index).Diagnostics {
		if d.Severity == diagnostic.Error {
			t.Errorf("named: %s %s %s", filepath.Base(d.File), d.Code, nativeDiagnosticText(d))
		}
	}
	for _, body := range []string{
		"pkg.Shell.MissingNested l;",
		"Object o = new pkg.Shell.MissingNested();",
		"Type t = pkg.Shell.MissingNested.class;",
		"List<pkg.Shell.MissingNested> l;",
	} {
		for _, d := range sema.AnalyzeAnonymous(index, body, "62.0").Diagnostics {
			if d.Severity == diagnostic.Error {
				t.Errorf("anonymous %q: %s %s", body, d.Code, nativeDiagnosticText(d))
			}
		}
	}
}

// A vendored copy of a package loaded as a sub-project namespace is not known
// to be complete either: a nested miss under a present outer stays open.
func TestSubprojectNamespaceNestedMissStaysOpen(t *testing.T) {
	root := t.TempDir()
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"62.0","packageDirectories":[{"path":"app","default":true},{"path":"vendor/pkgsrc"}]}`)
	writeFile(t, filepath.Join(root, "vendor/pkgsrc/sfdx-project.json"), `{"namespace":"pkg","sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "vendor/pkgsrc/force-app/main/default/classes/Shell.cls"), "global class Shell { global class Present {} }")
	writeFile(t, filepath.Join(root, "vendor/pkgsrc/force-app/main/default/classes/Shell.cls-meta.xml"), meta)
	writeFile(t, filepath.Join(root, "app/main/default/classes/Use.cls"), "public class Use { public void m() { pkg.Shell.MissingNested l; pkg.Shell.Present p; } }")
	writeFile(t, filepath.Join(root, "app/main/default/classes/Use.cls-meta.xml"), meta)
	index := loadTestIndex(t, root)
	for _, d := range sema.Analyze(index).Diagnostics {
		if d.Severity == diagnostic.Error {
			t.Errorf("named: %s %s", d.Code, nativeDiagnosticText(d))
		}
	}
	for _, body := range []string{"pkg.Shell.MissingNested l;", "Type t = pkg.Shell.MissingNested.class;"} {
		for _, d := range sema.AnalyzeAnonymous(index, body, "62.0").Diagnostics {
			if d.Severity == diagnostic.Error {
				t.Errorf("anonymous %q: %s %s", body, d.Code, nativeDiagnosticText(d))
			}
		}
	}
}

// Own-namespace nested misses under a present outer stay Invalid type (design 2.4).
func TestOwnNamespaceNestedMissStillRejected(t *testing.T) {
	root := t.TempDir()
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"own","sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/Shell.cls"), "global class Shell { global class Present {} }")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/Shell.cls-meta.xml"), meta)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/Use.cls"), "public class Use { public void m() { own.Shell.Bogus l; } }")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/Use.cls-meta.xml"), meta)
	index := loadTestIndex(t, root)
	var named []string
	for _, d := range sema.Analyze(index).Diagnostics {
		if d.Severity == diagnostic.Error {
			named = append(named, nativeDiagnosticText(d))
		}
	}
	var anon []string
	for _, d := range sema.AnalyzeAnonymous(index, "own.Shell.Bogus l;", "62.0").Diagnostics {
		if d.Severity == diagnostic.Error {
			anon = append(anon, nativeDiagnosticText(d))
		}
	}
	if len(named) != 1 || named[0] != "Invalid type: own.Shell.Bogus" || len(anon) != 1 || anon[0] != "Invalid type: own.Shell.Bogus" {
		t.Fatalf("named=%q anonymous=%q", named, anon)
	}
}
