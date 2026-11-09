package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestReportTypedExceptionsNativeSema(t *testing.T) {
	const dir = "../resource/testdata/report_rows_capture/typed-exceptions"
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
	for _, row := range rows[1:] {
		api, route, name := row[0], row[1], row[2]
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			var body string
			switch name {
			case "invalid-message":
				body = `Reports.InvalidFilterException e = new Reports.InvalidFilterException('Synthetic invalid');String x=e.getMessage();`
			case "metadata-message-null-guard":
				body = `Reports.MetadataException e=null;if(e!=null)System.debug(e.getMessage());`
			case "run-message-null-guard":
				body = `Reports.ReportRunException e=null;if(e!=null)System.debug(e.getMessage());`
			case "missing-method":
				body = `Reports.InvalidFilterException e = new Reports.InvalidFilterException('Synthetic invalid');e.b6MissingMethod();`
			case "metadata-string-constructor":
				body = `Reports.MetadataException e = new Reports.MetadataException('Synthetic metadata');`
			default:
				t.Fatalf("unknown native case %s", name)
			}
			root := t.TempDir()
			idx := typesys.Build(project.Project{Root: root, SourceAPIVersion: api}, schema.Schema{})
			var result Result
			if route == "anonymous" {
				result = AnalyzeAnonymous(idx, body, api)
			} else {
				path := filepath.Join(root, "B6Typed.cls")
				if err := os.WriteFile(path, []byte(`@IsTest private class B6Typed { @IsTest static void check(){`+body+`} }`), 0600); err != nil {
					t.Fatal(err)
				}
				idx = typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				result = Analyze(idx)
			}
			reject := name == "missing-method" || name == "metadata-string-constructor"
			if result.HasErrors() != reject {
				t.Fatalf("native reject=%v; diagnostics=%+v", reject, result.Diagnostics)
			}
		})
	}
}
