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

func TestDoctorPlatformContextRoutes(t *testing.T) {
	data, err := os.ReadFile("../project/testdata/org_setup_platform_context/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Route, Source, Feature, File string
		E2E                          bool
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		if !tc.E2E {
			continue
		}
		for _, api := range []string{"62.0", "67.0"} {
			for _, route := range []string{"doctorJSON", "doctorText", "failedTestHint", "failedTestCommand"} {
				t.Run(route+"/"+api+"/"+tc.Route, func(t *testing.T) {
					root := t.TempDir()
					body := "System.assert(false, 'synthetic failure');"
					if strings.HasSuffix(tc.File, ".cls") {
						body = "SetupUse.observe(); " + body
					}
					writeDoctorFixture(t, root, map[string]string{
						"sfdx-project.json":                              `{"packageDirectories":[{"path":"force-app"}],"sourceApiVersion":"` + api + `"}`,
						"force-app/main/default/classes/" + tc.File:      tc.Source,
						"force-app/main/default/classes/FailureTest.cls": "@IsTest private class FailureTest { @IsTest static void fails() { " + body + " } }",
					})
					var stdout, stderr bytes.Buffer
					switch route {
					case "failedTestCommand":
						code := Run(context.Background(), []string{"test", "--project", root, "--class", "FailureTest", "--no-cache", "--no-serve", "--no-progress", "--json"}, &stdout, &stderr)
						var result struct {
							Summary struct{ Failed, CompileErrors int } `json:"summary"`
						}
						if err := json.Unmarshal(stdout.Bytes(), &result); err != nil || code == 0 || result.Summary.Failed != 1 || result.Summary.CompileErrors != 0 {
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
						if tc.Feature == "" {
							if len(info.OrgSetup) != 0 {
								t.Fatalf("name-only source generated advice: %+v", info.OrgSetup)
							}
						} else if len(info.OrgSetup) != 1 || info.OrgSetup[0].Feature != tc.Feature {
							t.Fatalf("want %s advice, got %+v", tc.Feature, info.OrgSetup)
						}
						return
					}
					out := stdout.String()
					if strings.HasPrefix(route, "failedTest") {
						out = stderr.String()
					}
					if tc.Feature == "" {
						if strings.Contains(out, "Org setup the project uses") || strings.Contains(out, "Hint: some failures") || strings.Contains(out, "features: [") {
							t.Fatalf("name-only source generated advice:\n%s", out)
						}
					} else if !strings.Contains(out, "features: ["+tc.Feature+"]") {
						t.Fatalf("missing %s advice:\n%s", tc.Feature, out)
					}
				})
			}
		}
	}
}
