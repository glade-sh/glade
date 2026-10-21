package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

// These named check-only deployments isolate the start return contract from
// execute scope variance. Rejected declarations cannot have a runtime route.
func TestBatchableStartOrgConformance(t *testing.T) {
	var data struct {
		Objects map[string]string
		Rows    []struct {
			API, ID, ClassName, Source, SourceSHA256 string
			Accepted                                 bool
			Problems                                 []string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/batchable_start/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 31 || len(data.Objects) != 1 {
		t.Fatal("incomplete native batch start matrix")
	}
	for _, row := range data.Rows {
		t.Run(row.API+"/"+row.ID, func(t *testing.T) {
			sum := sha256.Sum256([]byte(row.Source))
			if hex.EncodeToString(sum[:]) != row.SourceSHA256 {
				t.Fatal("captured declaration changed")
			}
			path := filepath.Join("testdata/conformance/batchable_start", "api"+strings.TrimSuffix(row.API, ".0"), "org.tsv")
			native, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			accepted := "false"
			if row.Accepted {
				accepted = "true"
			}
			bound := false
			for _, line := range strings.Split(string(native), "\n") {
				fields := strings.SplitN(line, "\t", 4)
				if len(fields) != 4 || fields[0] != row.API || fields[1] != row.ID || fields[2] != accepted {
					continue
				}
				var problems []string
				if json.Unmarshal([]byte(fields[3]), &problems) == nil && reflect.DeepEqual(problems, row.Problems) {
					bound = true
				}
			}
			if !bound {
				t.Fatal("declaration missing from native rows")
			}
			index := nativeBoundaryIndex(t, row.API, map[string]string{row.ClassName: row.Source}, data.Objects)
			if index.HasErrors() {
				t.Fatalf("declaration parse failed: %v", index.Diagnostics)
			}
			analysis := sema.Analyze(index)
			problems := []string{}
			for _, item := range analysis.Diagnostics {
				if item.Severity != diagnostic.Error {
					continue
				}
				text := item.NativeMessage
				if text == "" {
					text = item.Message
				}
				problems = append(problems, text)
			}
			if row.Accepted != !analysis.HasErrors() || !reflect.DeepEqual(problems, row.Problems) {
				t.Fatalf("native accepted=%v problems=%v; local accepted=%v problems=%v", row.Accepted, row.Problems, !analysis.HasErrors(), problems)
			}
		})
	}
}
