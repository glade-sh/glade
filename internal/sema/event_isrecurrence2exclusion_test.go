package sema

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native rejects a direct assignment to a non-createable, non-updateable
// standard field at class API 65, 66 and 67 (dot-27 evidence). The rule comes
// from describe createable/updateable, not a field-name list, so a second
// standard field with the same flags must give the same diagnostic.
func TestNonWriteableStandardFieldAssignmentNativeMessage(t *testing.T) {
	cases := []struct{ object, field, value string }{
		{"Event", "IsRecurrence2Exclusion", "false"},
		{"Contact", "CreatedDate", "Datetime.now()"},
	}
	for _, tc := range cases {
		for _, api := range []string{"65.0", "66.0", "67.0"} {
			t.Run(fmt.Sprintf("%s.%s/%s", tc.object, tc.field, api), func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "Bnd.cls")
				writeSemaFile(t, path, fmt.Sprintf("public class Bnd { public static void run() { %s c = new %s(); c.%s = %s; } }", tc.object, tc.object, tc.field, tc.value))
				result := Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{}))
				if len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != "GLADESEMA027" {
					t.Fatalf("expected one GLADESEMA027, got %#v", result.Diagnostics)
				}
				want := "Field is not writeable: " + tc.object + "." + tc.field
				if got := result.Diagnostics[0].NativeMessage; got != want {
					t.Fatalf("native message = %q, want %q", got, want)
				}
			})
		}
	}
}
