package resource

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestOpportunityReportTimeFrameSnapshot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	base := filepath.Join(root, "force-app/main/default/reports/B6_Reports")
	capture := "testdata/report_rows_capture/opportunity-timeframe/reports/B6_Reports"
	for _, name := range []string{"Inside", "Future", "Open"} {
		data, err := os.ReadFile(filepath.Join(capture, "B6_"+name+".report-meta.xml"))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(base, "B6_"+name+".report-meta.xml"), string(data))
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.OrgState
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	for _, r := range restored.Metadata.ProjectReports {
		if !r.Supported || r.DateColumn != "CREATED_DATE" {
			t.Fatalf("restored %s: %#v", r.DeveloperName, r)
		}
		switch r.DeveloperName {
		case "B6_Inside":
			if r.StartDate != "2000-01-01" || r.EndDate != "2100-01-01" {
				t.Fatalf("inside: %#v", r)
			}
		case "B6_Future":
			if r.StartDate != "2099-01-01" || r.EndDate != "2100-01-01" {
				t.Fatalf("future: %#v", r)
			}
		case "B6_Open":
			if r.StartDate != "" || r.EndDate != "" {
				t.Fatalf("open: %#v", r)
			}
		default:
			t.Fatalf("unexpected report %s", r.DeveloperName)
		}
	}
	if len(restored.Metadata.ProjectReports) != 3 {
		t.Fatalf("report count %d", len(restored.Metadata.ProjectReports))
	}
}
