package project

import (
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
