package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadFileExternalNamespaces(t *testing.T) {
	for _, tc := range []struct {
		yaml string
		want []string
		err  string
	}{
		{yaml: "project:\n  externalNamespaces: [ccrz, \"pkgOne\", pkg_two]\n", want: []string{"ccrz", "pkgOne", "pkg_two"}},
		{yaml: "project:\n  externalNamespaces: []\n", want: nil},
		{yaml: "project:\n  externalNamespaces: [ccrz, CCRZ]\n", err: "duplicate namespace"},
		{yaml: "project:\n  externalNamespaces: [\"ccrz:src\"]\n", err: "invalid namespace"},
		{yaml: "project:\n  externalNamespaces: [ns__c]\n", err: "invalid namespace"},
		{yaml: "project:\n  externalNamespaces: [1ns]\n", err: "invalid namespace"},
		{yaml: "project:\n  externalNamespaces: ccrz\n", err: "expected inline list"},
	} {
		root := t.TempDir()
		path := filepath.Join(root, "glade.yml")
		if err := os.WriteFile(path, []byte(tc.yaml), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadFile(path)
		if tc.err != "" {
			if err == nil || !strings.Contains(err.Error(), tc.err) || !strings.Contains(err.Error(), "project.externalNamespaces") {
				t.Errorf("%q: err %v, want %q", tc.yaml, err, tc.err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.yaml, err)
		}
		if len(tc.want) == 0 && len(cfg.Project.ExternalNamespaces) == 0 {
			continue
		}
		if !reflect.DeepEqual(cfg.Project.ExternalNamespaces, tc.want) {
			t.Errorf("%q: externalNamespaces=%v, want %v", tc.yaml, cfg.Project.ExternalNamespaces, tc.want)
		}
	}
}
