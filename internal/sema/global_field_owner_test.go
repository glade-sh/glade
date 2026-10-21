package sema

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestGlobalFieldEnclosingOwnerUsesSameFile(t *testing.T) {
	root := t.TempDir()
	var paths []string
	for unit, source := range map[string]string{
		"public":  "public class Container { public class Child { public Integer value; } }",
		"global":  "global class Container { global class Child { global Integer value; } }",
		"invalid": "public class Container { global class Child { global Integer value; } }",
	} {
		dir := filepath.Join(root, unit)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "Container.cls")
		writeSemaFile(t, path, source)
		paths = append(paths, path)
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: paths}, schema.Schema{})
	// Put the unrelated public owner first, as the duplicate-name index may do.
	sort.SliceStable(index.Types, func(i, j int) bool {
		left, right := index.Types[i], index.Types[j]
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		return strings.Contains(left.File, string(filepath.Separator)+"public"+string(filepath.Separator)) &&
			!strings.Contains(right.File, string(filepath.Separator)+"public"+string(filepath.Separator))
	})
	result := Analyze(index)
	invalidRejected := false
	for _, d := range result.Diagnostics {
		if d.Severity != diagnostic.Error {
			continue
		}
		if !strings.Contains(d.File, string(filepath.Separator)+"invalid"+string(filepath.Separator)) {
			t.Fatalf("unrelated enclosing declaration rejected valid global field: %#v", d)
		}
		if strings.Contains(d.Message, "Enclosing type for global fields") {
			invalidRejected = true
		}
	}
	if !invalidRejected {
		t.Fatalf("own non-global enclosing type lost its rejection: %#v", result.Diagnostics)
	}
}
