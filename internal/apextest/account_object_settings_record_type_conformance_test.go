package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

const accountObjectSettingsFixture = "testdata/native-capture/account_object_settings_record_type"

// accountObjectSettingsKnownGaps lists captured rows Glade does not match yet,
// with the reason. Each must still differ.
var accountObjectSettingsKnownGaps = map[string]string{}

// TestAccountObjectSettingsRecordTypeNativeConformance replays the
// account_object_settings_record_type capture (fresh Developer scratch org
// whose definition sets objectSettings.account.defaultRecordType, API 62.0
// and 67.0, anonymous and isTest): static and dynamic Account.RecordTypeId,
// its describe, record type infos and inserts.
func TestAccountObjectSettingsRecordTypeNativeConformance(t *testing.T) {
	native := accountCaptureNativeRows(t, accountObjectSettingsFixture)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := loadTestIndex(t, accountCaptureProject(t, accountObjectSettingsFixture, api))
			org := capturedFixtureOrg(t, index, api)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true})
			got := map[string]string{}
			for _, probe := range []string{"rt", "s1", "s2", "s3", "s4", "s5", "s6"} {
				raw, err := os.ReadFile(filepath.Join(accountObjectSettingsFixture, "anon", probe+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				source := string(raw)
				compileID := strings.ToUpper(probe) + " compile"
				if probe != "rt" {
					analysis := sema.AnalyzeAnonymous(index, source, api)
					if analysis.HasErrors() {
						got[compileID] = "false " + fmt.Sprint(analysis.Diagnostics)
						continue
					}
					got[compileID] = "true"
				}
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatalf("%s: %v", probe, err)
				}
				result, err := runner.execute(program)
				if err != nil {
					t.Errorf("%s: run error %v", probe, err)
				}
				for id, observation := range accountCaptureDebugRows(result.Debug) {
					got[id] = observation
				}
			}
			compare := func(route, id, want, observed string) {
				if strings.HasSuffix(id, " compile") && strings.HasPrefix(want, "false ") {
					// Native compile text follows "with the error: ".
					_, problem, _ := strings.Cut(want, "with the error: ")
					if strings.HasPrefix(observed, "false ") && strings.Contains(observed, problem) {
						observed = want
					}
				}
				reason, gap := accountObjectSettingsKnownGaps[api+" "+id]
				switch {
				case gap && observed == want:
					t.Errorf("%s %s: known gap (%s) now matches; remove it", route, id, reason)
				case !gap && observed != want:
					t.Errorf("%s %s: native <%s> glade <%s>", route, id, want, observed)
				}
			}
			for _, row := range native[api]["anonymous"] {
				compare("anonymous", row[0], row[1], got[row[0]])
			}
			run := Run(index, Options{SelectedClasses: []string{"LpRtProbeTest"}, NoDiskCache: true, Parallelism: 1})
			gotTest := accountCaptureTestRows(t, run)
			for _, row := range native[api]["istest"] {
				compare("isTest", row[0], row[1], gotTest[row[0]])
			}
		})
	}
}
