package gladecli

import (
	"bytes"
	"context"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

// Only the one-shot local route skips the ambient org fingerprint. The
// daemon route can transition later and keeps computing it.
func TestRunTestAmbientFingerprintByRoute(t *testing.T) {
	routes := []struct {
		name  string
		args  []string
		calls int64
	}{
		{"one-shot local", []string{"--no-serve", "--no-cache"}, 0},
		{"daemon", []string{"--daemon"}, 1},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			apextest.InvalidateRuntimeCaches()
			t.Cleanup(apextest.InvalidateRuntimeCaches)
			fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{classes: map[string]string{
				"OneShotFingerprint": `@IsTest private class OneShotFingerprint { @IsTest static void ok(){ System.assertEquals(1, 1); } }`,
			}})
			args := append([]string{"--project", fixture.projectRoot, "--parallelism", "1", "--no-progress"}, route.args...)
			before := apextest.RuntimePatchAmbientFingerprintCalls()
			var output, progress bytes.Buffer
			run, err := runTest(context.Background(), args, &output, &progress)
			if err != nil || run.Summary().Total != 1 || run.Summary().Passed != 1 {
				t.Fatalf("run: %v %#v\n%s", err, run.Summary(), &output)
			}
			if got := apextest.RuntimePatchAmbientFingerprintCalls() - before; got != route.calls {
				t.Fatalf("ambient fingerprints = %d, want %d", got, route.calls)
			}
		})
	}
}
