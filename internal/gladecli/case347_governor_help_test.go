package gladecli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/cliui"
)

func TestGovernorFlagsVisibleInCommandHelp(t *testing.T) {
	flags := []string{"--limit-profile", "--limit-queries", "--limit-query-rows", "--limit-dml-statements", "--limit-dml-rows", "--limit-heap-size", "--limit-cpu-ms", "--limit-callouts", "--limit-email-invocations", "--limit-async-jobs", "--limit-future-calls", "--limit-queueable-jobs", "--limit-batch-jobs", "--limit-scheduled-jobs", "--limit-sosl-queries", "--limit-query-locator-rows", "--limit-run-as", "--limit-savepoints", "--limit-savepoint-rollbacks", "--limit-publish-immediate-dml"}
	for _, command := range []string{"exec", "test", "server"} {
		t.Run(command, func(t *testing.T) {
			var out bytes.Buffer
			var err error
			if command == "test" {
				err = writeTestHelp(&out)
			} else {
				err = cliui.WriteCommandHelp(&out, []string{command})
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, flag := range flags {
				if !strings.Contains(out.String(), flag) {
					t.Errorf("missing %s", flag)
				}
			}
		})
	}
}
