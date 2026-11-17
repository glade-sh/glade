package vm

import "testing"

// The Set bucket and the pairwise decimalScalesDiffer agree, including a Decimal
// with no text, which matches any scale.
func TestValueScaleDiffersSetMatchesPairwise(t *testing.T) {
	a, _ := decimalFromText("1.5")
	b, _ := decimalFromText("1.50")
	blank := Decimal(1.5)
	blank.Text = ""
	set := func(values ...Value) Value { return Value{Kind: ValueSet, Set: values} }
	cases := []struct {
		name        string
		left, right Value
	}{
		{"same scale", set(a), set(a)},
		{"different scale", set(a), set(b)},
		{"blank right", set(a), set(blank)},
		{"blank left", set(blank), set(b)},
	}
	for _, c := range cases {
		want := false
		for _, l := range c.left.Set {
			for _, r := range c.right.Set {
				if l.Decimal == r.Decimal && decimalScalesDiffer(l, r) {
					want = true
				}
			}
		}
		// the pairwise rule flags a left element only when no right element of that value has its scale
		matched := false
		for _, l := range c.left.Set {
			any := false
			for _, r := range c.right.Set {
				if l.Decimal == r.Decimal && !decimalScalesDiffer(l, r) {
					any = true
				}
			}
			if !any {
				matched = true
			}
		}
		if got := valueScaleDiffers(c.left, c.right); got != (want && matched) {
			t.Fatalf("%s: bucket %v, pairwise %v", c.name, got, want && matched)
		}
	}
}
