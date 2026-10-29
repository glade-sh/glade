package apextest

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

func writeVisualforceControllerHintProject(t *testing.T, controllerBody string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/AccountView.page"), `<apex:page standardController="Account" extensions="AccountViewExtension"><apex:outputText value="{!name}"/></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/AccountView.page-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><label>AccountView</label></ApexPage>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/AccountViewExtension.cls"), controllerBody)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/AccountViewExtension.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	return root
}

const hintAcceptedExtension = `public with sharing class AccountViewExtension {
  public String name { get; set; }
  public AccountViewExtension(ApexPages.StandardController controller) {
    name = ((Account) controller.getRecord()).Name;
  }
}
`

const hintRejectedExtension = `public with sharing class AccountViewExtension {
  public String name { get; set; }
  public AccountViewExtension(ApexPages.StandardController controller) {
    Missing__Type probe = null;
  }
}
`

// The semantic gate's diagnostics, offered to Visualforce page registration,
// must register exactly the pages the standalone controller compile registers.
func TestVisualforcePageNamesFromSemanticGateMatchOwnCompile(t *testing.T) {
	for name, body := range map[string]string{"accepted": hintAcceptedExtension, "rejected": hintRejectedExtension} {
		t.Run(name, func(t *testing.T) {
			index := loadTestIndex(t, writeVisualforceControllerHintProject(t, body))
			analysis := sema.AnalyzeWithOptions(SemanticAnalysisIndex(index), sema.AnalyzeOptions{Diagnostics: true, ExportTypes: true, SuppressPerformanceDiagnostics: true})
			hint := visualforceControllerCompilationHint(index, analysis.Diagnostics)
			if hint == nil {
				t.Fatal("a complete index offered no compilation")
			}
			own := visualforcePageNames(index)
			reused := visualforcePageNamesWithCompilation(index, hint)
			if !reflect.DeepEqual(own, reused) {
				t.Fatalf("page names differ: own compile %v, gate reuse %v", own, reused)
			}
			switch name {
			case "accepted":
				if len(own) != 1 || own[0] != "AccountView" {
					t.Fatalf("accepted extension registered %v", own)
				}
			case "rejected":
				if len(own) != 0 {
					t.Fatalf("rejected extension registered %v", own)
				}
			}
		})
	}
}

// An index whose metadata schema failed to load is not what the controller
// compile gate analyzes; it must not be offered.
func TestVisualforceControllerCompilationHintSkipsFailedSchema(t *testing.T) {
	index := loadTestIndex(t, writeVisualforceControllerHintProject(t, hintAcceptedExtension))
	index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESCHEMA001", Message: "metadata schema load failed"})
	if visualforceControllerCompilationHint(index, nil) != nil {
		t.Fatal("an index with a failed schema load was offered as the controller compilation")
	}
}

// A compile-gate subset drops the excluded classes and their diagnostics, so
// it is not the project's compilation. Page registration from it must match the
// strict loader's own compile, which rejects the page.
func TestVisualforceControllerCompilationHintSkipsCompileGateSubset(t *testing.T) {
	root := writeVisualforceControllerHintProject(t, hintRejectedExtension)
	index := loadTestIndex(t, root)
	excluded := map[string]bool{filepath.Clean(filepath.Join(root, "force-app/main/default/classes/AccountViewExtension.cls")): true}
	subset := filterCompileGateIndex(index, excluded)
	analysis := sema.AnalyzeWithOptions(SemanticAnalysisIndex(subset), sema.AnalyzeOptions{Diagnostics: true, ExportTypes: true, SuppressPerformanceDiagnostics: true})
	hint := visualforceControllerCompilationHint(subset, analysis.Diagnostics)
	own := visualforcePageNames(subset)
	reused := visualforcePageNamesWithCompilation(subset, hint)
	if len(own) != 0 || !reflect.DeepEqual(own, reused) {
		t.Fatalf("page names differ: own compile %v, subset reuse %v", own, reused)
	}
	if hint != nil {
		t.Fatal("a compile-gate subset index was offered as the controller compilation")
	}
}
