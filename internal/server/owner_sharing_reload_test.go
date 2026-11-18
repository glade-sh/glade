package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native visibility oracle: no_rows_visibility/org.tsv:7,14,22,23.
func TestOwnerSharingReloadTransitions(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "no_rows_visibility", "project")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["PrivateProbe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "PrivateProbe__c", SharingModel: "Private"}}
	if err := resource.ApplyProjectGroups(&org, p); err != nil {
		t.Fatal(err)
	}
	s := New(&org)
	rules := p.OwnerSharingRuleFiles
	for _, step := range []struct {
		files []string
		want  int
	}{{nil, 0}, {rules, 2}, {nil, 0}} {
		p.OwnerSharingRuleFiles = step.files
		s.ReloadProjectState(SourceMetadata{Project: p}, typesys.Index{}, nil, nil)
		if s.runtimeErr != nil {
			t.Fatal(s.runtimeErr)
		}
		if got := len(org.Objects["PrivateProbe__c"].Definition.OwnerSharingRules); got != step.want {
			t.Fatalf("rules=%d want=%d", got, step.want)
		}
	}
}

func TestCriteriaSharingReloadWithUnsupportedRules(t *testing.T) {
	root := filepath.Join("..", "apextest", "testdata", "conformance", "case_phase", "project")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p.OwnerSharingRuleFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	unsupported := strings.Replace(string(data), "<operation>equals</operation>", "<operation>notEqual</operation>", 1)
	unsupported = strings.Replace(unsupported, "CasePhaseSubjectShare", "Unsupported", 1)
	start := strings.Index(unsupported, "<sharingCriteriaRules>")
	end := strings.Index(unsupported, "</sharingCriteriaRules>") + len("</sharingCriteriaRules>")
	dir := t.TempDir()
	casePath := filepath.Join(dir, "Case.sharingRules-meta.xml")
	accountPath := filepath.Join(dir, "Account.sharingRules-meta.xml")
	if err := os.WriteFile(casePath, []byte(strings.Replace(string(data), "</SharingRules>", unsupported[start:end]+"</SharingRules>", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(accountPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	p.OwnerSharingRuleFiles = []string{casePath, accountPath}
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Case")
	c := org.Objects["Case"]
	c.Definition.SharingModel = "Private"
	org.Objects["Case"] = c
	s := New(&org)
	s.ReloadProjectState(SourceMetadata{Project: p}, typesys.Index{}, nil, nil)
	if s.runtimeErr != nil {
		t.Fatal(s.runtimeErr)
	}
	if got := len(org.Objects["Case"].Definition.CriteriaSharingRules); got != 1 {
		t.Fatalf("supported rules = %d", got)
	}
}
