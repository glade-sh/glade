package apextest_test

// Source-only Q14 proposal. Depends on the separately staged c12 capture and
// c14 two-column helpers; no production transport is changed.
import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
)

type c11Q14Case struct {
	API        string            `json:"api"`
	Arm        string            `json:"actor_arm"`
	Class      string            `json:"class"`
	Method     string            `json:"method"`
	Marker     string            `json:"marker"`
	Key        string            `json:"key"`
	Projection string            `json:"projection"`
	Files      map[string]string `json:"files"`
}
type c11Q14Manifest struct {
	Schema int          `json:"schema"`
	TSV    string       `json:"tsv_sha256"`
	Cases  []c11Q14Case `json:"cases"`
}

func c11Q14ID(id string) bool {
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

// Only explicit source-gated identity slots are replaced, independently for
// each invocation. The direct admin rows are anchored by their emitted values.
func c11Q14Project(o map[string]any, mode string) error {
	groups := []struct{ path, token string }{{"/orgId", "<org>"}, {"/userId", "<actor>"}, {"/profileId", "<profile>"}}
	if mode == "direct-values" {
		a, err := c12At(o, "/rows/0/Value__c")
		if err != nil || a != json.Number("50") {
			return fmt.Errorf("first value anchor mismatch")
		}
		b, err := c12At(o, "/rows/1/Value__c")
		if err != nil || b != json.Number("100") {
			return fmt.Errorf("second value anchor mismatch")
		}
		groups = append(groups, struct{ path, token string }{"/rows/0/Id", "<row-value50>"}, struct{ path, token string }{"/rows/1/Id", "<row-value100>"})
	} else if mode != "identity" {
		return fmt.Errorf("unknown projection")
	}
	seen := map[string]bool{}
	for _, g := range groups {
		v, err := c12At(o, g.path)
		if err != nil {
			return err
		}
		id, ok := v.(string)
		if !ok || !c11Q14ID(id) || seen[id] {
			return fmt.Errorf("invalid or repeated identity slot")
		}
		seen[id] = true
		if err := c12Set(o, g.path, g.token); err != nil {
			return err
		}
	}
	// Exact-byte digest retains sensitivity to the full message without exporting
	// it. It is environment-bound if the private message contains dynamic text.
	if ex, exists := o["exception"]; exists {
		m, ok := ex.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid exception")
		}
		v, ok := m["message"].(string)
		if !ok {
			return fmt.Errorf("missing exception message")
		}
		m["message"] = fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(v)))
	}
	return nil
}
func TestC11CorrectedQ14(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "c11_q14_corrected")
	raw := c12Read(t, filepath.Join(dir, "provenance.json"))
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != "1b63277b7c6f4e0d4a7d98cc5ae5f14218449af84bff280376b8c56d2636aa92" {
		t.Fatal("provenance changed")
	}
	var spec c11Q14Manifest
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Schema != 1 || len(spec.Cases) != 32 {
		t.Fatal("case inventory")
	}
	tsv := c12Read(t, filepath.Join(dir, "org.tsv"))
	if fmt.Sprintf("%x", sha256.Sum256(tsv)) != spec.TSV {
		t.Fatal("oracle changed")
	}
	rows, order, err := c14ReadTwoColumnRows(tsv)
	if err != nil {
		t.Fatal(err)
	}
	expectedOrder := []string{}
	seen := map[string]bool{}
	for _, c := range spec.Cases {
		if (c.API != "62.0" && c.API != "67.0") || (c.Arm != "admin" && c.Arm != "restricted") || c.Key != c.API+"/"+c.Arm+"_"+c.Method || seen[c.Key] {
			t.Fatal("invalid selection")
		}
		seen[c.Key] = true
		expectedOrder = append(expectedOrder, c.Key)
	}
	if !reflect.DeepEqual(order, expectedOrder) {
		t.Fatal("oracle method order mismatch")
	}
	expectedMethods := []string{"allowedGroup", "baseline", "directDenied", "hiddenGroup", "hiddenHaving", "sum", "systemGroup", "systemHaving", "systemSum"}
	for _, c := range spec.Cases {
		c := c
		t.Run(c.Key, func(t *testing.T) {
			root := t.TempDir()
			c12CopyProject(t, filepath.Join(dir, "project", c.API), root, c.Files)
			index := c12LoadIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatal("source diagnostics")
			}
			file := c12VerifyContext(t, index, c.Class, "", c.API)
			found := apextest.Discover(index, apextest.Options{SelectedClasses: []string{c.Class}})
			names := []string{}
			for _, item := range found {
				if item.ClassName != c.Class || item.File != file {
					t.Fatal("source selection mismatch")
				}
				names = append(names, item.MethodName)
			}
			sort.Strings(names)
			if !reflect.DeepEqual(names, expectedMethods) {
				t.Fatal("source method inventory mismatch")
			}
			run := func(method string) testreport.Case {
				r := apextest.Run(index, apextest.Options{SelectedClasses: []string{c.Class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
				if len(r.Suites) != 1 || len(r.Suites[0].Cases) != 1 {
					t.Fatal("method selection mismatch")
				}
				return r.Suites[0].Cases[0]
			}
			b := run("baseline")
			if c11ActorGateCarry(t, b, c.Class, "baseline", c.Arm) {
				t.Log("known local carry batch Dot stdlib cases Dot Dot SOQL cases and Dot async cases: actor profile prerequisite prevents native observation")
				apextest.Case345Skip(t)
			}
			if b.ClassName != c.Class || b.MethodName != "baseline" || b.Status != testreport.StatusPass || b.Problem != nil {
				t.Fatalf("baseline failed: %s", c11Diagnostic(b))
			}
			result := run(c.Method)
			if c11ActorGateCarry(t, result, c.Class, c.Method, c.Arm) {
				t.Log("known local carry batch Dot stdlib cases Dot Dot SOQL cases and Dot async cases: actor profile prerequisite prevents native observation")
				apextest.Case345Skip(t)
			}
			payload, err := c12CapturePayload(result, c.Class, c.Method, c.Marker)
			if err != nil {
				t.Fatalf("capture transport mismatch: %v", err)
			}
			got, err := c12JSONObject(payload)
			if err != nil {
				t.Fatal("invalid capture JSON")
			}
			if err := c11Q14Project(got, c.Projection); err != nil {
				t.Fatal("projection failed")
			}
			if !reflect.DeepEqual(got, rows[c.Key]) {
				t.Fatal("projected capture differs from pinned native oracle")
			}
		})
	}
}
