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
