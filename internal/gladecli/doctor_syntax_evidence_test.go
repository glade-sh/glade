package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestDoctorSyntaxEvidenceRoutes(t *testing.T) {
	data, err := os.ReadFile("../project/testdata/conformance/org_setup_syntax/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Route, Class, Want string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		source, err := os.ReadFile("../project/testdata/conformance/org_setup_syntax/project/force-app/main/default/classes/" + tc.Class + ".cls")
		if err != nil {
			t.Fatal(err)
		}
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range []string{"doctorJSON", "doctorText", "failedTestHint", "failedTestCommand", "execAnonymous", "testNamed"} {
				t.Run(route+"/"+api+"/"+tc.Route, func(t *testing.T) {
					root := t.TempDir()
					body := "System.assertEquals('" + tc.Want + "', " + tc.Class + ".observe());"
					if route != "testNamed" {
						body += " System.assert(false, 'synthetic failure');"
					}
					writeDoctorFixture(t, root, map[string]string{
						"sfdx-project.json": `{"packageDirectories":[{"path":"force-app"}],"sourceApiVersion":"` + api + `"}`,
						"force-app/main/default/classes/" + tc.Class + ".cls": string(source),
						"force-app/main/default/classes/FailureTest.cls":      "@IsTest private class FailureTest { @IsTest static void fails() { " + body + " } }",
					})
					var stdout, stderr bytes.Buffer
					switch route {
					case "execAnonymous":
						code := Run(context.Background(), []string{"exec", "--project", root, "--json", "System.assertEquals('" + tc.Want + "', " + tc.Class + ".observe());"}, &stdout, &stderr)
						if code != 0 {
							t.Fatalf("native-matched anonymous route: exit=%d\n%s\n%s", code, &stdout, &stderr)
						}
					case "testNamed", "failedTestCommand":
						code := Run(context.Background(), []string{"test", "--project", root, "--class", "FailureTest", "--no-cache", "--no-serve", "--no-progress", "--json"}, &stdout, &stderr)
						var result struct {
							Summary struct{ Passed, Failed, CompileErrors int } `json:"summary"`
						}
						wantPassed, wantFailed := 0, 1
						if route == "testNamed" {
							wantPassed, wantFailed = 1, 0
						}
						if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || (code == 0) != (wantFailed == 0) || result.Summary.Passed != wantPassed || result.Summary.Failed != wantFailed || result.Summary.CompileErrors != 0 {
							t.Fatalf("expected executed assertion failure: exit=%d result=%+v err=%v\n%s\n%s", code, result, err, stdout.String(), stderr.String())
						}
					case "failedTestHint":
						result := testreport.Run{Suites: []testreport.Suite{{Cases: []testreport.Case{{Status: testreport.StatusFail}}}}}
						writeTestOrgSetupHint(&stderr, []string{"--project", root}, result)
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
							t.Fatalf("doctor JSON: %v\n%s", err, stdout.String())
						}
						if len(info.OrgSetup) != 0 {
							t.Fatalf("inert syntax generated advice: %+v", info.OrgSetup)
						}
						return
					}
					out := stdout.String()
					if strings.HasPrefix(route, "failedTest") {
						out = stderr.String()
					}
					if strings.Contains(out, "Org setup the project uses") || strings.Contains(out, "Hint: some failures") || strings.Contains(out, "features: [") {
						t.Fatalf("inert syntax generated advice:\n%s", out)
					}
				})
			}
		}
	}
}
