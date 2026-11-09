package gladecli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// anonymousLocalMethodsC4Carry lists the rows Glade does not match yet, with
// the Glade outcome the test pins. class_calls_method: a local class method
// calling a block method; Glade hoists local classes to top-level types, so
// the call does not resolve.
var anonymousLocalMethodsC4Carry = map[string]string{
	"class_calls_method": `GLADESEMA008: method "go" calls unknown method "helper"`,
}

// The rows in ../apextest/testdata/conformance/anonymous_local_methods_c4
// (captured 2026-11-07, sf apex run at API 62.0 and 67.0): anonymous blocks
// that declare methods (static, void, returning, overloaded, recursive,
// forward-referenced, with modifiers, reading and writing top-level
// variables, beside a local class) and call them. DEBUG rows list the CAP|lm|
// debug values in order; COMPILE rows give the native line, column and text.
func TestAnonymousLocalMethodsC4OrgRows(t *testing.T) {
	dir := filepath.Join("..", "apextest", "testdata", "conformance", "anonymous_local_methods_c4")
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 51 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		api, name, outcome := row[0], row[1], row[2]
		t.Run(api+"/"+name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(dir, "anon", "lm_"+name+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			if err := os.MkdirAll(filepath.Join(root, "force-app"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(root)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"exec", "--project", root, "--json", string(source)}, &stdout, &stderr)
			if carry, ok := anonymousLocalMethodsC4Carry[name]; ok {
				if code == 0 || !strings.Contains(stderr.String(), carry) {
					t.Fatalf("carry changed: native %s; Glade code=%d stdout=%s stderr=%s", outcome, code, stdout.String(), stderr.String())
				}
				t.Logf("known carry: native %s; Glade %s", outcome, carry)
				return
			}
			kind, detail, _ := strings.Cut(outcome, "|")
			switch kind {
			case "DEBUG":
				if code != 0 {
					t.Fatalf("native %s; Glade code=%d stderr=%s", outcome, code, stderr.String())
				}
				var envelope struct {
					Data struct {
						Debug []string `json:"debug"`
					} `json:"data"`
				}
				if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
					t.Fatalf("%v: %s", err, stdout.String())
				}
				var got []string
				for _, line := range envelope.Data.Debug {
					if rest, ok := strings.CutPrefix(line, "CAP|lm|"); ok {
						got = append(got, rest)
					}
				}
				encoded, _ := json.Marshal(got)
				if string(encoded) != detail {
					t.Fatalf("native %s; Glade %s", detail, encoded)
				}
			case "COMPILE":
				if code == 0 {
					t.Fatalf("native %s; Glade compiled and ran: %s", outcome, stdout.String())
				}
				parts := strings.SplitN(detail, "|", 3)
				line, _ := strconv.Atoi(parts[0])
				column, _ := strconv.Atoi(parts[1])
				analysis := sema.AnalyzeAnonymous(typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: api}}, string(source), api)
				if len(analysis.Diagnostics) == 0 {
					t.Fatalf("native %s; Glade sema reports nothing (exec stderr %s)", outcome, stderr.String())
				}
				first := analysis.Diagnostics[0]
				if first.NativeMessage != parts[2] || first.Range == nil || first.Range.Start.Line != line || first.Range.Start.Column != column {
					t.Fatalf("native %s; Glade %+v range %+v", outcome, first, first.Range)
				}
			default:
				t.Fatalf("unknown outcome %q", outcome)
			}
		})
	}
}
