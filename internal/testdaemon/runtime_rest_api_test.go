package testdaemon

import (
	"bytes"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestRuntimeRESTAPIRequestAndReceipt(t *testing.T) {
	for _, version := range []string{"", "65.0", "67.0"} {
		request := NewRunRequestV1(apextest.Options{RuntimeRESTAPIVersion: version, Parallelism: 1}, "", 1, 0, false)
		var wire bytes.Buffer
		if err := EncodeRequestV1(&wire, RequestV1{Version: ProtocolVersionV1, Op: OpRun, Run: &request}); err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeRequestV1(&wire)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateRunRequestV1(*decoded.Run); err != nil {
			t.Fatal(err)
		}
		if got := apexOptionsFromRunRequestV1(*decoded.Run).RuntimeRESTAPIVersion; got != version {
			t.Fatalf("request context lost: %q", got)
		}
		effective := version
		if effective == "" {
			effective = "65.0"
		}
		response := ResponseV1{Version: ProtocolVersionV1, Op: OpRunResult, OK: true, Run: &testreport.Run{RuntimeRESTAPIVersion: effective}}
		if err := EncodeResponseV1(&wire, response); err != nil {
			t.Fatal(err)
		}
		result, err := DecodeResponseV1(&wire)
		if err != nil {
			t.Fatal(err)
		}
		if result.Run.RuntimeRESTAPIVersion != effective {
			t.Fatalf("receipt context lost: %#v", result.Run)
		}
	}
	for _, version := range []string{"999.0", "67", "67.1", "NaN"} {
		request := NewRunRequestV1(apextest.Options{RuntimeRESTAPIVersion: version, Parallelism: 1}, "", 0, 0, false)
		if err := validateRunRequestV1(request); err == nil {
			t.Fatalf("invalid context accepted: %q", version)
		}
	}
}
