package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/cpurisk"
	"github.com/glade-sh/glade/internal/vm"
)

func TestWriteCPURiskTextGoesToStderrOnly(t *testing.T) {
	c := cpurisk.NewCollector()
	c.Record("A", "b", vm.WorkCounts{DMLRows: 10}, nil)
	var errOut bytes.Buffer
	if err := writeCPURisk(c.Report(), true, "", &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "CPU risk (advisory)") || !strings.Contains(errOut.String(), "A.b") {
		t.Fatalf("report = %q", errOut.String())
	}
	errOut.Reset()
	if err := writeCPURisk(c.Report(), false, "", &errOut); err != nil || errOut.Len() != 0 {
		t.Fatalf("no flag wrote %q, err %v", errOut.String(), err)
	}
}

func TestWriteCPURiskJSONFile(t *testing.T) {
	c := cpurisk.NewCollector()
	c.Record("A", "b", vm.WorkCounts{DMLRows: 10}, nil)
	path := filepath.Join(t.TempDir(), "nested", "risk.json")
	var errOut bytes.Buffer
	if err := writeCPURisk(c.Report(), false, path, &errOut); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `"syncLimitMs": 10000`) || errOut.Len() != 0 {
		t.Fatalf("json = %q err %v stderr %q", data, err, errOut.String())
	}
}

func TestCPURiskRejectsNonOneShotModes(t *testing.T) {
	for _, mode := range []string{"--connect", "--daemon", "--watch", "--watch-once", "--last-failed"} {
		var out, errOut bytes.Buffer
		_, err := runTest(context.Background(), []string{"--project", t.TempDir(), "--cpu-risk", mode}, &out, &errOut)
		if err == nil || !strings.Contains(err.Error(), "--cpu-risk cannot be combined with "+mode) {
			t.Errorf("%s: err = %v", mode, err)
		}
	}
}

func TestRiskWorkSinkNilWithoutCollector(t *testing.T) {
	if riskWorkSink(nil) != nil {
		t.Fatal("sink must be nil when the report is not requested")
	}
}
