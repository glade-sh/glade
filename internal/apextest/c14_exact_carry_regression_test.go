package apextest

import "testing"

// Native outcome is retained and every permitted local difference is pinned.
func TestC14CarryPinsObservedValues(t *testing.T) {
	native := map[string]any{"status": "exception", "exception": map[string]any{"type": "System.QueryException"}, "rows": nil, "unchanged": true}
	local := map[string]any{"status": "ok", "exception": nil, "rows": []any{map[string]any{"total": float64(3)}}, "unchanged": true}
	expected := map[string]any{"status": "ok", "exception": nil, "rows": []any{map[string]any{"total": float64(3)}}}
	fields := []string{"status", "exception", "rows"}
	if !c14ExactCarryEqual(local, native, expected, fields) {
		t.Fatal("exact observed carry rejected")
	}
	for _, tc := range []struct {
		name, field string
		value       any
	}{
		{"wrong status", "status", "other"}, {"wrong exception", "exception", map[string]any{"type": "Other"}}, {"wrong rows", "rows", []any{}}, {"other field", "unchanged", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := map[string]any{}
			for k, v := range local {
				changed[k] = v
			}
			changed[tc.field] = tc.value
			if c14ExactCarryEqual(changed, native, expected, fields) {
				t.Fatal("changed result accepted")
			}
		})
	}
	if c14ExactCarryEqual(local, native, map[string]any{"status": "ok"}, fields) {
		t.Fatal("incomplete expected result accepted")
	}
}

func TestC14CarryPinsAbsentFields(t *testing.T) {
	native := map[string]any{"status": "ok", "error": "none"}
	local := map[string]any{"status": "failed"}
	expected := map[string]any{"status": "failed"}
	fields := []string{"status", "error"}
	if !c14ExactCarryEqual(local, native, expected, fields) {
		t.Fatal("observed absent field rejected")
	}
	changed := map[string]any{"status": "failed", "error": nil}
	if c14ExactCarryEqual(changed, native, expected, fields) {
		t.Fatal("newly present field accepted")
	}
}

func TestC14CarrySnapshotRequiresExactCase(t *testing.T) {
	native := map[string]any{"status": "native", "rows": nil}
	local := map[string]any{"status": "local", "rows": []any{float64(1)}}
	fields := []string{"status", "rows"}
	records := map[string]map[string]any{"case-a": {"status": "local", "rows": []any{float64(1)}}, "case-b": {"status": "local", "rows": []any{float64(2)}}}
	if !c14CarrySnapshotMatch("case-a", local, native, fields, records) {
		t.Fatal("recorded case rejected")
	}
	if c14CarrySnapshotMatch("case-b", local, native, fields, records) {
		t.Fatal("other case accepted")
	}
	if c14CarrySnapshotMatch("missing", local, native, fields, records) {
		t.Fatal("missing case accepted")
	}
}
