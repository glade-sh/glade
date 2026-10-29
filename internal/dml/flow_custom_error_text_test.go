package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Formats observed natively at API 62.0 and 67.0 (flow_gaps P01-P03).
func TestFlowMergeFieldTextMatchesNativeRows(t *testing.T) {
	for name, tc := range map[string]struct {
		value storage.Value
		want  string
		ok    bool
	}{
		"date":     {storage.DateValue("2031-03-09"), "March 9, 2031", true},
		"decimal":  {storage.Value{Kind: storage.ValueDecimal, String: "1234.50"}, "1,234.5", true},
		"small":    {storage.Value{Kind: storage.ValueDecimal, String: "5.0"}, "5", true},
		"integer":  {storage.IntegerValue(1234567), "1,234,567", true},
		"negative": {storage.IntegerValue(-1000), "-1,000", true},
		"null":     {storage.NullValue(), "", true},
		"boolean":  {storage.BooleanValue(true), "true", true},
		"string":   {storage.StringValue("ERR"), "ERR", true},
		"datetime": {storage.DateTimeValue("2031-03-09T10:00:00.000Z"), "", false},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := flowMergeFieldText(tc.value)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("flowMergeFieldText = %q, %v; want %q, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}
