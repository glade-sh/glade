package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// memberViewSharingProject writes an SFDX project with count handler classes.
// Every handler overrides a zero-argument void method of an abstract base, the
// shape the declaration-contract pass inspects through a member view. A few
// shapes are invalid so the pass has diagnostics to reproduce.
func memberViewSharingProject(t *testing.T, count int) string {
	t.Helper()
	root := t.TempDir()
	classes := filepath.Join(root, "force-app/main/default/classes")
	if err := os.MkdirAll(classes, 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	write := func(name, body string) {
		writeSemaFile(t, filepath.Join(classes, name+".cls"), body)
		writeSemaFile(t, filepath.Join(classes, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	}
	write("BaseHandler", "public abstract class BaseHandler {\n public abstract void run();\n public virtual void reset() {}\n public abstract String label();\n}\n")
	write("LabelledHandler", "public abstract class LabelledHandler extends BaseHandler {\n public abstract override String label();\n}\n")
	for i := 0; i < count; i++ {
		switch i % 4 {
		case 0:
			write(fmt.Sprintf("Handler%d", i), fmt.Sprintf("public class Handler%d extends BaseHandler {\n public override void run() { Integer v = %d; v++; }\n public override String label() { return 'h%d'; }\n}\n", i, i, i))
		case 1:
			write(fmt.Sprintf("Handler%d", i), fmt.Sprintf("public class Handler%d extends BaseHandler {\n public override void run() { }\n public override void missing() { }\n public override String label() { return 'h'; }\n}\n", i))
		case 2:
			write(fmt.Sprintf("Handler%d", i), fmt.Sprintf("public abstract class Handler%d extends LabelledHandler {\n public abstract override Integer label();\n public override void run() { }\n}\n", i))
		default:
			write(fmt.Sprintf("Handler%d", i), fmt.Sprintf("public class Handler%d extends BaseHandler {\n public override void run() { }\n public override void reset() { }\n public override String label() { return 'x'; }\n}\n", i))
		}
	}
	return root
}

func memberViewSharingIndex(t *testing.T, root string) (typesys.Index, bool) {
	t.Helper()
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, false
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		return typesys.Index{}, false
	}
	return typesys.Build(p, s), true
}

func TestAnalysisBuildsOneTypeMemberModel(t *testing.T) {
	index, ok := memberViewSharingIndex(t, memberViewSharingProject(t, 40))
	if !ok {
		t.Fatal("generated project did not load")
	}
	// Warm process-wide caches so the count covers this analysis alone.
	Analyze(index)
	before := semaTypeMemberLayerBuilds.Load()
	result := Analyze(index)
	builds := semaTypeMemberLayerBuilds.Load() - before
	if builds > 2 {
		t.Fatalf("analysis built %d type-member models for 40 handler classes, want at most 2", builds)
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("generated project must produce diagnostics for the equality checks to mean something")
	}
}
