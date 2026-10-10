package sema

import (
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"path/filepath"
	"testing"
)

func TestDataWeaveLegacyCompileContracts(t *testing.T) {
	for _, test := range []struct {
		name, body string
		reject     bool
	}{
		{"carrierField", `DataWeave.Result value = null; String result=value.valueAsString;`, true},
		{"carrierMime", `DataWeave.Result value = null; String result=value.getMimeType();`, true},
		{"createdDate", `Contact value=new Contact(); value.CreatedDate=Datetime.now();`, true},
		{"supportedCarrier", `DataWeave.Result value=null; Object output=value.getValue(); String text=value.getValueAsString();`, false},
		{"recordIdentity", `Contact value=new Contact(); value.Id='003000000000001'; value.LastName='Owned'; Datetime created=value.CreatedDate;`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := AnalyzeAnonymous(typesys.Index{}, test.body, "65.0")
			if result.HasErrors() != test.reject {
				t.Fatalf("anonymous reject=%v diagnostics=%#v", test.reject, result.Diagnostics)
			}
			root := t.TempDir()
			p := filepath.Join(root, "OwnedLegacyProof.cls")
			writeSemaFile(t, p, "public class OwnedLegacyProof { public static void run() {"+test.body+"} }")
			result = Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: "65.0", ApexFiles: []string{p}}, schema.Schema{}))
			if result.HasErrors() != test.reject {
				t.Fatalf("project reject=%v diagnostics=%#v", test.reject, result.Diagnostics)
			}
		})
	}
}
