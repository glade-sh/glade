package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestPersonAccountFormulaNativeAnonymousExec(t *testing.T) {
	personAccountFormulaNativeAnonymousExec(t, false)
}

func TestPersonAccountFormulaNativeAnonymousRead(t *testing.T) {
	personAccountFormulaNativeAnonymousExec(t, true)
}

func personAccountFormulaNativeAnonymousExec(t *testing.T, readOnly bool) {
	personAccountFormulaNativeAnonymousExecRoute(t, readOnly, false)
}

func personAccountFormulaNativeAnonymousExecRoute(t *testing.T, readOnly bool, typed bool) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	const fixture = "../apextest/testdata/conformance/person_account_formulas/"
	raw, err := os.ReadFile(fixture + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases struct{ Files map[string]string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(fixture + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			want := map[string]any{}
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				row := strings.SplitN(line, "\t", 4)
				if row[0] != api || row[1] != "anonymous" {
					continue
				}
				var value any
				if err := json.Unmarshal([]byte(row[3]), &value); err != nil {
					t.Fatal(err)
				}
				want[row[2]] = value
			}
			if len(want) != 13 {
				t.Fatalf("native rows=%d", len(want))
			}
			if readOnly {
				for _, id := range []string{"business", "clear", "related_update", "contact_update", "account_update"} {
					delete(want, id)
				}
			}
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeTestFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [PersonAccounts]\n")
			for name, body := range cases.Files {
				if typed {
					body = regexp.MustCompile(`Flow\.Interview\.createInterview\('([^']+)',`).ReplaceAllString(body, "new Flow.Interview.$1(")
				}
				if readOnly && strings.HasSuffix(name, "/PersonFormulaProbe.cls") {
					prefix, _, found := strings.Cut(body, "  a.Note__pc='account';")
					if !found {
						t.Fatal("missing captured read/update boundary")
					}
					body = prefix + "  return JSON.serialize(rows);\n }\n}"
				}
				writeTestFile(t, filepath.Join(root, name), strings.ReplaceAll(body, "62.0", api))
			}
			var stdout, stderr bytes.Buffer
			if code := Run(context.Background(), []string{"exec", "--project", root, "--json", "System.debug(PersonFormulaProbe.observe());"}, &stdout, &stderr); code != 0 {
				t.Fatalf("native probe succeeded; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
			}
			var result struct{ Debug []string }
			decodeCLIEnvelopeData(t, stdout.Bytes(), "exec", &result)
			if len(result.Debug) != 1 {
				t.Fatalf("missing observation: %v", result.Debug)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(result.Debug[0]), &got); err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Fatalf("observations=%d, want %d", len(got), len(want))
			}
			for id, value := range want {
				t.Run(id, func(t *testing.T) {
					if !reflect.DeepEqual(got[id], value) {
						t.Fatalf("native=%v; Glade=%v", value, got[id])
					}
				})
			}
		})
	}
}

func TestPersonAccountFormulaNativeTypedAnonymousExec(t *testing.T) {
	personAccountFormulaNativeAnonymousExecRoute(t, false, true)
}
