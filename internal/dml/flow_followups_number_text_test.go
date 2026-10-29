package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestFlowFollowupsNumberMergeText(t *testing.T) {
	// Native API 62/67: flow_followups/org.tsv CE_negative_fraction,
	// CE_negative_whole, CE_negative_small, CE_zero and CE_negative_zero.
	for _, tc := range []struct{ input, want string }{
		{"-1234.50", "-1,234.5"},
		{"-1234.00", "-1,234"},
		{"-0.50", "-0.5"},
		{"1234.00", "1,234"},
		{"0.00", "0"},
		{"-0.00", "0"},
		{"-0", "0"},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, supported := flowMergeFieldText(storage.DecimalValue(tc.input))
			if !supported || got != tc.want {
				t.Fatalf("merge(%q) = %q, %t; want %q", tc.input, got, supported, tc.want)
			}
		})
	}
}
