package vm

import (
	"strings"
	"testing"
)

// Quoted literals must not be case-folded merely to check bare keywords.
func TestQuotedLiteralAvoidsKeywordFoldAllocation(t *testing.T) {
	raw := "'" + strings.Repeat("MiXeD", 1024) + "'"
	var result Value
	var err error
	allocations := testing.AllocsPerRun(100, func() { result, err = parseLiteral(raw) })
	if err != nil || result.Text != raw[1:len(raw)-1] {
		t.Fatalf("literal result %q, error %v", result.Text, err)
	}
	if allocations != 0 {
		t.Fatalf("quoted literal incurred %v allocations during keyword matching, want 0", allocations)
	}
}
