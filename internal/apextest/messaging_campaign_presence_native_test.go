package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
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

func TestMessagingCampaignFieldNativeFacets(t *testing.T) {
	const dir = "testdata/conformance/messaging_campaign_presence"
	f, err := os.Open(filepath.Join(dir, "facets.tsv"))
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
			body := `Schema.DescribeFieldResult d=Schema.getGlobalDescribe().get('MessagingSession').getDescribe().fields.getMap().get('CampaignId').getDescribe(); System.assertEquals('CampaignId',d.getName()); System.assertEquals('Campaign ID',d.getLabel()); System.assertEquals('REFERENCE',String.valueOf(d.getType())); System.assertEquals(18,d.getLength()); System.assertEquals(true,d.isNillable()); System.assertEquals(true,d.isCreateable()); System.assertEquals(true,d.isUpdateable()); System.assertEquals(true,d.isFilterable()); System.assertEquals(true,d.isSortable()); System.assertEquals(true,d.isGroupable()); System.assertEquals(true,d.isAggregatable()); System.assertEquals(true,d.isPermissionable()); System.assertEquals('Campaign',d.getRelationshipName()); System.assertEquals(false,d.isCascadeDelete()); System.assertEquals(false,d.isRestrictedDelete());`
			runStandardFieldEvidenceRoutes(t, root, row[0], body)
		})
	}
}

func TestMessagingCampaignFieldNativeCompileAndQueryRoutes(t *testing.T) {
	const dir = "testdata/conformance/messaging_campaign_presence"
	f, err := os.Open(filepath.Join(dir, "routes.tsv"))
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
	cfg, err := os.ReadFile(filepath.Join(dir, "scratch-def.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		if row[1] == "rest-describe" {
			continue
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(cfg))
			if strings.HasPrefix(row[1], "named-") {
				tag := "Valid"
				if row[1] == "named-invalid" {
					tag = "Invalid"
				}
				name := "MsgRoute" + tag + strings.TrimSuffix(row[0], ".0")
				for _, s := range []string{".cls", ".cls-meta.xml"} {
					rel := filepath.Join("force-app/main/default/classes", name+s)
					b, err := os.ReadFile(filepath.Join(dir, "routes-project", rel))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, rel), string(b))
				}
				idx := loadTestIndex(t, root)
				analysis := sema.Analyze(idx)
				got := !idx.HasErrors() && !analysis.HasErrors()
				if got != (row[2] == "accept") {
					t.Fatalf("compile accepted=%t native=%s diagnostics=%v %v", got, row[2], idx.Diagnostics, analysis.Diagnostics)
				}
				return
			}
			idx := loadTestIndex(t, root)
			source, err := os.ReadFile(filepath.Join(dir, map[string]string{"anonymous": "anonymous.apex", "dynamic-query": "query.apex"}[row[1]]))
			if err != nil {
				t.Fatal(err)
			}
			if row[1] == "anonymous" {
				analysis := sema.AnalyzeAnonymous(idx, string(source), row[0])
				got := !analysis.HasErrors()
				if got != (row[2] == "success") {
					t.Fatalf("anonymous accepted=%t native=%s diagnostics=%v", got, row[2], analysis.Diagnostics)
				}
				return
			}
			body := `Database.query('SELECT CampaignId FROM MessagingSession LIMIT 1');`
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(idx)
			runner := newConformanceRunner(t, idx, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err = runner.execute(program); err != nil {
				t.Fatalf("native query succeeded: %v", err)
			}
		})
	}
}

func TestMessagingCampaignNativeQueryVersionRoutes(t *testing.T) {
	const dir = "testdata/conformance/messaging_campaign_presence"
	f, err := os.Open(filepath.Join(dir, "query-versions.tsv"))
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
	for _, row := range rows[1:] {
		row := row
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(cfg))
			if row[1] == "static-query" {
				name := "MsgStaticQuery" + strings.TrimSuffix(row[0], ".0")
				for _, s := range []string{".cls", ".cls-meta.xml"} {
					rel := filepath.Join("force-app/main/default/classes", name+s)
					b, err := os.ReadFile(filepath.Join(dir, "query-project", rel))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, rel), string(b))
				}
				idx := loadTestIndex(t, root)
				analysis := sema.Analyze(idx)
				got := !idx.HasErrors() && !analysis.HasErrors()
				if got != (row[2] == "accept") {
					t.Fatalf("static query accepted=%t native=%s diagnostics=%v %v", got, row[2], idx.Diagnostics, analysis.Diagnostics)
				}
				return
			}
			idx := loadTestIndex(t, root)
			program, err := vm.CompileAnonymousWithOptions(`Database.query('SELECT CampaignId FROM MessagingSession LIMIT 1');`, vm.CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(idx)
			runner := newConformanceRunner(t, idx, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err = runner.execute(program); err != nil {
				t.Fatalf("native dynamic query succeeded: %v", err)
			}
		})
	}
}

func TestMessagingCustomFieldNativeCompileRoutes(t *testing.T) {
	const dir = "testdata/conformance/messaging_campaign_presence"
	f, err := os.Open(filepath.Join(dir, "custom-fields.tsv"))
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
	if len(rows) != 9 {
		t.Fatalf("native rows=%d", len(rows)-1)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "scratch-def.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		row := row
		if row[1] == "MissingStd" {
			continue
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(cfg))
			field := filepath.Join("force-app/main/default/objects/MessagingSession/fields/OwnedProbe__c.field-meta.xml")
			b, err := os.ReadFile(filepath.Join(dir, "custom-project", field))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, field), string(b))
			name := "MsgCustom" + row[1] + strings.TrimSuffix(row[0], ".0")
			for _, s := range []string{".cls", ".cls-meta.xml"} {
				rel := filepath.Join("force-app/main/default/classes", name+s)
				b, err := os.ReadFile(filepath.Join(dir, "custom-project", rel))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, rel), string(b))
			}
			idx := loadTestIndex(t, root)
			analysis := sema.Analyze(idx)
			got := !idx.HasErrors() && !analysis.HasErrors()
			if got != (row[2] == "accept") {
				t.Fatalf("compile accepted=%t native=%s diagnostics=%v %v", got, row[2], idx.Diagnostics, analysis.Diagnostics)
			}
		})
	}
}

func TestMessagingCampaignFieldAPI68NativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/messaging_campaign_presence"
	f, err := os.Open(filepath.Join(dir, "api68.tsv"))
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
	if len(rows) != 3 || rows[1][2] != "true" || rows[2][2] != "true" {
		t.Fatalf("native rows=%v", rows)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, "scratch-def.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"68.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(cfg))
	runStandardFieldEvidenceRoutes(t, root, "68.0", `System.assertEquals(true,Schema.getGlobalDescribe().get('MessagingSession').getDescribe().fields.getMap().containsKey('CampaignId'));`)
}
