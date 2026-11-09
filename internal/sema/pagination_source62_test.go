package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// hierarchy_setting_runas_pagination/ORG-RUN.txt: the HhBody project deployed
// and ran Database.getPaginationCursor(String) and (String, AccessLevel) at
// source API 62.0 and 67.0 (org.tsv rows 62.0 and 67.0, both routes).
func TestHierarchySettingRunAsPaginationProjectCompiles(t *testing.T) {
	fixture := "../apextest/testdata/conformance/hierarchy_setting_runas_pagination/project"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			err := filepath.Walk(fixture, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(fixture, path)
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				target := filepath.Join(root, rel)
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				writeSemaFile(t, target, strings.ReplaceAll(string(raw), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(p, s)
			result := Analyze(index)
			for _, d := range append(index.Diagnostics, result.Diagnostics...) {
				t.Errorf("native deployed at %s; Glade %s %s", api, d.Code, d.Message)
			}
		})
	}
}
