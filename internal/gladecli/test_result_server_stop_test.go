package gladecli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

func TestGracefulStopFinalResultServerOnlyProjectRoutes(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
			t.Run(api+"/"+cause.Error(), func(t *testing.T) {
				root := writeDiscoveryStopProject(t, api)
				startCompletedStopPeer(t, root, cause, nil)
				var output bytes.Buffer
				result, err := runTest(context.Background(), []string{"--project", root, "--connect", "--json", "--no-progress"}, &output, io.Discard)
				if !errors.Is(err, cause) || strings.Count(err.Error(), cause.Error()) != 1 || result.Summary().Passed != 1 {
					t.Fatalf("server stop lost its sole error: summary=%+v error=%v", result.Summary(), err)
				}
				assertStoppedReport(t, output.Bytes(), "json", 1)
			})
		}
	}
}
