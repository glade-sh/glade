package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestNamespaceCrossOwnerTriggerBody(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			child := filepath.Join(root, "child")
			if err := os.MkdirAll(child, 0755); err != nil {
				t.Fatal(err)
			}
			foreign := filepath.Join(child, "ForeignShape.cls")
			trigger := filepath.Join(root, "AccountTrg.trigger")
			writeSemaFile(t, foreign, "public class ForeignShape {}")
			writeSemaFile(t, trigger, "trigger AccountTrg on Account (before insert) { host.ForeignShape value; }")
			writeSemaFile(t, trigger+"-meta.xml", `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexTrigger>`)
			idx := typesys.Build(project.Project{Root: root, Namespace: "host", SourceAPIVersion: api, SourceNamespaces: []string{"other"}, SourceNamespaceRoots: map[string]string{child: "other"}, ApexFiles: []string{foreign, trigger}}, schema.Schema{})
			res := Analyze(idx)
			found := false
			for _, d := range res.Diagnostics {
				if d.Severity == diagnostic.Error && d.NativeMessage == "Invalid type: host.ForeignShape" {
					found = true
				}
			}
			if !found {
				t.Errorf("trigger did not report native Invalid type: %+v", res.Diagnostics)
			}
		})
	}
}
