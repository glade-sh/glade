package apextest_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/sema"
)

func TestC11DeniedChildLookup(t *testing.T) {
	root := filepath.Join("testdata", "conformance", "c11sharing_child_denied")
	index := c12LoadIndex(t, filepath.Join(root, "project"))
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatal("source diagnostics")
	}
	expected := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(c12Read(t, filepath.Join(root, "org.tsv")))), "\n")[1:] {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			t.Fatal("native row")
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(parts[2]), &row); err != nil {
			t.Fatal(err)
		}
		expected[parts[0]] = row
	}
	for _, tc := range []struct{ api, class, marker string }{{"62.0", "C11Q3R62C1", "c11.Q3.restricted.api62|Q3.positiveChildProjection"}, {"67.0", "C11Q3R67C1", "c11.Q3.restricted.api67|Q3.positiveChildProjection"}} {
		t.Run(tc.api, func(t *testing.T) {
			c12VerifyContext(t, index, tc.class, "", tc.api)
			report := apextest.Run(index, apextest.Options{SelectedClasses: []string{tc.class}, SelectedMethod: "positiveChildProjection", NoDiskCache: true, Parallelism: 1})
			if len(report.Suites) != 1 || len(report.Suites[0].Cases) != 1 {
				t.Fatal("selection mismatch")
			}
			raw, err := c12CapturePayload(report.Suites[0].Cases[0], tc.class, "positiveChildProjection", tc.marker)
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			actual := map[string]any{"status": got["status"], "stage": got["stage"], "exception": got["exception"]}
			if !reflect.DeepEqual(actual, expected[tc.api]) {
				t.Fatalf("denied lookup differs: %v", actual)
			}
		})
	}
}
