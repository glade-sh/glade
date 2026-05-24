package typesys

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestStressBuildLargeProjectIndex(t *testing.T) {
	root := t.TempDir()
	files := make([]string, 0, 240)
	for i := 0; i < 200; i++ {
		path := filepath.Join(root, fmt.Sprintf("classes/Class%03d.cls", i))
		writeStressFile(t, path, fmt.Sprintf("public class Class%03d { public Integer run() { return %d; } }", i, i))
		files = append(files, path)
	}
	for i := 0; i < 40; i++ {
		path := filepath.Join(root, fmt.Sprintf("triggers/Trigger%03d.trigger", i))
		writeStressFile(t, path, fmt.Sprintf("trigger Trigger%03d on Account (before insert) {}", i))
		files = append(files, path)
	}

	idx := Build(project.Project{Root: root, ApexFiles: files}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	if idx.HasErrors() {
		t.Fatalf("unexpected diagnostics: %#v", idx.Diagnostics)
	}
	if len(idx.Types) != 200 || len(idx.Triggers) != 40 {
		t.Fatalf("index sizes: types=%d triggers=%d", len(idx.Types), len(idx.Triggers))
	}
}

func writeStressFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
