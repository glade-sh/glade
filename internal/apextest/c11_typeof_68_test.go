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

func TestC11CustomTypeofAPI68(t *testing.T) {
	root := filepath.Join("testdata", "conformance", "c11sharing_typeof_custom")
	index := c12LoadIndex(t, filepath.Join(root, "project"))
	analysis := sema.Analyze(index)
	if index.HasErrors() || analysis.HasErrors() {
		t.Fatal("source diagnostics")
	}
	rows := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(string(c12Read(t, filepath.Join(root, "org.tsv")))), "\n")[1:] {
		p := strings.SplitN(line, "\t", 4)
		if len(p) != 4 {
			t.Fatal("invalid native row")
		}
		var v map[string]any
		if err := json.Unmarshal([]byte(p[3]), &v); err != nil {
			t.Fatal(err)
		}
		rows[p[0]+"/"+p[1]+"/"+p[2]] = v
	}
	for _, arm := range []struct{ label, class, marker string }{{"restricted", "C11Q5R68C1", "c11.Q5.restricted.api68"}, {"admin", "C11Q5A68C1", "c11.Q5.admin.api68"}} {
		for _, method := range []string{"typeofAllowed", "typeofDenied"} {
			t.Run(arm.label+"/"+method, func(t *testing.T) {
				c12VerifyContext(t, index, arm.class, "", "68.0")
				report := apextest.Run(index, apextest.Options{SelectedClasses: []string{arm.class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
				if len(report.Suites) != 1 || len(report.Suites[0].Cases) != 1 {
					t.Fatal("selection mismatch")
				}
				raw, err := c12CapturePayload(report.Suites[0].Cases[0], arm.class, method, arm.marker+"|Q5."+method)
				if err != nil {
					t.Fatal(err)
				}
				var got map[string]any
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatal(err)
				}
				actual := map[string]any{"status": got["status"], "stage": got["stage"]}
				if exc, ok := got["exception"]; ok {
					actual["exception"] = exc
				}
				if observed, ok := got["rows"].([]any); ok {
					for _, item := range observed {
						row := item.(map[string]any)
						rowID, rowOK := row["Id"].(string)
						targetID, targetOK := row["WhatId"].(string)
						what, whatOK := row["What"].(map[string]any)
						projectedID, projectedOK := what["Id"].(string)
						if !rowOK || !targetOK || !whatOK || !projectedOK || !originalValid18(rowID) || !originalValid18(targetID) || targetID != projectedID || rowID == targetID {
							t.Fatal("invalid TYPEOF identity projection")
						}
						row["Id"] = "<ID:REDACTED>"
						row["WhatId"] = "<ID:REDACTED>"
						what["Id"] = "<ID:REDACTED>"
					}
					actual["rows"] = observed
				}
				if !reflect.DeepEqual(actual, rows["68.0/"+arm.label+"/"+method]) {
					t.Fatal("observation differs from native")
				}
			})
		}
	}
}
