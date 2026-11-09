package enterprise

import (
	"github.com/glade-sh/glade/internal/project"
	"testing"
)

func TestCountReportAndDashboardMetadata(t *testing.T) {
	p := project.Project{ReportFiles: []string{"one.report-meta.xml", "two.report"}, DashboardFiles: []string{"one.dashboard-meta.xml"}}
	if got := CountMetadataFiles(p); got != 3 {
		t.Fatalf("metadata files = %d, want 3", got)
	}
}
