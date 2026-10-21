package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Both native routes reject the direct and caught publishing calls when
// Knowledge is disabled. A caught runtime exception requires compilation first.
func TestPublishingVisibilityOrgConformance(t *testing.T) {
	base := "testdata/conformance/publishing_visibility"
	var sources []struct {
		API, ID, ClassName, Anonymous, Named string
	}
	raw, err := os.ReadFile(filepath.Join(base, "sources.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &sources); err != nil {
		t.Fatal(err)
	}
	if len(sources) != 9 {
		t.Fatalf("native sources = %d, want 9", len(sources))
	}
	definition, err := os.ReadFile(filepath.Join(base, "scratch-def.json"))
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(base, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 19 || fmt.Sprint(rows[0]) != "[api route id terminal line message]" {
		t.Fatalf("unexpected native table: %d rows", len(rows))
	}
	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		api, route, id := row[0], row[1], row[2]
		key := api + "/" + id + "/" + route
		if seen[key] || row[3] != "COMPILE_ERROR" || row[4] != "12" || row[5] != "Type is not visible: KbManagement.PublishingService" {
			t.Fatalf("invalid native row: %v", row)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			var className, source string
			for _, candidate := range sources {
				if candidate.API == api && candidate.ID == id {
					className = candidate.ClassName
					switch route {
					case "anonymous":
						source = candidate.Anonymous
					case "isTest":
						source = candidate.Named
					}
				}
			}
			if source == "" {
				t.Fatalf("missing source for %s", key)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(definition))
			proj := project.Project{Root: root, SourceAPIVersion: api}
			if route == "isTest" {
				path := filepath.Join(root, className+".cls")
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				proj.ApexFiles = []string{path}
			}
			index := typesys.Build(proj, gladeschema.Schema{})
			if index.HasErrors() {
				t.Fatalf("source parser: %v", index.Diagnostics)
			}
			var result sema.Result
			if route == "anonymous" {
				result = sema.AnalyzeAnonymous(index, source, api)
			} else {
				result = sema.Analyze(index)
			}
			observed := "ACCEPTED"
			for _, d := range result.Diagnostics {
				if d.Severity == diagnostic.Error {
					line := 0
					if d.Range != nil {
						line = d.Range.Start.Line
					}
					observed = fmt.Sprintf("COMPILE_ERROR line %d: %s", line, d.Message)
					break
				}
			}
			line, err := strconv.Atoi(row[4])
			if err != nil {
				t.Fatal(err)
			}
			want := fmt.Sprintf("%s line %d: %s", row[3], line, row[5])
			if observed != want {
				t.Fatalf("native expected <%s> actual <%s>", want, observed)
			}
		})
	}
}
