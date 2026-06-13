package cliui

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteCheckResultPluralizesSummaryCounts(t *testing.T) {
	var out bytes.Buffer
	if err := WriteCheckResult(&out, CheckResultInfo{
		ProjectRoot: "/tmp/project",
		Types:       1,
		Triggers:    1,
		Objects:     1,
	}); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "1 type · 1 trigger · 1 object") {
		t.Fatalf("summary line = %q", got)
	}
	if strings.Contains(got, "1 types") || strings.Contains(got, "1 triggers") || strings.Contains(got, "1 objects") {
		t.Fatalf("summary line used plural for singular count: %q", got)
	}
}
