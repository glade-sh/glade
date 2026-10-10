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

func TestWorkspaceSourcePathMemoMatchesSymlinkResolution(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "physical")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	physical := filepath.Join(dir, "MemoPath.cls")
	second := filepath.Join(dir, "Second.cls")
	unicode := filepath.Join(dir, "  путь λ .cls")
	writeSemaFile(t, physical, "public class MemoPath {}")
	writeSemaFile(t, second, "public class Second {}")
	writeSemaFile(t, unicode, "public class UnicodePath {}")
	for _, link := range []struct{ name, target string }{
		{"directory", dir},
		{"Absolute.cls", physical},
		{"Relative.cls", filepath.Join("physical", "MemoPath.cls")},
		{"Chain.cls", "Relative.cls"},
		{filepath.Join("physical", "InDirectory.cls"), "MemoPath.cls"},
	} {
		if err := os.Symlink(link.target, filepath.Join(root, link.name)); err != nil {
			t.Fatal(err)
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, physical)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{
		physical, second, unicode, relative,
		filepath.Join(root, "directory", "MemoPath.cls"),
		filepath.Join(root, "directory", "InDirectory.cls"),
		filepath.Join(root, "Absolute.cls"),
		filepath.Join(root, "Relative.cls"),
		filepath.Join(root, "Chain.cls"),
	}
	_, artifacts := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: paths}, schema.Schema{})
	sources := artifacts.Sources.All()
	if len(sources) != len(paths) {
		t.Fatalf("captured %d sources, want %d", len(sources), len(paths))
	}
	for i, path := range paths {
		metadata := sources[i].Metadata()
		if metadata.RequestedPath != path || metadata.PhysicalPath != workspaceSourcePathReference(path) {
			t.Errorf("source %q metadata = %#v, want physical path %q", path, metadata, workspaceSourcePathReference(path))
		}
	}
	if got := artifacts.Sources.Stats().PhysicalReadAttempts; got != 3 {
		t.Fatalf("physical read attempts = %d, want 3", got)
	}
}

func TestWorkspaceSourcePathMemoPreservesReadErrors(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "physical")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, link := range []struct{ name, target string }{
		{"directory", dir},
		{"Dangling.cls", "Missing.cls"},
		{"CycleA.cls", "CycleB.cls"},
		{"CycleB.cls", "CycleA.cls"},
	} {
		if err := os.Symlink(link.target, filepath.Join(root, link.name)); err != nil {
			t.Fatal(err)
		}
	}
	paths := []string{
		filepath.Join(root, "Missing.cls"),
		filepath.Join(root, "missing-directory", "Missing.cls"),
		filepath.Join(root, "directory", "Missing.cls"),
		filepath.Join(root, "Dangling.cls"),
		filepath.Join(root, "CycleA.cls"),
		filepath.Join(root, "  \t "),
		filepath.Join(root, "不存在.cls"),
		"",
	}
	index, artifacts := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: paths}, schema.Schema{})
	if len(index.Diagnostics) != len(paths) {
		t.Fatalf("diagnostics = %#v, want one per path", index.Diagnostics)
	}
	for i, path := range paths {
		_, err := os.ReadFile(workspaceSourcePathReference(path))
		pathErr, ok := err.(*os.PathError)
		if !ok {
			t.Fatalf("reference read %q error = %v, want path error", path, err)
		}
		requestedErr := *pathErr
		requestedErr.Path = path
		want := diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADETYPE000", Message: requestedErr.Error(), File: path}
		if index.Diagnostics[i] != want {
			t.Errorf("diagnostic %d = %#v, want %#v", i, index.Diagnostics[i], want)
		}
	}
	if got := len(artifacts.Sources.All()); got != 0 {
		t.Fatalf("captured %d sources after failed reads", got)
	}
}

func TestWorkspaceSourcePathMemoDoesNotSurviveBuildOrRefresh(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first", "second"} {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeSemaFile(t, filepath.Join(dir, "MemoPath.cls"), "public class MemoPath {}")
	}
	alias := filepath.Join(root, "directory")
	if err := os.Symlink("first", alias); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(alias, "MemoPath.cls")
	p := project.Project{Root: root, ApexFiles: []string{path}}
	index, previous := typesys.BuildWithArtifacts(p, schema.Schema{})
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("second", alias); err != nil {
		t.Fatal(err)
	}
	_, rebuilt := typesys.BuildWithArtifacts(p, schema.Schema{})
	refreshed, err := typesys.RefreshBuildArtifacts(index, &previous)
	if err != nil {
		t.Fatal(err)
	}
	want := workspaceSourcePathReference(path)
	for name, artifacts := range map[string]typesys.BuildArtifacts{"rebuilt": rebuilt, "refreshed": refreshed} {
		sources := artifacts.Sources.All()
		if len(sources) != 1 || sources[0].Metadata().PhysicalPath != want {
			t.Errorf("%s sources = %#v, want physical path %q", name, sources, want)
		}
		if got := artifacts.Sources.Stats().PhysicalReadAttempts; got != 1 {
			t.Errorf("%s physical read attempts = %d, want 1", name, got)
		}
	}
}

func workspaceSourcePathReference(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	abs = filepath.Clean(abs)
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs
	}
	resolvedAbs, err := filepath.Abs(resolved)
	if err != nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(resolvedAbs)
}
