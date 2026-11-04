package apextest_test

// Source-only proposal: depends on c12 capture and c14 two-column helpers.
// Exact capture role fields and same-invocation equalities only; no global masking.
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
)

type c11Q3Case struct {
	API    string            `json:"api"`
	Class  string            `json:"class"`
	Method string            `json:"method"`
	Marker string            `json:"marker"`
	Files  map[string]string `json:"files"`
}
type c11Q3Manifest struct {
	Cases []c11Q3Case `json:"cases"`
	TSV   string      `json:"tsv_sha256"`
}

func c11Q3ID(id string) bool {
	if len(id) != 18 {
		return false
	}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	for k := 0; k < 3; k++ {
		bits := 0
		for j := 0; j < 5; j++ {
			v := id[k*5+j]
			if v >= 'A' && v <= 'Z' {
				bits |= 1 << j
			}
		}
		if id[15+k] != alphabet[bits] {
			return false
		}
	}
	return true
}
func c11Q3RepairProject(o map[string]any) error {
	v, err := c12At(o, "/rows/0/OwnedChildren__r/0/Name")
	if err != nil || v != "hidden-child" {
		return fmt.Errorf("child source role missing")
	}
	groups := []struct {
		token string
		paths []string
	}{
		{"<org>", []string{"/orgId"}}, {"<actor>", []string{"/userId"}}, {"<profile>", []string{"/profileId"}},
		{"<parent>", []string{"/expectedParentId", "/childProjection/0/parentId", "/rows/0/Id", "/rows/0/OwnedChildren__r/0/Parent__c"}},
		{"<child>", []string{"/expectedChildId", "/childProjection/0/childIds/0", "/rows/0/OwnedChildren__r/0/Id"}},
	}
	seen := map[string]bool{}
	for _, g := range groups {
		id := ""
		for _, path := range g.paths {
			v, err := c12At(o, path)
			if err != nil {
				return err
			}
			s, ok := v.(string)
			if !ok || !c11Q3ID(s) {
				return fmt.Errorf("invalid role ID")
			}
			if id != "" && id != s {
				return fmt.Errorf("unequal role IDs")
			}
			id = s
		}
		if seen[id] {
			return fmt.Errorf("distinct roles equal")
		}
		seen[id] = true
		for _, path := range g.paths {
			if err := c12Set(o, path, g.token); err != nil {
				return err
			}
		}
	}
	return nil
}
func TestC11Q3RepairAdmin(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "c11_q3_repair_admin")
	raw := c12Read(t, filepath.Join(dir, "provenance.json"))
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != "239d8c52755b4eac68e00c6ba3757b6df64b76405622010dc5c47aab4432db77" {
		t.Fatal("provenance mismatch")
	}
	var spec c11Q3Manifest
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	tsv := c12Read(t, filepath.Join(dir, "org.tsv"))
	if fmt.Sprintf("%x", sha256.Sum256(tsv)) != spec.TSV {
		t.Fatal("oracle mismatch")
	}
	rows, order, err := c14ReadTwoColumnRows(tsv)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{}
	for _, c := range spec.Cases {
		want = append(want, c.API+"/"+c.Method)
	}
	if !reflect.DeepEqual(order, want) {
		t.Fatal("inventory mismatch")
	}
	for _, c := range spec.Cases {
		t.Run(c.API, func(t *testing.T) {
			if (c.API != "62.0" && c.API != "67.0") || c.Method != "childProjection" {
				t.Fatal("unexpected selection")
			}
			root := t.TempDir()
			c12CopyProject(t, filepath.Join(dir, "project", c.API), root, c.Files)
			index := c12LoadIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatal("source diagnostics")
			}
			c12VerifyContext(t, index, c.Class, "", c.API)
			run := func(method string) testreport.Case {
				r := apextest.Run(index, apextest.Options{SelectedClasses: []string{c.Class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
				if len(r.Suites) != 1 || len(r.Suites[0].Cases) != 1 {
					t.Fatal("selection mismatch")
				}
				return r.Suites[0].Cases[0]
			}
			b := run("baseline")
			if c11ActorGateCarry(t, b, c.Class, "baseline", "admin") {
				t.Log("known local carry batch Dot stdlib cases Dot Dot SOQL cases and Dot async cases: actor profile prerequisite prevents native observation")
				apextest.Case345Skip(t)
			}
			if b.ClassName != c.Class || b.MethodName != "baseline" || b.Status != testreport.StatusPass || b.Problem != nil {
				t.Fatalf("baseline failed: %s", c11Diagnostic(b))
			}
			raw, err := c12CapturePayload(run(c.Method), c.Class, c.Method, c.Marker)
			if err != nil {
				t.Fatal(err)
			}
			got, err := c12JSONObject(raw)
			if err != nil {
				t.Fatal(err)
			}
			if err = c11Q3RepairProject(got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, rows[c.API+"/"+c.Method]) {
				t.Fatal("projected observation differs")
			}
		})
	}
}
