package vm

import "testing"

func TestMatcherPositions(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		bytes       []int
		inside      []bool
	}{
		{"ascii", "abc", []int{0, 1, 2, 3}, []bool{false, false, false, false}},
		{"bmp", "aéz", []int{0, 1, 3, 4}, []bool{false, false, false, false}},
		{"supplementary", "a😀b", []int{0, 1, 1, 5, 6}, []bool{false, false, true, false, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newMatcherPositions(tc.input)
			for unit, wantByte := range tc.bytes {
				at, inside, ok := p.byteAtUnit(unit)
				if !ok || at != wantByte || inside != tc.inside[unit] {
					t.Fatalf("unit %d: got %d, %v, %v; want %d, %v, true", unit, at, inside, ok, wantByte, tc.inside[unit])
				}
				if !inside {
					got, ok := p.unitAtByte(at)
					if !ok || got != unit {
						t.Fatalf("byte %d: got %d, %v; want %d, true", at, got, ok, unit)
					}
				}
			}
			if _, _, ok := p.byteAtUnit(len(tc.bytes)); ok {
				t.Fatal("accepted unit beyond end")
			}
		})
	}
}
