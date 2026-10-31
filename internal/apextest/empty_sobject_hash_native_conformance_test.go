package apextest

import "testing"

func TestEmptySObjectHashNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "empty_sobject_hash", "EmptyHashProbeTest", func(t *testing.T, got, want any) {
		t.Helper()
		gm, gok := got.(map[string]any)
		wm, wok := want.(map[string]any)
		if !gok || !wok {
			t.Fatal("invalid hash observation")
		}
		gs, gok := gm["H02"].([]any)
		ws, wok := wm["H02"].([]any)
		if !gok || !wok || len(gs) != 2 || len(ws) != 2 || gs[0] != ws[0] || gs[1] != ws[1] {
			t.Fatalf("empty Account/Contact hashes = %v; native = %v", gm["H02"], wm["H02"])
		}
	}, "62.0", "67.0")
}
