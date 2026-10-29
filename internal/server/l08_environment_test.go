package server

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Exercise the real integration entry point without Node. An ordinary Go run
// must report its missing browser coverage; an explicit browser/capture run
// must fail on missing prerequisites instead of reporting native mismatches.
func TestL08BrowserEnvironmentGuard(t *testing.T) {
	for _, tt := range []struct {
		name, browser, capture, want string
		wantFailure                  bool
	}{
		{name: "ordinary Go run", want: "--- SKIP: TestL08SalesforceConformance"},
		{name: "browser requested", browser: "1", want: "L08 browser prerequisites: node:", wantFailure: true},
		{name: "capture requested", capture: "1", want: "L08 browser prerequisites: node:", wantFailure: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestL08SalesforceConformance$", "-test.v")
			cmd.Env = append(os.Environ(), "PATH="+t.TempDir(), "GLADE_LWC_BROWSER="+tt.browser, "GLADE_L08_CAPTURE="+tt.capture, "GLADE_L08_REPORT=")
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("integration prerequisite check timed out: %s", output)
			}
			if (err != nil) != tt.wantFailure || !strings.Contains(string(output), tt.want) {
				t.Fatalf("exit error=%v; want failure=%t and %q in output:\n%s", err, tt.wantFailure, tt.want, output)
			}
			if !tt.wantFailure && !strings.Contains(string(output), "set GLADE_LWC_BROWSER=1") {
				t.Fatalf("skip must explain how to run browser coverage:\n%s", output)
			}
		})
	}
}
