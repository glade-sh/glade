package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native ../apextest/testdata/conformance/class_literal_prefix/org.tsv (API
// 62/67/68, both routes): a class literal whose suffix names no type resolves
// to its nearest enclosing type, whatever the spelling case of the source
// prefix. rejections.tsv keeps static values and unknown roots invalid.
func TestClassLiteralMissingSuffixResolvesEnclosingTypeInClassBody(t *testing.T) {
	t.Parallel()
	shapes := `public class LiteralShapes {
    public class Part {}
    public enum Color { RED }
    public static String TOKEN = 'x';
}`
	for _, tc := range []struct {
		name, literal string
		reject        bool
	}{
		{"mixed_case_root", "literalshapes.UNKNOWN", false},
		{"deep_missing", "LiteralShapes.Unknown.Deeper", false},
		{"outer_missing", "LiteralShapes.Unknown", false},
		{"nested_missing", "LiteralShapes.Part.Unknown", false},
		{"enum_missing", "LiteralShapes.Color.Unknown", false},
		{"static_field", "LiteralShapes.TOKEN", true},
		{"enum_constant", "LiteralShapes.Color.RED", true},
		{"missing_root", "UnregisteredRoot", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeSemaFile(t, filepath.Join(root, "LiteralShapes.cls"), shapes)
			writeSemaFile(t, filepath.Join(root, "LiteralUser.cls"), `public class LiteralUser {
    public static String observe() {
        Type literal = `+tc.literal+`.class;
        return literal.getName();
    }
}`)
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{
				filepath.Join(root, "LiteralShapes.cls"),
				filepath.Join(root, "LiteralUser.cls"),
			}}, schema.Schema{})
			var errors []string
			for _, item := range Analyze(index).Diagnostics {
				if item.Severity == diagnostic.Error && strings.HasSuffix(item.File, "LiteralUser.cls") {
					errors = append(errors, item.Code+": "+item.Message)
				}
			}
			if tc.reject && len(errors) == 0 {
				t.Fatalf("%s.class accepted; native rejects it", tc.literal)
			}
			if !tc.reject && len(errors) != 0 {
				t.Fatalf("%s.class rejected; native resolves the enclosing type: %v", tc.literal, errors)
			}
		})
	}
}
