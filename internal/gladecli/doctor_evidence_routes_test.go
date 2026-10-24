package gladecli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestDoctorOrgSetupEvidenceRoutes(t *testing.T) {
	const dir = "../project/testdata/conformance/org_setup_evidence"
	f, err := os.Open(filepath.Join(dir, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		source, err := os.ReadFile(filepath.Join(dir, "project/force-app/main/default/classes", row[1]+".cls"))
		if err != nil {
			t.Fatal(err)
		}
		for _, route := range []string{"doctorJSON", "doctorText", "failedTestHint"} {
			t.Run(route+"/"+row[0], func(t *testing.T) {
				root := t.TempDir()
				writeDoctorFixture(t, root, map[string]string{
					"sfdx-project.json": `{"packageDirectories":[{"path":"force-app"}]}`,
					"force-app/main/default/classes/" + row[1] + ".cls": string(source),
				})
				var stdout, stderr bytes.Buffer
				if route == "failedTestHint" {
					result := testreport.Run{Suites: []testreport.Suite{{Cases: []testreport.Case{{Status: testreport.StatusFail}}}}}
					writeTestOrgSetupHint(&stdout, []string{"--project", root}, result)
				} else {
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
					if row[2] == "none" {
						if len(info.OrgSetup) != 0 {
							t.Fatalf("name-only source generated advice: %+v", info.OrgSetup)
						}
						return
					}
					if len(info.OrgSetup) != 1 || info.OrgSetup[0].Kind != row[2] {
						t.Fatalf("expected %s %s: %+v", row[2], row[3], info.OrgSetup)
					}
					if row[2] == "recordType" && !reflect.DeepEqual(info.OrgSetup[0].Names, []string{row[3]}) {
						t.Fatalf("record-type names: %+v", info.OrgSetup)
					}
					if row[2] == "feature" && info.OrgSetup[0].Feature != row[3] {
						t.Fatalf("feature: %+v", info.OrgSetup)
					}
					return
				}
				out := stdout.String()
				if row[2] == "none" {
					if strings.Contains(out, "Org setup the project uses") || strings.Contains(out, "Hint: some failures") || strings.Contains(out, "features: [Communities]") || strings.Contains(out, "Territory2; Enterprise") {
						t.Fatalf("name-only source generated advice:\n%s", out)
					}
					return
				}
				want := "features: [Communities]"
				if row[2] == "recordType" {
					want = "record types Account.Synthetic_Client"
					if route == "failedTestHint" {
						want = "references record types whose metadata files are absent"
					}
				}
				if !strings.Contains(out, want) || strings.Contains(out, "Account.Synthetic_Author") {
					t.Fatalf("expected %q without relationship-name advice:\n%s", want, out)
				}
			})
		}
	}
}
