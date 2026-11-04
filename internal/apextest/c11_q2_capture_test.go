package apextest_test

// Unrun Q2 proposal. IDs are checked per source path; no cross-run identity is inferred.
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type c11q2Case struct {
	API     string `json:"api"`
	Class   string `json:"class"`
	Method  string `json:"method"`
	Arm     string `json:"actor_arm"`
	Marker  string `json:"marker"`
	Report  string `json:"report_sha256"`
	Pointer string `json:"report_pointer"`
	Payload string `json:"payload_sha256"`
}
type c11q2Manifest struct {
	Cases []c11q2Case       `json:"cases"`
	Files map[string]string `json:"files"`
	TSV   string            `json:"tsv_sha256"`
}

func c11q2Project(o map[string]any) error {
	seen := map[string]bool{}
	for path, prefix := range map[string]string{"/orgId": "00D", "/userId": "005", "/profileId": "00e"} {
		v, e := c12At(o, path)
		if e != nil {
			return e
		}
		id, ok := v.(string)
		if !ok || !originalValid18(id) || !strings.HasPrefix(id, prefix) || seen[id] {
			return fmt.Errorf("invalid identity %s", path)
		}
		seen[id] = true
		if e = c12Set(o, path, "<ID:REDACTED>"); e != nil {
			return e
		}
	}
	rows, ok := o["rows"].([]any)
	if !ok {
		return fmt.Errorf("missing rows")
	}
	for _, v := range rows {
		row, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid row")
		}
		id, ok := row["Id"].(string)
		if !ok || !originalValid18(id) || seen[id] {
			return fmt.Errorf("invalid or duplicate row ID")
		}
		seen[id] = true
		row["Id"] = "<ID:REDACTED>"
	}
	return nil
}
func TestC11Q2CaptureProposal(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "c11_q2")
	raw := c12Read(t, filepath.Join(dir, "provenance.json"))
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != "7279bf980764efcbdb0e9b06a7ba4605140e4a370a460fe4c8296d8e831b90ef" {
		t.Fatal("provenance changed")
	}
	var m c11q2Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	b := c12Read(t, filepath.Join(dir, "org.tsv"))
	if fmt.Sprintf("%x", sha256.Sum256(b)) != m.TSV {
		t.Fatal("oracle changed")
	}
	rows, order, err := c14ReadTwoColumnRows(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Cases) != 12 || len(order) != 12 {
		t.Fatal("inventory changed")
	}
	for i, c := range m.Cases {
		if order[i] != c.API+"/"+c.Class+"."+c.Method {
			t.Fatal("case order changed")
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		api := api
		t.Run(api, func(t *testing.T) {
			prefix := "project/" + api + "/"
			files := map[string]string{}
			for n, h := range m.Files {
				if strings.HasPrefix(n, prefix) {
					files[strings.TrimPrefix(n, prefix)] = h
				}
			}
			root := t.TempDir()
			c12CopyProject(t, filepath.Join(dir, "project", api), root, files)
			index := c12LoadIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatal("source diagnostics")
			}
			run := func(class, method string) testreport.Case {
				r := apextest.Run(index, apextest.Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
				if len(r.Suites) != 1 || len(r.Suites[0].Cases) != 1 {
					t.Fatal("selection mismatch")
				}
				return r.Suites[0].Cases[0]
			}
			for _, c := range m.Cases {
				if c.API != api {
					continue
				}
				c := c
				t.Run(c.Class+"/"+c.Method, func(t *testing.T) {
					c12VerifyContext(t, index, c.Class, "", api)
					base := run(c.Class, "baseline")
					if c11ActorGateCarry(t, base, c.Class, "baseline", c.Arm) {
						t.Log("known local carry batch Dot stdlib cases Dot Dot SOQL cases and Dot async cases: actor profile prerequisite prevents native observation")
						apextest.Case345Skip(t)
					}
					if base.Status != testreport.StatusPass || base.Problem != nil || base.ClassName != c.Class || base.MethodName != "baseline" {
						t.Fatalf("baseline failed: %s", c11Diagnostic(base))
					}
					result := run(c.Class, c.Method)
					if c11ActorGateCarry(t, result, c.Class, c.Method, c.Arm) {
						t.Log("known local carry batch Dot stdlib cases Dot Dot SOQL cases and Dot async cases: actor profile prerequisite prevents native observation")
						apextest.Case345Skip(t)
					}
					raw, e := c12CapturePayload(result, c.Class, c.Method, c.Marker)
					if e != nil {
						t.Fatal(e)
					}
					o, e := c12JSONObject(raw)
					if e != nil {
						t.Fatal(e)
					}
					if e = c11q2Project(o); e != nil {
						t.Fatal(e)
					}
					if !reflect.DeepEqual(o, rows[c.API+"/"+c.Class+"."+c.Method]) {
						t.Fatal("observation differs")
					}
				})
			}
		})
	}
}
