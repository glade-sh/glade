package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

func TestSObjectConstructorValuesNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/sobject_constructor_values/"
	var rows []struct {
		API, Route, ID, ClassName, Source, SourceSHA256, Context string
		Accepted                                                 bool
		Problems                                                 []string
	}
	raw, err := os.ReadFile(dir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	native, err := os.ReadFile(dir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 40 {
		t.Fatal("incomplete constructor value observations")
	}
	seen := map[string]bool{}
	for _, row := range rows {
		t.Run(row.API+"/"+row.Route+"/"+row.ID, func(t *testing.T) {
			key := row.API + "/" + row.Route + "/" + row.ID
			if seen[key] || (row.API != "62.0" && row.API != "67.0") || row.Context != "full-field-access" {
				t.Fatal("duplicate or uncaptured constructor cell")
			}
			seen[key] = true
			sum := sha256.Sum256([]byte(row.Source))
			if hex.EncodeToString(sum[:]) != row.SourceSHA256 {
				t.Fatal("captured constructor source changed")
			}
			accepted := "false"
			if row.Accepted {
				accepted = "true"
			}
			bound := false
			for _, line := range strings.Split(string(native), "\n") {
				fields := strings.SplitN(line, "\t", 6)
				if len(fields) != 6 || fields[0] != row.API || fields[1] != row.Route || fields[2] != row.ID || fields[3] != accepted || fields[5] != row.SourceSHA256 {
					continue
				}
				var problems []string
				bound = json.Unmarshal([]byte(fields[4]), &problems) == nil && reflect.DeepEqual(problems, row.Problems)
			}
			if !bound {
				t.Fatal("constructor answer missing from native rows")
			}
			classes := map[string]string{}
			if row.Route == "named" {
				classes[row.ClassName] = row.Source
			} else if row.Route != "anonymous" {
				t.Fatal("uncaptured constructor route")
			}
			index := nativeBoundaryIndex(t, row.API, classes, nil)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			analysis := sema.Analyze(index)
			if row.Route == "anonymous" {
				analysis = sema.AnalyzeAnonymous(index, row.Source, row.API)
			}
			problems := []string{}
			for _, d := range analysis.Diagnostics {
				if d.Severity == diagnostic.Error {
					text := d.NativeMessage
					if text == "" {
						text = d.Message
					}
					problems = append(problems, text)
				}
			}
			if row.Accepted != !analysis.HasErrors() || !reflect.DeepEqual(row.Problems, problems) {
				t.Fatalf("native accepted=%v problems=%v; Glade accepted=%v problems=%v", row.Accepted, row.Problems, !analysis.HasErrors(), problems)
			}
		})
	}
}
