package gladecli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNetworkExternalUserAccessNativeProjectRoutes(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	const fixture = "../apextest/testdata/conformance/network_external_user_access"
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	reader := csv.NewReader(strings.NewReader(read("org.tsv")))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]string{}
	for _, row := range rows[1:] {
		key := row[0] + "/" + row[1]
		if want[key] == nil {
			want[key] = map[string]string{}
		}
		if _, duplicate := want[key][row[2]]; duplicate {
			t.Fatalf("duplicate native cell: %s/%s", key, row[2])
		}
		want[key][row[2]] = row[3]
	}
	if len(rows) != 17 || len(want) != 4 {
		t.Fatal("incomplete native API/route matrix")
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"default", "scratch_definition"} {
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			if shape == "scratch_definition" {
				writeTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), read("project/config/project-scratch-def.json"))
			}
			name := "ExternalUserAccessProbe" + strings.TrimSuffix(api, ".0")
			for _, class := range []string{name, name + "Test"} {
				path := "force-app/main/default/classes/" + class + ".cls"
				writeTestFile(t, filepath.Join(root, path), read("project/"+path))
				writeTestFile(t, filepath.Join(root, path+"-meta.xml"), read("project/"+path+"-meta.xml"))
			}
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(api+"/"+route+"/"+shape, func(t *testing.T) {
					observed := ""
					var stdout, stderr bytes.Buffer
					if route == "anonymous" {
						body := "System.debug('OBS|' + " + name + ".observe());"
						code := Run(context.Background(), []string{"exec", "--project", root, "--json", body}, &stdout, &stderr)
						var envelope struct{ Data struct{ Debug []string } }
						if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || code != 0 || len(envelope.Data.Debug) != 1 {
							t.Fatalf("missing anonymous observation: exit=%d stdout=%s stderr=%s error=%v", code, stdout.String(), stderr.String(), err)
						}
						_, observed, _ = strings.Cut(envelope.Data.Debug[0], "OBS|")
					} else {
						code := Run(context.Background(), []string{"test", "--project", root, "--class", name + "Test", "--method", "observe", "--no-cache", "--no-serve", "--no-progress", "--parallelism", "1", "--json"}, &stdout, &stderr)
						run, err := decodeTestRunJSON(stdout.Bytes())
						if err != nil || code != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
							t.Fatalf("expected one test observation: exit=%d stdout=%s stderr=%s error=%v", code, stdout.String(), stderr.String(), err)
						}
						problem := run.Suites[0].Cases[0].Problem
						if problem == nil || !strings.HasSuffix(problem.Type, "AssertException") {
							t.Fatalf("missing assertion transport: %+v", problem)
						}
						_, observed, _ = strings.Cut(problem.Message, "OBS|")
					}
					var got map[string]string
					if err := json.Unmarshal([]byte(observed), &got); err != nil || len(got) != 4 {
						t.Fatalf("invalid observation %q: %v", observed, err)
					}
					for id, expected := range want[api+"/"+route] {
						if got[id] != expected {
							t.Errorf("%s: native=%q; Glade=%q", id, expected, got[id])
						}
					}
				})
			}
		}
	}
}
