package apextest

import (
	"reflect"
	"testing"
)

func TestPopulatedSObjectHashNativeConformance(t *testing.T) {
	for _, fixture := range []struct {
		family string
		class  string
		keys   []string
	}{
		{"empty_sobject_hash", "EmptyHashProbeTest", []string{"H01", "H03", "H04"}},
		{"adjacent_empty_sobject_hash", "HashAdjacent62Test", []string{"H02"}},
	} {
		t.Run(fixture.family, func(t *testing.T) {
			runInnerTypeNativeConformanceWithComparison(t, fixture.family, fixture.class, func(t *testing.T, got, want any) {
				t.Helper()
				gm, gok := got.(map[string]any)
				wm, wok := want.(map[string]any)
				if !gok || !wok {
					t.Fatal("invalid hash observation")
				}
				for _, key := range fixture.keys {
					if !reflect.DeepEqual(gm[key], wm[key]) {
						t.Errorf("%s = %v; native = %v", key, gm[key], wm[key])
					}
				}
			}, "62.0", "67.0")
		})
	}
}
