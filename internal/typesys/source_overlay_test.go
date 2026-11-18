package typesys

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestOverlayGenerationIsVolatile(t *testing.T) {
	root := t.TempDir()
	writeIncrementalSFDXProject(t, root, "")
	path := filepath.Join(root, "force-app/main/default/classes/First.cls")
	original := "public class First {}"
	writeFile(t, path, original)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, previous := BuildWithArtifacts(p, schema.Schema{})
	overlay := NewSourceOverlay(map[string]string{path: original})
	changed, ok := TryUpdateApexFileWithOverlay(idx, path, p, overlay)
	if !ok {
		t.Fatal("overlay update declined")
	}
	artifacts, err := RefreshBuildArtifactsWithOverlay(changed, &previous, overlay)
	if err != nil {
		t.Fatal(err)
	}
	if !changed.Volatile() || !artifacts.Volatile() {
		t.Fatal("overlay generation is not volatile")
	}
	if err := ValidateBuildGeneration(changed, &artifacts); !errors.Is(err, ErrVolatileGeneration) {
		t.Fatalf("build validation: %v", err)
	}
	if err := ValidateSourceGeneration(changed, artifacts.SourceDigests); !errors.Is(err, ErrVolatileGeneration) {
		t.Fatalf("source validation: %v", err)
	}
	if MatchesProjectSources(changed, p) {
		t.Fatal("volatile index matches project")
	}
}

func TestOverlayReadAndIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "First.cls")
	if err := os.WriteFile(path, []byte("disk"), 0600); err != nil {
		t.Fatal(err)
	}
	input := map[string]string{path: "buffer"}
	overlay := NewSourceOverlay(input)
	input[path] = "mutated"
	got, err := overlay.ReadFile(path)
	if err != nil || string(got) != "buffer" {
		t.Fatalf("overlay read = %q, %v", got, err)
	}
	got[0] = 'x'
	got, err = overlay.ReadFile(path)
	if err != nil || string(got) != "buffer" {
		t.Fatalf("overlay mutated = %q, %v", got, err)
	}
	if !overlay.Has(path) || overlay.Len() != 1 {
		t.Fatal("overlay membership")
	}
}

func TestOverlayUpdateEqualsDiskUpdate(t *testing.T) {
	for _, tc := range []struct{ name, file, initial, edited string }{
		{"body", "First.cls", "public class First { void run() { Integer x = 1; } }", "public class First { void run() { Integer x = 2; } }"},
		{"method", "First.cls", "public class First {}", "public class First { void added() {} }"},
		{"trigger", "First.trigger", "trigger First on Account (before insert) {}", "trigger First on Account (before insert) { Integer x = 1; }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeIncrementalSFDXProject(t, root, "")
			path := filepath.Join(root, "force-app/main/default/classes", tc.file)
			writeFile(t, path, tc.initial)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			initial, previous := BuildWithArtifacts(p, schema.Schema{})
			overlay := NewSourceOverlay(map[string]string{path: tc.edited})
			got, ok := TryUpdateApexFileWithOverlay(initial, path, p, overlay)
			if !ok {
				t.Fatal("overlay update declined")
			}
			artifacts, err := RefreshBuildArtifactsWithOverlay(got, &previous, overlay)
			if err != nil {
				t.Fatal(err)
			}
			if !artifacts.Volatile() {
				t.Fatal("overlay artifacts not volatile")
			}
			writeFile(t, path, tc.edited)
			want, ok, err := TryUpdateApexFilesCheckedWithLoadedProject(initial, []string{path}, nil, p)
			if err != nil || !ok {
				t.Fatalf("disk update: %v %v", ok, err)
			}
			got.sourceDigests.volatile = false
			if !reflect.DeepEqual(got, want) {
				t.Fatal("overlay index differs from disk update")
			}
		})
	}
}

func TestOverlayDeclines(t *testing.T) {
	root := t.TempDir()
	writeIncrementalSFDXProject(t, root, "")
	path := filepath.Join(root, "force-app/main/default/classes/First.cls")
	writeFile(t, path, "public class First {}")
	other := filepath.Join(root, "force-app/main/default/classes/Second.cls")
	writeFile(t, other, "public class Second {}")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx := Build(p, schema.Schema{})
	for name, value := range map[string]string{"parse": "public class First {", "duplicate": "public class Second {}"} {
		t.Run(name, func(t *testing.T) {
			if _, ok := TryUpdateApexFileWithOverlay(idx, path, p, NewSourceOverlay(map[string]string{path: value})); ok {
				t.Fatal("invalid update accepted")
			}
		})
	}
	withDiagnostics := idx
	withDiagnostics.Diagnostics = append([]diagnostic.Diagnostic(nil), diagnostic.Diagnostic{})
	if _, ok := TryUpdateApexFileWithOverlay(withDiagnostics, path, p, NewSourceOverlay(map[string]string{path: "public class First {}"})); ok {
		t.Fatal("index with diagnostics accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, ok := TryUpdateApexFileWithOverlay(idx, path, p, NewSourceOverlay(map[string]string{path: "public class First {}"})); ok {
		t.Fatal("absent file accepted")
	}
}

func TestOverlayNeverReadsOverlaidPathFromDisk(t *testing.T) {
	root := t.TempDir()
	writeIncrementalSFDXProject(t, root, "")
	first := filepath.Join(root, "force-app/main/default/classes/First.cls")
	second := filepath.Join(root, "force-app/main/default/classes/Second.cls")
	writeFile(t, first, "public class First {}")
	writeFile(t, second, "public class Second {}")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, previous := BuildWithArtifacts(p, schema.Schema{})
	overlay := NewSourceOverlay(map[string]string{first: "public class First { void run() {} }"})
	idx, ok := TryUpdateApexFileWithOverlay(idx, first, p, overlay)
	if !ok {
		t.Fatal("update declined")
	}
	if err := os.Chmod(first, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(first, 0600) })
	artifacts, err := RefreshBuildArtifactsWithOverlay(idx, &previous, overlay)
	if err != nil {
		t.Fatal(err)
	}
	if got := artifacts.Sources.Stats().PhysicalReadAttempts; got != 1 {
		t.Fatalf("read attempts = %d, want one overlay read and no read of unchanged file", got)
	}
}

func TestOverlayConflictingAliasesDecline(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "First.cls")
	alias := filepath.Join(root, "Alias.cls")
	writeFile(t, path, "public class First {}")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	o := NewSourceOverlay(map[string]string{path: "one", alias: "two"})
	if _, err := o.ReadFile(path); err == nil {
		t.Fatal("conflicting aliases accepted")
	}
	if _, err := o.ReadFile(alias); err == nil {
		t.Fatal("conflicting aliases accepted")
	}
}

func TestOverlayReadsThroughUnregisteredAlias(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(real, "First.cls")
	writeFile(t, path, "public class First {}")
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	o := NewSourceOverlay(map[string]string{path: "public class First { void buffered() {} }"})
	got, err := o.ReadFile(filepath.Join(link, "First.cls"))
	if err != nil || string(got) != "public class First { void buffered() {} }" {
		t.Fatalf("alias read = %q, %v", got, err)
	}
	other := filepath.Join(real, "Other.cls")
	writeFile(t, other, "public class Other {}")
	if got, err := o.ReadFile(filepath.Join(link, "Other.cls")); err != nil || string(got) != "public class Other {}" {
		t.Fatalf("unbuffered alias read = %q, %v", got, err)
	}
}
