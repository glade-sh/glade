package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/inputview"
)

func TestLoadWithInputsCarriesView(t *testing.T) {
	v := inputview.New()
	p, err := LoadWithInputs(t.TempDir(), v)
	if err != nil {
		t.Fatal(err)
	}
	if p.Inputs() != v {
		t.Fatal("loaded project lost its input view")
	}
	if _, err := p.Stat(p.Root); err != nil {
		t.Fatal(err)
	}
	if err := v.Recheck(); err != nil {
		t.Fatal(err)
	}
}

func TestMetadataReadersDefaultToProjectInputs(t *testing.T) {
	for _, kind := range []string{"email", "muting"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "metadata.xml")
			if err := os.WriteFile(path, []byte("<metadata/>"), 0600); err != nil {
				t.Fatal(err)
			}
			v := inputview.New()
			p := Project{inputs: v}
			if kind == "email" {
				p.EmailServiceFiles = []string{path}
				LoadEmailServicesTolerant(p, nil)
			} else {
				p.MutingPermissionSetFiles = []string{path}
				PermissionMutingRejections(p, nil)
			}
			if err := os.WriteFile(path, []byte("<changed/>"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := v.Recheck(); err == nil {
				t.Fatal("metadata read was not observed")
			}
		})
	}
}

func TestLoadWithInputsObservesSourceDirectoryMembership(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "force-app", "main", "default", "objects")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	v := inputview.New()
	if _, err := LoadWithInputs(root, v); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Added.object-meta.xml"), []byte("<CustomObject/>"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := v.Recheck(); err == nil {
		t.Fatal("source directory membership was not observed")
	}
}

func TestSetupMetadataRootsObservesAbsentStages(t *testing.T) {
	root := t.TempDir()
	v := inputview.New()
	p := Project{Root: root, inputs: v}
	if roots := p.SetupMetadataRoots(); len(roots) != 0 {
		t.Fatalf("unexpected roots: %v", roots)
	}
	if err := os.MkdirAll(filepath.Join(root, "unpackaged", "pre"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := v.Recheck(); err == nil {
		t.Fatal("new setup stage was not observed")
	}
}
