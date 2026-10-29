package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// A38 shadow-api62 and shadow-api67 org.tsv and named-org.tsv S001-S004:
// a source class shadows the unqualified Builder while explicit System keeps
// the platform Builder. platform_type_references/org.tsv invalid_system
// retains rejection of an unknown System root at both versions and routes.
func TestNestedPlatformTypeWithProjectShadow(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			shadow := filepath.Join(root, "QueueableDuplicateSignature.cls")
			probe := filepath.Join(root, "NestedPlatformProbe.cls")
			writeSemaFile(t, shadow, `public class QueueableDuplicateSignature { public class Builder { public String value = 'project'; } }`)
			writeSemaFile(t, probe, `public class NestedPlatformProbe { public static void run() {
                Object a = new QueueableDuplicateSignature.Builder();
                Object b = new System.QueueableDuplicateSignature.Builder();
            } }`)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{shadow, probe}}, schema.Schema{})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if result := Analyze(index); result.HasErrors() {
				t.Fatalf("named: %#v", result.Diagnostics)
			}
			if result := AnalyzeAnonymous(index, `Object a = new QueueableDuplicateSignature.Builder(); Object b = new System.QueueableDuplicateSignature.Builder();`, api); result.HasErrors() {
				t.Fatalf("anonymous: %#v", result.Diagnostics)
			}
			if result := AnalyzeAnonymous(index, `Object b = new System.NotAType();`, api); !result.HasErrors() {
				t.Fatal("unknown System type accepted")
			} else if !strings.Contains(result.Diagnostics[0].Message, "NotAType") {
				t.Fatalf("unexpected rejection: %#v", result.Diagnostics)
			}
		})
	}
}
