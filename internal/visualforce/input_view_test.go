package visualforce

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/inputview"
	"github.com/glade-sh/glade/internal/project"
)

func TestPageNamesReadsThroughProjectView(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "src", "pages", "Home.page")
	if err := os.MkdirAll(filepath.Dir(page), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, []byte(`<apex:page>before</apex:page>`), 0600); err != nil {
		t.Fatal(err)
	}
	v := inputview.New()
	p, err := project.LoadWithInputs(root, v)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.ReadFile(page); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, []byte(`<apex:page>after</apex:page>`), 0600); err != nil {
		t.Fatal(err)
	}
	_ = PageNames(p)
	if v.Err() == nil {
		t.Fatal("page loader bypassed the pinned body")
	}
}
