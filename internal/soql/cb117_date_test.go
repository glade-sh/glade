package soql

import (
	"testing"
	"time"
)

func TestCB117LastNWeeksIncludesCurrentWeek(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	start, end, ok := dateLiteral("LAST_N_WEEKS:1", now)
	if !ok {
		t.Fatal("LAST_N_WEEKS:1 was not recognized")
	}
	if got := start.String; got != "2026-04-26" {
		t.Fatalf("LAST_N_WEEKS:1 start = %q, want 2026-04-26", got)
	}
	if got := end.String; got != "2026-05-03" {
		t.Fatalf("LAST_N_WEEKS:1 end = %q, want 2026-05-03", got)
	}
}
