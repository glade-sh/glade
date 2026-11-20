package vm

import (
	"fmt"
	"sync"
	"testing"
)

func TestJSONGeneratorCopiedMidBuild(t *testing.T) {
	for _, concurrent := range []bool{false, true} {
		t.Run(fmt.Sprintf("concurrent=%t", concurrent), func(t *testing.T) {
			start := newJSONGenerator(false)
			_, start, _, _, _ = jsonGeneratorWriteContainer(start, "array")
			_, start, _, _, _ = jsonGeneratorWriteScalar(start, String("prefix"))
			left, right := cloneValue(start), cloneValue(start)
			// Retain the shared immutable output prefix, as value copies do.
			left.Fields["out"] = start.Fields["out"]
			right.Fields["out"] = start.Fields["out"]
			outputs := [2]string{}
			errors := [2]error{}
			run := func(i int, g Value) {
				for n := 0; n < 64; n++ {
					_, g, _, _, errors[i] = jsonGeneratorWriteScalar(g, Int(int64(n+i)))
					if errors[i] != nil {
						return
					}
				}
				g, errors[i] = jsonGeneratorFinish(g)
				outputs[i] = jsonGeneratorStringField(g, "out").Text
			}
			if concurrent {
				var wg sync.WaitGroup
				wg.Add(2)
				go func() { defer wg.Done(); run(0, left) }()
				go func() { defer wg.Done(); run(1, right) }()
				wg.Wait()
			} else {
				run(0, left)
				run(1, right)
			}
			for i := range outputs {
				if errors[i] != nil {
					t.Fatal(errors[i])
				}
				want := `["prefix"`
				for n := 0; n < 64; n++ {
					want += fmt.Sprintf(",%d", n+i)
				}
				want += "]"
				if outputs[i] != want {
					t.Fatalf("copy %d = %s; want %s", i, outputs[i], want)
				}
			}
		})
	}
}
