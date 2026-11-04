package apextest

import "testing"

func TestCaptureClockRelationshipNormalization(t *testing.T) {
	good := map[string]any{"before": "2026-10-31T22:20:00.000Z", "after": "2026-10-31T22:20:00.010Z", "future": "2026-11-01T06:55:00.000Z", "rows": []any{map[string]any{"StartDateTime": "2026-11-01T06:55:00.000Z"}}}
	if _, err := normalizeClockCapture(good); err != nil {
		t.Fatalf("valid clock relation: %v", err)
	}
	good["after"] = "2026-11-01T07:00:00.000Z"
	if _, err := normalizeClockCapture(good); err == nil {
		t.Fatal("accepted an expired future row")
	}
}
