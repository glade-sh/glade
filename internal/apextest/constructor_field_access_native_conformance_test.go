package apextest

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
)

// Constructor typing is compared with native field access explicitly granted.
// The default-profile answers remain in the capture as permission observations.
func TestConstructorFieldAccessNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/constructor_field_visibility/"
	var cases []struct{ ID, Source string }
	raw, err := os.ReadFile(dir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	var names struct {
		Rows []struct{ ID, APIVersion, Route, Source, ExpectedCompilation, NativeContext string }
	}
	raw, err = os.ReadFile("testdata/conformance/name_resolution.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &names); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(dir + "org.tsv")
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
	if len(rows) != 11 || strings.Join(rows[0], ",") != "id,api,context,compiled,message,line,source_sha256" {
		t.Fatal("incomplete native field access matrix")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		key := row[0] + "/" + row[1] + "/" + row[2]
		if seen[key] || (row[1] != "62.0" && row[1] != "67.0") {
			t.Fatalf("invalid native cell %s", key)
		}
		seen[key] = true
		source := ""
		for _, c := range cases {
			if c.ID == row[0] {
				source = c.Source
			}
		}
		sum := sha256.Sum256([]byte(source))
		if source == "" || hex.EncodeToString(sum[:]) != row[6] {
			t.Fatalf("captured source changed: %s", key)
		}
		if row[2] == "default-field-access" {
			if row[0] != "original" || row[3] != "false" || row[4] != "Field does not exist: HasOptedOutOfEmail on Contact" {
				t.Fatalf("unexpected default-profile observation: %v", row)
			}
			continue
		}
		if row[2] != "explicit-field-access" {
			t.Fatalf("unknown native field access context: %s", key)
		}
		if row[0] == "original" {
			// The name-resolution harness splits transient class declarations
			// from this source and checks its unchanged executable line numbers.
			bound := false
			for _, r := range names.Rows {
				if r.ID == "S117" && r.Route == "anonymous" && r.APIVersion == row[1] {
					bound = r.Source == source && r.NativeContext != "" && r.ExpectedCompilation == "COMPILE_ERROR\tline "+row[5]+": "+row[4]
				}
			}
			if !bound {
				t.Fatalf("native answer missing from name-resolution conformance: %s", key)
			}
			continue
		}
		t.Run(key, func(t *testing.T) {
			analysis := sema.AnalyzeAnonymous(nativeBoundaryIndex(t, row[1], nil, nil), source, row[1])
			var problems []string
			for _, d := range analysis.Diagnostics {
				if d.Severity != diagnostic.Error {
					continue
				}
				message := d.NativeMessage
				if message == "" {
					message = d.Message
				}
				line := 0
				if d.Range != nil {
					line = d.Range.Start.Line
				}
				if d.NativeLine != nil {
					line = *d.NativeLine
				}
				problems = append(problems, strconv.Itoa(line)+": "+message)
			}
			want := ""
			if row[3] == "false" {
				want = row[5] + ": " + row[4]
			}
			if got := strings.Join(problems, "; "); got != want {
				t.Fatalf("native <%s>; Glade <%s>", want, got)
			}
		})
	}
	for _, api := range []string{"62.0", "67.0"} {
		if !seen["original/"+api+"/default-field-access"] {
			t.Fatalf("missing default-profile control at %s", api)
		}
		for _, c := range cases {
			if !seen[c.ID+"/"+api+"/explicit-field-access"] {
				t.Fatalf("missing granted field cell %s at %s", c.ID, api)
			}
		}
	}
}
