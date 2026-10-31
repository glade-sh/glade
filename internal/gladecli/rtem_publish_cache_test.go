package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

// The source's named Apex compilation is captured in
// internal/sema/testdata/rtem_publish_route_twins/org.tsv:5,19.
func TestRTEMPublishCheckColdAndWarm(t *testing.T) {
	t.Setenv("GLADE_DISABLE_DISK_CACHE", "")
	restore := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restore)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			checkSemanticResults.Reset()
			t.Cleanup(checkSemanticResults.Reset)
			fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{sourceAPIVersion: api, classes: map[string]string{
				"Probe": `public class Probe { public static void run() { EventBus.publish(new FileEvent()); EventBus.publish(new FileEvent(), null); } }`,
			}})
			root := fixture.projectRoot
			outputs := make([]string, 0, 2)
			for i, expected := range []string{"build", "disk"} {
				if i == 1 {
					checkSemanticResults.Reset()
				}
				perf := filepath.Join(t.TempDir(), "perf.json")
				var stdout, stderr bytes.Buffer
				code := Run(context.Background(), []string{"check", "--project", root, "--json", "--no-progress", "--perf-json", perf}, &stdout, &stderr)
				if code != 0 {
					t.Fatalf("%s check exit=%d stdout=%s stderr=%s", expected, code, stdout.String(), stderr.String())
				}
				cache := requireJSONObject(t, readPerfJSONObject(t, perf), "semanticCache")
				if cache["source"] != expected {
					t.Fatalf("cache source=%v want %s", cache["source"], expected)
				}
				outputs = append(outputs, stdout.String())
			}
			if outputs[0] != outputs[1] {
				t.Fatalf("cold and warm diagnostics differ: %s / %s", outputs[0], outputs[1])
			}
		})
	}
}
