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

func TestRunDoctorReportsFeatureUsage(t *testing.T) {
	root := t.TempDir()
	writeDoctorFixture(t, root, map[string]string{
		"glade.yml":         "org:\n  features: [Chatter]\n",
		"sfdx-project.json": `{"packageDirectories":[{"path":"force-app"}]}`,
	})
	paths, err := filepath.Glob("../project/testdata/conformance/org_setup/project/force-app/main/default/classes/*.cls")
	if err != nil || len(paths) != 9 {
		t.Fatalf("native sources: %v, %v", paths, err)
	}
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		writeDoctorFixture(t, root, map[string]string{"force-app/classes/" + filepath.Base(path): string(source)})
	}
	var stdout, stderr bytes.Buffer
	Run(context.Background(), []string{"doctor", "--project", root, "--json"}, &stdout, &stderr)
	var info cliui.DoctorInfo
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		t.Fatalf("doctor JSON: %v\n%s", err, stdout.String())
	}
	if len(info.OrgSetup) != 5 {
		t.Errorf("expected three features, Territory2, and record type; got %+v", info.OrgSetup)
	}
	stdout.Reset()
	Run(context.Background(), []string{"doctor", "--project", root}, &stdout, &stderr)
	for _, want := range []string{
		"PersonAccounts", "Communities", "MultiCurrency", "Territory2", "not modelled",
		"record types Account.Synthetic_Client", "objects/<object>/recordTypes/",
		"features: [Chatter, PersonAccounts, Communities, MultiCurrency]",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("doctor missing %q:\n%s", want, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), "features: [Territory2]") {
		t.Fatal("doctor suggested an unimplemented feature")
	}
}

func TestWriteTestOrgSetupHintWithoutConfigMapping(t *testing.T) {
	root := t.TempDir()
	writeDoctorFixture(t, root, map[string]string{
		"sfdx-project.json":              `{"packageDirectories":[{"path":"force-app"}]}`,
		"force-app/classes/SetupUse.cls": "class SetupUse { Object x = Database.query('SELECT Id FROM Territory2'); }",
	})
	result := testreport.Run{Suites: []testreport.Suite{{Cases: []testreport.Case{{Status: testreport.StatusFail}}}}}
	var buf bytes.Buffer
	writeTestOrgSetupHint(&buf, []string{"--project", root}, result)
	if out := buf.String(); !strings.Contains(out, "Territory2") || !strings.Contains(out, "not modelled") || strings.Contains(out, "Add:") || strings.Contains(out, "features:") {
		t.Fatalf("unsupported feature hint: %q", out)
	}
}
