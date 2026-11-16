package testdaemon

import (
	"bufio"
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
)

// A coverage run carries its options to the daemon and its coverage back.
// Without coverage the request frame is unchanged, so an older daemon still
// serves plain runs and rejects a coverage run rather than dropping coverage.
func TestProtocolV1CarriesCodeCoverage(t *testing.T) {
	options := apextest.Options{Parallelism: 1, CodeCoverage: true, CoverageGate: testreport.CoverageGateRunLocalTests}
	request := NewRunRequestV1(options, "", 0, 0, false)
	if !request.CodeCoverage || request.CoverageGate != testreport.CoverageGateRunLocalTests {
		t.Fatalf("request = %+v", request)
	}
	if mapped := apexOptionsFromRunRequestV1(request); !mapped.CodeCoverage || mapped.CoverageGate != testreport.CoverageGateRunLocalTests {
		t.Fatalf("options from request = %+v", mapped)
	}
	if err := validateRunRequestV1(request); err != nil {
		t.Fatal(err)
	}
	bad := request
	bad.CoverageGate = "RunAllTests"
	if err := validateRunRequestV1(bad); err == nil || !strings.Contains(err.Error(), "coverage gate") {
		t.Fatalf("unknown gate error = %v", err)
	}
	var encoded bytes.Buffer
	if err := EncodeRequestV1(&encoded, RequestV1{Version: ProtocolVersionV1, Op: OpRun, Run: &request}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(encoded.String(), `"codeCoverage":true`) || !strings.Contains(encoded.String(), `"coverageGate":"RunLocalTests"`) {
		t.Fatalf("encoded request = %s", encoded.String())
	}
	encoded.Reset()
	if err := EncodeRequestV1(&encoded, RequestV1{Version: ProtocolVersionV1, Op: OpRun, Run: &RunRequestV1{}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded.String(), "overage") {
		t.Fatalf("a run without coverage sends coverage fields: %s", encoded.String())
	}

	want := ResponseV1{
		Version: ProtocolVersionV1,
		Op:      OpRunResult,
		OK:      true,
		Ready:   true,
		Run: &testreport.Run{
			Name: "glade test",
			Suites: []testreport.Suite{{Name: "ShapeTest", Cases: []testreport.Case{{
				ClassName: "ShapeTest", MethodName: "left", Status: testreport.StatusPass,
				Coverage: []testreport.TestCoverage{{Name: "Shape", Type: "class", NumLinesCovered: 1, NumLinesUncovered: 2, CoveredLines: []int{2}}},
			}}}},
			Coverage: &testreport.Coverage{
				Summary:    testreport.CoverageSummary{TotalLines: 3, CoveredLines: 1, TestRunCoverage: 33, OrgWideTotalLines: 3, OrgWideCovered: 1, OrgWideCoverage: 33},
				Components: []testreport.ComponentCoverage{{Name: "Shape", Type: "class", TotalLines: 3, TotalCovered: 1, CoveredPercent: 33, CoveredLines: []int{2}, UncoveredLines: []int{3, 4}}},
				Gate:       &testreport.CoverageGate{TestLevel: testreport.CoverageGateRunLocalTests, Percent: 33, Warnings: []string{"Average test coverage across all Apex Classes and Triggers is 33%, at least 75% test coverage is required."}},
			},
		},
	}
	encoded.Reset()
	if err := EncodeResponseV1(&encoded, want); err != nil {
		t.Fatal(err)
	}
	got, err := DecodeResponseV1(bufio.NewReader(&encoded))
	if err != nil {
		t.Fatalf("DecodeResponseV1: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response round trip mismatch:\n got: %#v\nwant: %#v", got.Run.Coverage, want.Run.Coverage)
	}
	if summary := got.Run.Summary(); !summary.CoverageGateFailed || summary.OK() {
		t.Fatalf("decoded summary = %+v", summary)
	}
}
