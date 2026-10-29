package visualforce

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// A caller's compilation mirrors the Apex test runner's semantic gate: the
// same index construction, analyzed with diagnostics and exported types.
func gateLikeCompilation(t *testing.T, p project.Project) *ControllerCompilation {
	t.Helper()
	objects, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, objects)
	analysis := sema.AnalyzeWithOptions(index, sema.AnalyzeOptions{Diagnostics: true, ExportTypes: true, SuppressPerformanceDiagnostics: true})
	return &ControllerCompilation{Index: index, Diagnostics: analysis.Diagnostics}
}

func writeControllerPageProject(t *testing.T, controllerBody string) project.Project {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeController.cls"), controllerBody)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeController.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Probe.page"), `<apex:page controller="ProbeController"><apex:outputText value="{!value}"/></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Probe.page-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><label>Probe</label></ApexPage>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Plain.page"), `<apex:page/>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Plain.page-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><label>Plain</label></ApexPage>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

const validProbeController = `public with sharing class ProbeController {
  public String value { get; set; }
  public ProbeController() { value = 'ok'; }
}
`

// The controller declares a local of an unknown type: a controller compile
// rejection whose metadata diagnostic is rewritten before the gate reads it.
const rejectedProbeController = `public with sharing class ProbeController {
  public String value { get; set; }
  public void run() { Missing__Type local = null; }
}
`

// Page registration served from a caller's compilation must register exactly
// the pages the strict loader registers on its own, for accepted and rejected
// controllers alike, and must leave the caller's diagnostics untouched.
func TestPageNamesReuseSuppliedControllerCompilation(t *testing.T) {
	for name, body := range map[string]string{"accepted": validProbeController, "rejected": rejectedProbeController} {
		t.Run(name, func(t *testing.T) {
			p := writeControllerPageProject(t, body)
			compilation := gateLikeCompilation(t, p)
			original := diagnosticMessages(compilation)
			plain := PageNames(p)
			before := controllerCompilationReuses.Load()
			reused := PageNamesWithControllerCompilation(p, compilation)
			if controllerCompilationReuses.Load() != before+1 {
				t.Fatalf("supplied compilation was not reused (%d -> %d)", before, controllerCompilationReuses.Load())
			}
			if !reflect.DeepEqual(plain, reused) {
				t.Fatalf("page names differ: own compile %v, reused compilation %v", plain, reused)
			}
			if name == "accepted" && len(plain) != 2 {
				t.Fatalf("accepted controller registered %v", plain)
			}
			if name == "rejected" && plain != nil {
				t.Fatalf("rejected controller still registered %v", plain)
			}
			if got := diagnosticMessages(compilation); !reflect.DeepEqual(got, original) {
				t.Fatalf("reuse rewrote the caller's diagnostics in place: %v -> %v", original, got)
			}
		})
	}
}

// Every compile decision is a project-level one: a compilation whose index
// does not carry this project's identity is ignored, and the loader compiles.
func TestPageNamesIgnoreForeignControllerCompilation(t *testing.T) {
	accepted := writeControllerPageProject(t, validProbeController)
	rejected := writeControllerPageProject(t, rejectedProbeController)
	foreign := gateLikeCompilation(t, accepted)
	before := controllerCompilationReuses.Load()
	if got := PageNamesWithControllerCompilation(rejected, foreign); got != nil {
		t.Fatalf("foreign compilation accepted a rejected controller: %v", got)
	}
	if got := PageNamesWithControllerCompilation(rejected, &ControllerCompilation{}); got != nil {
		t.Fatalf("empty compilation accepted a rejected controller: %v", got)
	}
	if got := PageNamesWithControllerCompilation(accepted, nil); len(got) != 2 {
		t.Fatalf("nil compilation changed registration: %v", got)
	}
	if controllerCompilationReuses.Load() != before {
		t.Fatal("a compilation without this project's identity was reused")
	}
}

func diagnosticMessages(compilation *ControllerCompilation) []string {
	out := make([]string, 0, len(compilation.Diagnostics))
	for _, diag := range compilation.Diagnostics {
		out = append(out, diag.Code+": "+diag.Message)
	}
	return out
}

// A source that changes between the compilation and the page load makes the
// compilation stale: the loader must compile again and see the rejection.
func TestPageNamesIgnoreStaleControllerCompilation(t *testing.T) {
	p := writeControllerPageProject(t, validProbeController)
	compilation := gateLikeCompilation(t, p)
	writeFile(t, filepath.Join(p.Root, "force-app/main/default/classes/ProbeController.cls"), rejectedProbeController)
	before := controllerCompilationReuses.Load()
	if got := PageNamesWithControllerCompilation(p, compilation); got != nil {
		t.Fatalf("stale compilation accepted a rejected controller: %v", got)
	}
	if controllerCompilationReuses.Load() != before {
		t.Fatal("a stale compilation was reused")
	}
}

// An Apex file added after the compilation is outside its file set.
func TestPageNamesIgnoreControllerCompilationMissingFile(t *testing.T) {
	p := writeControllerPageProject(t, validProbeController)
	compilation := gateLikeCompilation(t, p)
	writeFile(t, filepath.Join(p.Root, "force-app/main/default/classes/Added.cls"), "public class Added {}\n")
	writeFile(t, filepath.Join(p.Root, "force-app/main/default/classes/Added.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	reloaded, err := project.Load(p.Root)
	if err != nil {
		t.Fatal(err)
	}
	before := controllerCompilationReuses.Load()
	PageNamesWithControllerCompilation(reloaded, compilation)
	if controllerCompilationReuses.Load() != before {
		t.Fatal("a compilation missing an Apex file was reused")
	}
}
