package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

func TestMessagingCampaignFieldPresenceNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/messaging_campaign_presence"
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("native rows=%d", len(rows)-1)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "scratch-def.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(cfg))
			body := `System.assertEquals(` + row[1] + `,Schema.getGlobalDescribe().get('MessagingSession').getDescribe().fields.getMap().containsKey('CampaignId'));`
			runStandardFieldEvidenceRoutes(t, root, row[0], body)
		})
	}
}

func TestMessagingCampaignFieldVersionNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/messaging_campaign_presence"
	f, err := os.Open(filepath.Join(dir, "versions.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 13 {
		t.Fatalf("native rows=%d", len(rows)-1)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "scratch-def.json"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(rows); i += 2 {
		a, b := rows[i], rows[i+1]
		if a[0] != b[0] || a[1] != "anonymous" || b[1] != "istest" || a[2] != b[2] {
			t.Fatalf("invalid route pair: %v %v", a, b)
		}
		t.Run(a[0], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+a[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(cfg))
			body := `System.assertEquals(` + a[2] + `,Schema.getGlobalDescribe().get('MessagingSession').getDescribe().fields.getMap().containsKey('CampaignId'));`
			runStandardFieldEvidenceRoutes(t, root, a[0], body)
		})
	}
}
