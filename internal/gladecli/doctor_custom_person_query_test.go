package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestDoctorCustomPersonQueryDiagnosticRoutes(t *testing.T) {
	const dir = "../project/testdata/conformance/org_setup_custom_person_query"
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	var cases []struct {
		Route, Class, Want string
		Feature            bool
	}
	if err := json.Unmarshal([]byte(read("cases.json")), &cases); err != nil {
		t.Fatal(err)
	}
	field := read("project/force-app/main/default/objects/Contact/fields/Loyalty__c.field-meta.xml")
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range cases {
			for _, route := range []string{"doctorJSON", "doctorText", "failedTestHint", "failedTestCommand"} {
				if route == "failedTestCommand" && !tc.Feature {
					continue
				}
				t.Run(route+"/"+api+"/"+tc.Route, func(t *testing.T) {
					root := t.TempDir()
					writeDoctorFixture(t, root, map[string]string{
						"sfdx-project.json": `{"packageDirectories":[{"path":"force-app"}],"sourceApiVersion":"` + api + `"}`,
						"force-app/main/default/classes/" + tc.Class + ".cls":                     read("project/force-app/main/default/classes/" + tc.Class + ".cls"),
						"force-app/main/default/objects/Contact/fields/Loyalty__c.field-meta.xml": field,
					})
					var stdout, stderr bytes.Buffer
					switch route {
					case "failedTestHint":
						writeTestOrgSetupHint(&stderr, []string{"--project", root}, testreport.Run{Suites: []testreport.Suite{{Cases: []testreport.Case{{Status: testreport.StatusFail}}}}})
					case "failedTestCommand":
						writeDoctorFixture(t, root, map[string]string{"force-app/main/default/classes/PersonQueryFailureTest.cls": "@IsTest private class PersonQueryFailureTest { @IsTest static void needsSetup() { System.assertEquals('" + tc.Want + "', " + tc.Class + ".observe()); System.assert(false, 'synthetic failure'); } }"})
						code := Run(context.Background(), []string{"test", "--project", root, "--class", "PersonQueryFailureTest", "--no-cache", "--no-serve", "--no-progress", "--json"}, &stdout, &stderr)
						var result struct {
							Summary struct{ Failed, CompileErrors int } `json:"summary"`
						}
						if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || code == 0 || result.Summary.Failed+result.Summary.CompileErrors == 0 {
							t.Fatalf("expected failed project test: exit=%d result=%+v err=%v\n%s\n%s", code, result, err, &stdout, &stderr)
						}
					default:
						args := []string{"doctor", "--project", root}
						if route == "doctorJSON" {
							args = append(args, "--json")
						}
						Run(context.Background(), args, &stdout, &stderr)
					}
					if route == "doctorJSON" {
						var info cliui.DoctorInfo
						if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
							t.Fatal(err)
						}
						found := false
						for _, s := range info.OrgSetup {
							if s.Feature == "PersonAccounts" {
								found = true
								if len(s.Files) != 1 || !strings.HasSuffix(s.Files[0], tc.Class+".cls") {
									t.Errorf("wrong evidence source: %+v", s)
								}
							}
						}
						if found != tc.Feature {
							t.Fatalf("doctor JSON custom query advice=%v, want %v: %+v", found, tc.Feature, info.OrgSetup)
						}
						return
					}
					out := stdout.String()
					if strings.HasPrefix(route, "failedTest") {
						out = stderr.String()
					}
					if strings.Contains(out, "PersonAccounts") != tc.Feature {
						t.Fatalf("%s custom query advice mismatch, want=%v:\n%s", route, tc.Feature, out)
					}
				})
			}
		}
	}
}
