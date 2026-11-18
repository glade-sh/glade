package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/inputview"
	"github.com/glade-sh/glade/internal/project"
)

func TestProjectDataWalkPinsMembership(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sfdx-project.json"), []byte(`{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "src"), 0700); err != nil {
		t.Fatal(err)
	}
	data := filepath.Join(root, "outside", strings.Repeat("deep/", 14), "data")
	if err := os.MkdirAll(data, 0700); err != nil {
		t.Fatal(err)
	}
	v := inputview.New()
	p, err := project.LoadWithInputs(root, v)
	if err != nil {
		t.Fatal(err)
	}
	_ = projectReferencedRecordTypes(p)
	if err := os.WriteFile(filepath.Join(data, "records.json"), []byte(`[]`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := v.Recheck(); err == nil {
		t.Fatal("data file addition was not observed")
	}
}
