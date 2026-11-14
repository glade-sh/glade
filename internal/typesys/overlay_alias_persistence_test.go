package typesys

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

func TestAliasedOverlayGenerationRejectsPersistence(t *testing.T) {
	root := t.TempDir()
	writeIncrementalSFDXProject(t, root, "")
	path := filepath.Join(root, "force-app/main/default/classes/First.cls")
	text := "public class First {}"
	writeFile(t, path, text)
	alias := filepath.Join(root, "Alias.cls")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	disk, artifacts := BuildWithArtifacts(p, schema.Schema{})
	for _, tc := range []struct{ name, key string }{{"alias", alias}, {"direct", path}} {
		t.Run(tc.name, func(t *testing.T) {
			overlay := NewSourceOverlay(map[string]string{tc.key: text})
			got, ok := TryUpdateApexFileWithOverlay(disk, path, p, overlay)
			if !ok {
				t.Fatal("update declined")
			}
			if !got.Volatile() {
				t.Error("overlay generation is not volatile")
			}
			if err := ValidateBuildGeneration(got, &artifacts); !errors.Is(err, ErrVolatileGeneration) {
				t.Errorf("build validation: %v", err)
			}
			if err := ValidateSourceGeneration(got, artifacts.SourceDigests); !errors.Is(err, ErrVolatileGeneration) {
				t.Errorf("source validation: %v", err)
			}
			if MatchesProjectSources(got, p) {
				t.Error("overlay matches disk")
			}
			if err := WriteFrontendSnapshot(&bytes.Buffer{}, got, &artifacts); !errors.Is(err, ErrVolatileGeneration) {
				t.Errorf("snapshot: %v", err)
			}
			if err := WriteFrontendSnapshotRecords(&bytes.Buffer{}, got, &artifacts); !errors.Is(err, ErrVolatileGeneration) {
				t.Errorf("records: %v", err)
			}
			if _, err := EncodeFrontendSnapshot(got, &artifacts); !errors.Is(err, ErrVolatileGeneration) {
				t.Errorf("encode: %v", err)
			}
		})
	}
	// A read of an unrelated file through the overlay retains disk behavior.
	other := filepath.Join(root, "Other.cls")
	writeFile(t, other, "public class Other {}")
	got, ok := TryUpdateApexFileWithOverlay(disk, path, p, NewSourceOverlay(map[string]string{other: "public class Other {}"}))
	if !ok || got.Volatile() {
		t.Fatalf("unrelated overlay: ok=%v volatile=%v", ok, got.Volatile())
	}
	want, ok, err := TryUpdateApexFilesCheckedWithLoadedProject(disk, []string{path}, nil, p)
	if err != nil || !ok {
		t.Fatalf("disk update: %v %v", ok, err)
	}
	var a, b bytes.Buffer
	if err := WriteFrontendSnapshot(&a, got, &artifacts); err != nil {
		t.Fatal(err)
	}
	if err := WriteFrontendSnapshot(&b, want, &artifacts); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("unrelated overlay changed disk snapshot")
	}
}

func TestAliasedOverlayVolatilitySurvivesSubsequentDiskUpdate(t *testing.T) {
	root := t.TempDir()
	writeIncrementalSFDXProject(t, root, "")
	first := filepath.Join(root, "force-app/main/default/classes/First.cls")
	second := filepath.Join(root, "force-app/main/default/classes/Second.cls")
	writeFile(t, first, "public class First {}")
	writeFile(t, second, "public class Second {}")
	alias := filepath.Join(root, "Alias.cls")
	if err := os.Symlink(first, alias); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	base, previous := BuildWithArtifacts(p, schema.Schema{})
	overlay := NewSourceOverlay(map[string]string{alias: "public class First { void added() {} }"})
	updated, ok := TryUpdateApexFileWithOverlay(base, first, p, overlay)
	if !ok || !updated.Volatile() {
		t.Fatalf("first update ok=%v volatile=%v", ok, updated.Volatile())
	}
	artifacts, err := RefreshBuildArtifactsWithOverlay(updated, &previous, overlay)
	if err != nil || !artifacts.Volatile() {
		t.Fatalf("overlay artifacts volatile=%v err=%v", artifacts.Volatile(), err)
	}
	next, ok := TryUpdateApexFileWithOverlay(updated, second, p, overlay)
	if !ok || !next.Volatile() {
		t.Fatalf("subsequent update ok=%v volatile=%v", ok, next.Volatile())
	}
	if base.Volatile() || previous.Volatile() {
		t.Fatal("base generation was mutated")
	}
	// Saving the same bytes does not make an existing overlay generation safe.
	writeFile(t, first, "public class First { void added() {} }")
	if err := WriteFrontendSnapshot(&bytes.Buffer{}, next, &artifacts); !errors.Is(err, ErrVolatileGeneration) {
		t.Fatalf("snapshot after disk catches up: %v", err)
	}
}
