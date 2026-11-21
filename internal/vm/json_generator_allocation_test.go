package vm

import (
	"strings"
	"testing"
)

func TestJSONGeneratorAppendAllocationImprovement(t *testing.T) {
	chunk := strings.Repeat("x", 64)
	want := strings.Repeat(chunk, 256)
	measure := func(optimized bool) float64 {
		return testing.AllocsPerRun(3, func() {
			generator := newJSONGenerator(false)
			for range 256 {
				if optimized {
					jsonGeneratorAppend(&generator, chunk)
				} else {
					current := jsonGeneratorStringField(generator, "out").Text
					generator.Fields["out"] = String(current + chunk)
				}
			}
			if got := jsonGeneratorStringField(generator, "out").Text; got != want {
				panic("generator output changed")
			}
		})
	}
	optimized, parent := measure(true), measure(false)
	if optimized >= parent {
		t.Fatalf("append allocations = %v, parent algorithm = %v", optimized, parent)
	}
}
