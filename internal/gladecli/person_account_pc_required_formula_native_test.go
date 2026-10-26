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

var (
	personAccountPCCLIC1 = regexp.MustCompile(`C1 biz=(true|false)(.*?) pa-no-pc=(true|false)(.*?) pa-pc=(true|false)(.*?) nillable=(true|false) createable=(true|false)`)
	personAccountPCCLIC2 = regexp.MustCompile(`C2 acct=([^|"]*)\|([^ "]*) cont=([^|"]*)\|([^ "]*)`)
)

func parsePersonAccountPCCLIObservations(got map[string]any, text string) {
	for _, match := range personAccountPCCLIC1.FindAllStringSubmatch(text, -1) {
		insert := func(success, errs string) any { return []any{success == "true", strings.TrimSpace(errs)} }
		got["biz_insert"] = insert(match[1], match[2])
		got["pa_no_pc_insert"] = insert(match[3], match[4])
		got["pa_pc_insert"] = insert(match[5], match[6])
		got["must_describe"] = map[string]any{"nillable": match[7] == "true", "createable": match[8] == "true"}
	}
	for _, match := range personAccountPCCLIC2.FindAllStringSubmatch(text, -1) {
		got["mail_read"] = []any{match[1], match[3]}
		got["key_read"] = []any{match[2], match[4]}
	}
}

// Project-runner twin of TestPersonAccountPCRequiredFormulaNativeConformance:
// the native probe runs through `glade exec --project` and `glade test`.
func TestPersonAccountPCRequiredFormulaNativeProjectRunner(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/person_account_pc_required_formula/"
	const suffix = "4242"
	raw, err := os.ReadFile(fixture + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Files     map[string]string
		Anonymous string
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(fixture + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			t.Setenv("GLADE_HOME", t.TempDir())
			t.Setenv("GLADE_ROOT", "")
			root := t.TempDir()
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeTestFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [PersonAccounts]\n")
			for name, body := range cases.Files {
				body = strings.ReplaceAll(body, "Crypto.getRandomLong()", suffix+"L")
				writeTestFile(t, filepath.Join(root, name), strings.ReplaceAll(body, "62.0", api))
			}
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					want := map[string]any{}
					for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
						row := strings.SplitN(line, "\t", 4)
						if row[0] != api || row[1] != route {
							continue
						}
						var value any
						if err := json.Unmarshal([]byte(strings.ReplaceAll(row[3], "{u}", suffix)), &value); err != nil {
							t.Fatal(err)
						}
						want[row[2]] = value
					}
					if len(want) != 6 {
						t.Fatalf("native rows=%d", len(want))
					}
					got := map[string]any{}
					var stdout, stderr bytes.Buffer
					if route == "anonymous" {
						if code := Run(context.Background(), []string{"exec", "--project", root, "--json", cases.Anonymous}, &stdout, &stderr); code != 0 {
							t.Fatalf("native anonymous run succeeded; exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
						}
						var result struct{ Debug []string }
						decodeCLIEnvelopeData(t, stdout.Bytes(), "exec", &result)
						parsePersonAccountPCCLIObservations(got, strings.Join(result.Debug, "\n"))
					} else {
						Run(context.Background(), []string{"test", "--project", root, "--class", "PaCapTest", "--json", "--no-progress", "--no-cache", "--no-serve"}, &stdout, &stderr)
						var result struct {
							Data struct {
								Summary struct{ Total, Failed int }
							}
						}
						if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
							t.Fatalf("stdout is not JSON: %v\n%s%s", err, stdout.String(), stderr.String())
						}
						if summary := result.Data.Summary; summary.Total != 2 || summary.Failed != 2 {
							t.Fatalf("both observation tests must run and transport an assertion: %s", stdout.String())
						}
						parsePersonAccountPCCLIObservations(got, stdout.String())
					}
					if len(got) != len(want) {
						t.Fatalf("observations=%v, want %d rows; stdout=%s", got, len(want), stdout.String())
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
		})
	}
}
