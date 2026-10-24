package gladecli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestGracefulStopFinalResultErrorOwnershipRoutes(t *testing.T) {
	for _, route := range []string{"connect", "auto-connect"} {
		for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
			for _, resultKind := range []string{"partial", "completed-server-stop"} {
				t.Run(route+"/"+cause.Error()+"/"+resultKind, func(t *testing.T) {
					root := t.TempDir()
					ctx, cancel := discoveryStopContext(t, cause)
					result := completedStopReport()
					if resultKind == "partial" {
						result.Suites[0].Cases = append(result.Suites[0].Cases, testreport.Case{
							ClassName: "DiscoveryTest", MethodName: "interrupted", Status: testreport.StatusUnsupported, Reason: testreport.ReasonCancelled,
						})
					}
					stub := startProtocolV1Stub(t, root, func(request testdaemon.RequestV1) testdaemon.ResponseV1 {
						response := testdaemon.ResponseV1{Version: testdaemon.ProtocolVersionV1, Op: testdaemon.OpPong, OK: true, Ready: true}
						if request.Op == testdaemon.OpRun {
							response.Op, response.Run = testdaemon.OpRunResult, &result
							if resultKind == "completed-server-stop" {
								response.Op, response.OK, response.Error = testdaemon.OpError, false, cause.Error()
							}
							cancel()
						}
						return response
					})
					defer stub.Close()
					args := []string{"--project", root, "--json", "--no-progress"}
					if route == "connect" {
						args = append(args, "--connect")
					}
					var output bytes.Buffer
					got, err := runTest(ctx, args, &output, io.Discard)
					if ctx.Err() != cause || got.Summary() != result.Summary() {
						t.Fatalf("stop boundary not reached: context=%v summary=%+v error=%v", ctx.Err(), got.Summary(), err)
					}
					assertStoppedReport(t, output.Bytes(), "json", 1)
					if resultKind == "partial" {
						if err != nil {
							t.Errorf("failed summary gained a bare command error: %v", err)
						}
					} else if !errors.Is(err, cause) || strings.Count(err.Error(), cause.Error()) != 1 {
						t.Errorf("server stop must own exactly one command error: %v", err)
					}
				})
			}
		}
	}
}
