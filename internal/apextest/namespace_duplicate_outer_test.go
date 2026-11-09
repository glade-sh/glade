package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

// A multi-package project: a sub-project namespace holds Evaluator with a
// nested Wrapper, and an unrelated package directory holds another class named
// Evaluator that loses the cross-package selection or wins it. A
// namespace-qualified spelling keeps binding the class, whichever copy is selected.
func TestSubprojectNamespaceQualifiedNestedBindsNamespaceCopy(t *testing.T) {
	root := t.TempDir()
	meta := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"62.0","packageDirectories":[{"path":"a_user","default":true},{"path":"actions"},{"path":"processes"}]}`)
	writeFile(t, filepath.Join(root, "actions/c_pkg/sfdx-project.json"), `{"namespace":"pkg","sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "actions/c_pkg/force-app/main/default/classes/Evaluator.cls"), "global class Evaluator { global class Wrapper { global Wrapper(String a, String b) {} } global static String run(String f, String c) { return f; } }")
	writeFile(t, filepath.Join(root, "actions/c_pkg/force-app/main/default/classes/Evaluator.cls-meta.xml"), meta)
	// The namespace still owns a selected class, so it stays a source namespace.
	writeFile(t, filepath.Join(root, "actions/c_pkg/force-app/main/default/classes/Other.cls"), "global class Other {}")
	writeFile(t, filepath.Join(root, "actions/c_pkg/force-app/main/default/classes/Other.cls-meta.xml"), meta)
	writeFile(t, filepath.Join(root, "processes/d_plain/sfdx-project.json"), `{"namespace":"","sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "processes/d_plain/force-app/main/default/classes/Evaluator.cls"), "public class Evaluator { public class Wrapper { public Wrapper(String a, String b) {} } public static String run(String f) { return f; } }")
	writeFile(t, filepath.Join(root, "processes/d_plain/force-app/main/default/classes/Evaluator.cls-meta.xml"), meta)
	body := "public class UserOfPkg { public String m() { List<pkg.Evaluator.Wrapper> ctx = new List<pkg.Evaluator.Wrapper>(); ctx.add(new pkg.Evaluator.Wrapper('a', 'b')); return pkg.Evaluator.run('f', 'c'); } }"
	writeFile(t, filepath.Join(root, "a_user/main/default/classes/UserOfPkg.cls"), body)
	writeFile(t, filepath.Join(root, "a_user/main/default/classes/UserOfPkg.cls-meta.xml"), meta)
	index := loadTestIndex(t, root)
	for _, d := range sema.Analyze(index).Diagnostics {
		if d.Severity == diagnostic.Error {
			t.Errorf("%s %s %s", filepath.Base(d.File), d.Code, nativeDiagnosticText(d))
		}
	}
}
