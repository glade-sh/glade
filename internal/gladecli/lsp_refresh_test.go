package gladecli

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDiagnosticsRefreshAfterSave(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	project := filepath.Join(root, "project")
	writeTestProject(t, project)
	file := filepath.Join(project, "force-app/main/classes/Refresh.cls")
	writeTestFile(t, file, "public class Refresh { public void run() {} }")
	l, err := newLSPProjectLoader(project)
	if err != nil {
		t.Fatal(err)
	}
	l.analyzer(l.artifacts)(l.index)
	writeTestFile(t, file, "public class Refresh { public void run() { Integer x = missing; } }")
	index, analyze, err := l.refresh(context.Background(), []string{file})
	if err != nil {
		t.Fatal(err)
	}
	broken := analyze(index)
	if len(broken.Diagnostics) == 0 {
		t.Fatal("save did not produce diagnostics")
	}
	writeTestFile(t, file, "public class Refresh { public void run() {} }")
	index, analyze, err = l.refresh(context.Background(), []string{file})
	if err != nil {
		t.Fatal(err)
	}
	fixed := analyze(index)
	if len(fixed.Diagnostics) >= len(broken.Diagnostics) {
		t.Fatalf("diagnostics not cleared: %#v", fixed.Diagnostics)
	}
}

func TestSaveRefreshIsIncremental(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	project := filepath.Join(root, "project")
	writeTestProject(t, project)
	file := filepath.Join(project, "force-app/main/classes/One.cls")
	writeTestFile(t, file, "public class One { public void run() { Integer x = 1; } }")
	writeTestFile(t, filepath.Join(project, "force-app/main/classes/Two.cls"), "public class Two { public void run() { Integer x = 1; } }")
	l, err := newLSPProjectLoader(project)
	if err != nil {
		t.Fatal(err)
	}
	l.analyzer(l.artifacts)(l.index)
	for _, n := range []string{"2", "3"} {
		writeTestFile(t, file, "public class One { public void run() { Integer x = "+n+"; } }")
		index, analyze, err := l.refresh(context.Background(), []string{file})
		if err != nil {
			t.Fatal(err)
		}
		analyze(index)
	}
	stats := l.state.LastStats()
	if !stats.Incremental || stats.Rechecked >= stats.Types {
		t.Fatalf("not incremental: %+v", stats)
	}
}
