package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRunExecPreservesHistoricalApexSourceVersion(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"53.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/HistoricalExec53.cls"), `public class HistoricalExec53 {
 public static void createRecord() { insert new Contact(LastName='Historical exec'); }
 }`)
	writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/HistoricalExec53.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>`)
	const source = `HistoricalExec53.createRecord(); List<Contact> rows = [SELECT Id,LastName FROM Contact WITH SECURITY_ENFORCED]; System.assertEquals(1,rows.size()); System.assertEquals('Historical exec',rows[0].LastName);`
	project, index, err := loadProjectIndex(root)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareAnonymousSource(source, project.SourceAPIVersion)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.close()
	if prepared.apiVersion != "53.0" {
		t.Fatalf("anonymous version changed to %q", prepared.apiVersion)
	}
	if analysis := sema.AnalyzeAnonymous(index, prepared.body, prepared.apiVersion); analysis.HasErrors() {
		t.Fatalf("historical anonymous diagnostics: %#v", analysis.Diagnostics)
	}
	program, err := vm.CompileAnonymousWithOptions(prepared.body, vm.CompileOptions{APIVersion: prepared.apiVersion})
	if err != nil {
		t.Fatal(err)
	}
	if program.APIVersion != "53.0" {
		t.Fatalf("compiled version changed to %q", program.APIVersion)
	}
	// This query form is unavailable at67; a silent source-version upgrade cannot pass.
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), []string{"exec", "--project", root, "--json", source}, &stdout, &stderr); code != 0 {
		t.Fatalf("exec code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	// ResolveSource remains the strict registry used by the LWC compiler.
	if _, err := apexversion.ResolveSource("53.0"); err == nil {
		t.Fatal("checked/LWC version registry was broadened")
	}
}

func TestRunExecRejectsInvalidHistoricalApexVersions(t *testing.T) {
	for _, version := range []string{"68.0", "53.5", "0.0"} {
		t.Run(version, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+version+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"exec", "--project", root, "--json", "System.assert(false,'must not execute');"}, &stdout, &stderr)
			if code == 0 || !strings.Contains(stderr.String(), "unsupported source API version") {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
		})
	}
}
