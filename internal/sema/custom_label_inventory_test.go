package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestCustomLabelInventoryTracksMetadataGeneration(t *testing.T) {
	root := t.TempDir()
	writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	path := filepath.Join(root, "force-app/main/default/classes/LabelGeneration.cls")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, path, `public class LabelGeneration { public static String read() { return Label.PresentDiagnosticLabel; } }`)
	labels := filepath.Join(root, "force-app/main/default/labels/CustomLabels.labels-meta.xml")
	if err := os.MkdirAll(filepath.Dir(labels), 0755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, labels, `<CustomLabels xmlns="http://soap.sforce.com/2006/04/metadata"><labels><fullName>PresentDiagnosticLabel</fullName><value>Present</value></labels></CustomLabels>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	original := typesys.Build(p, schema.Schema{})
	if got := Analyze(original); got.HasErrors() {
		t.Fatalf("declared label rejected: %#v", got.Diagnostics)
	}
	writeSemaFile(t, labels, `<CustomLabels xmlns="http://soap.sforce.com/2006/04/metadata"><labels><fullName>RenamedDiagnosticLabel</fullName><value>Present</value></labels></CustomLabels>`)
	if got := Analyze(original); got.HasErrors() {
		t.Fatalf("retained index reread changed label metadata: %#v", got.Diagnostics)
	}
	fresh := typesys.Build(p, schema.Schema{})
	if got := Analyze(fresh); !got.HasErrors() {
		t.Fatal("renamed custom label is still accepted")
	}
	if got := AnalyzeAnonymous(original, `String value = Label.PresentDiagnosticLabel;`, "67.0"); got.HasErrors() {
		t.Fatalf("anonymous retained label rejected: %#v", got.Diagnostics)
	}
	if got := AnalyzeAnonymous(fresh, `String value = Label.PresentDiagnosticLabel;`, "67.0"); !got.HasErrors() {
		t.Fatal("anonymous renamed custom label is still accepted")
	}
}
