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

type c11q345Case struct {
	API     string   `json:"api"`
	Class   string   `json:"class"`
	Method  string   `json:"method"`
	Arm     string   `json:"actor_arm"`
	Marker  string   `json:"marker"`
	Report  string   `json:"report_sha256"`
	Pointer string   `json:"report_pointer"`
	Payload string   `json:"payload_sha256"`
	Route   string   `json:"route_id"`
	Paths   []string `json:"redacted_id_paths"`
	Omit    bool     `json:"exception_message_omitted_for_privacy"`
}
type c11q345Manifest struct {
	Cases []c11q345Case     `json:"cases"`
	Files map[string]string `json:"files"`
	TSV   string            `json:"tsv_sha256"`
}

func c11q345Project(o map[string]any, c c11q345Case) error {
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
	}
	if strings.Contains(c.Route, "-q3-") {
		parentValue, e := c12At(o, "/expectedParentId")
		if e != nil {
			return e
		}
		parent, ok := parentValue.(string)
		if !ok {
			return fmt.Errorf("invalid Q3 parent type")
		}
		pv, e := c12At(o, "/childProjection/0/parentId")
		p, ok := pv.(string)
		if e != nil || !ok || p != parent {
			return fmt.Errorf("Q3 parent mismatch")
		}
		if c.Method == "childProjection" {
			cv, e := c12At(o, "/expectedChildId")
			child, ok := cv.(string)
			if e != nil || !ok || child == parent {
				return fmt.Errorf("Q3 child equals parent")
			}
			v, e := c12At(o, "/rows/0/Id")
			rowID, ok := v.(string)
			if e != nil || !ok || rowID != parent {
				return fmt.Errorf("Q3 row parent mismatch")
			}
		} else {
			vv, e := c12At(o, "/expectedVisibleChildId")
			visible, ok := vv.(string)
			if e != nil || !ok {
				return fmt.Errorf("invalid visible child")
			}
			hv, e := c12At(o, "/expectedHiddenChildId")
			hidden, ok := hv.(string)
			if e != nil || !ok || hidden == visible || hidden == parent || visible == parent {
				return fmt.Errorf("Q3 seed IDs invalid")
			}
			v, e := c12At(o, "/childProjection/0/childIds")
			if e != nil {
				return e
			}
			a, ok := v.([]any)
			if !ok {
				return fmt.Errorf("invalid child IDs")
			}
			foundV, foundH := false, false
			for _, item := range a {
				id, ok := item.(string)
				if !ok {
					return fmt.Errorf("invalid child ID type")
				}
				if id == visible {
					foundV = true
				} else if id == hidden {
					foundH = true
				} else {
					return fmt.Errorf("unexpected child")
				}
			}
			if !foundV || foundH != (c.Arm == "admin") {
				return fmt.Errorf("Q3 child membership")
			}
		}
	}
	if strings.Contains(c.Route, "-q5-") && strings.HasPrefix(c.Method, "typeof") {
		if _, ok := o["rows"]; ok {
			av, e := c12At(o, "/rows/0/WhatId")
			a, ok := av.(string)
			if e != nil || !ok {
				return fmt.Errorf("invalid target type")
			}
			bv, e := c12At(o, "/rows/0/What/Id")
			b, ok := bv.(string)
			if e != nil || !ok || a != b {
				return fmt.Errorf("Q5 target mismatch")
			}
			tv, e := c12At(o, "/rows/0/Id")
			task, ok := tv.(string)
			if e != nil || !ok || task == a {
				return fmt.Errorf("Q5 task equals target")
			}
		}
	}
	for _, path := range c.Paths {
		v, e := c12At(o, path)
		if e != nil {
			return e
		}
		id, ok := v.(string)
		if !ok || !originalValid18(id) {
			return fmt.Errorf("invalid ID at %s", path)
		}
		if e = c12Set(o, path, "<ID:REDACTED>"); e != nil {
			return e
		}
	}
	if c.Omit {
		v, ok := o["exception"].(map[string]any)
		if !ok {
			return fmt.Errorf("missing exception")
		}
		message, ok := v["message"].(string)
		if !ok {
			return fmt.Errorf("missing exception message")
		}
		_ = message
		delete(v, "message")
	}
	return nil
}
func TestC11Q345CaptureProposal(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "c11_q345")
	raw := c12Read(t, filepath.Join(dir, "provenance.json"))
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != "22106c884ea57512d363b11d3228d8e6a56165a54c88f5d176a6e60b464f30bc" {
		t.Fatal("provenance changed")
	}
	var m c11q345Manifest
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
	if len(m.Cases) != 46 || len(order) != 46 {
		t.Fatal("inventory changed")
	}
	for i, c := range m.Cases {
		if order[i] != c.API+"/"+c.Class+"."+c.Method {
			t.Fatal("case order changed")
		}
	}
	for _, c := range m.Cases {
		c := c
		t.Run(c.Class+"/"+c.Method, func(t *testing.T) {
			prefix := "project/" + c.Route + "/"
			files := map[string]string{}
			for n, h := range m.Files {
				if strings.HasPrefix(n, prefix) {
					files[strings.TrimPrefix(n, prefix)] = h
				}
			}
			root := t.TempDir()
			c12CopyProject(t, filepath.Join(dir, "project", c.Route), root, files)
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
			base := run("baseline")
			if c11ActorGateCarry(t, base, c.Class, "baseline", c.Arm) {
				t.Log("known local carry batch Dot stdlib cases Dot Dot SOQL cases and Dot async cases: actor profile prerequisite prevents native observation")
				apextest.Case345Skip(t)
			}
			if base.Status != testreport.StatusPass || base.Problem != nil || base.ClassName != c.Class || base.MethodName != "baseline" {
				t.Fatalf("baseline failed: %s", c11Diagnostic(base))
			}
			result := run(c.Method)
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
			if e = c11q345Project(o, c); e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(o, rows[c.API+"/"+c.Class+"."+c.Method]) {
				t.Fatal("observation differs")
			}
		})
	}
}

func TestC11Q345RejectsInvalidRelations(t *testing.T) {
	identity := `"orgId":"00D000000000001EAA","userId":"005000000000001AAA","profileId":"00e000000000001AAA"`
	cases := []struct {
		route, method, raw string
		omit               bool
	}{
		{"-q3-", "childProjection", `{` + identity + `,"expectedParentId":[],"childProjection":[{"parentId":[]}]}`, false},
		{"q4", "denied", `{"orgId":"bad","userId":"005000000000001AAA","profileId":"00e000000000001AAA"}`, false},
		{"-q3-", "childProjection", `{` + identity + `,"expectedParentId":"001000000000001AAA","expectedChildId":"001000000000001AAA","childProjection":[{"parentId":"001000000000001AAA"}],"rows":[{"Id":"001000000000001AAA"}]}`, false},
		{"-q5-", "typeofAllowed", `{` + identity + `,"rows":[{"Id":"001000000000001AAA","WhatId":"001000000000001AAA","What":{"Id":"001000000000001AAA"}}]}`, false},
		{"q4", "denied", `{` + identity + `,"exception":{"type":"System.QueryException"}}`, true},
	}
	for _, tc := range cases {
		o, e := c12JSONObject([]byte(tc.raw))
		if e != nil {
			t.Fatal(e)
		}
		if e = c11q345Project(o, c11q345Case{Route: tc.route, Method: tc.method, Omit: tc.omit}); e == nil {
			t.Fatal("invalid observation accepted", tc.method)
		}
	}
}
