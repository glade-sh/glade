package storage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPersonAccountFormulaNativeSchemaRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/person_account_formulas/"
	raw, err := os.ReadFile(fixture + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases struct{ Files map[string]string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	cases.Files["sfdx-project.json"] = `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`
	for name, body := range cases.Files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	registry := sobject.BuildDescribeRegistry(s)
	raw, err = os.ReadFile(fixture + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		row := strings.SplitN(line, "\t", 4)
		field := map[string]string{"describe_formula": "Derived", "describe_context": "Context", "describe_chained": "Chained"}[row[2]]
		if field == "" {
			continue
		}
		var want []string
		if err := json.Unmarshal([]byte(row[3]), &want); err != nil || len(want) != 2 {
			t.Fatalf("invalid row: %s", line)
		}
		checked++
		for _, route := range []string{"new_org", "reapply", "frozen_clone"} {
			t.Run(strings.Join(row[:3], "/")+"/"+route, func(t *testing.T) {
				org := storage.NewOrgState()
				org.APIVersion = row[0]
				for name, describe := range registry.Objects {
					org.Objects[name] = storage.ObjectState{Definition: sobject.ToObjectDefinition(describe)}
				}
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				if route == "frozen_clone" {
					org = org.CloneRuntimeFrozenDefinition()
				}
				if route != "new_org" {
					storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
				}
				for i, name := range []string{"Account", "Contact"} {
					suffix := "__pc"
					if name == "Contact" {
						suffix = "__c"
					}
					if got := org.Objects[name].Definition.Fields[field+suffix].Formula; got != want[i] {
						t.Errorf("%s.%s: native=%q; Glade=%q", name, field+suffix, want[i], got)
					}
				}
			})
		}
	}
	if checked != 12 {
		t.Fatalf("formula rows=%d, want 12", checked)
	}
}
