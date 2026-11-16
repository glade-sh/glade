package vm

import (
	"strings"
	"testing"
)

// A digit string beyond the Double range keeps 33 significant digits and a
// negative scale; rounding must not go through float64 overflow.
func TestDecimalValueOfWideMagnitude(t *testing.T) {
	for _, sign := range []string{"", "-"} {
		text := sign + "1" + strings.Repeat("0", 309)
		got, err := decimalValueOfText(text)
		if err != nil {
			t.Fatalf("%q: %v", text[:12], err)
		}
		if got.Kind != ValueDecimal {
			t.Fatalf("kind = %v", got.Kind)
		}
		rat, ok := valueDecimalRat(got)
		if !ok {
			t.Fatal("not a rational")
		}
		digits := strings.TrimLeft(rat.Num().String(), "-")
		digits = strings.TrimRight(digits, "0")
		if digits != "1" {
			t.Fatalf("digits = %s", digits)
		}
		if got := decimalPrecision(got); got != 33 {
			t.Fatalf("precision = %d", got)
		}
		if got := decimalScale(got); got != -277 {
			t.Fatalf("scale = %d", got)
		}
	}
}

func TestDecimalValueOfWideMagnitudeRounds(t *testing.T) {
	text := "1234567890123456789012345678901234567890" + strings.Repeat("7", 270)
	got, err := decimalValueOfText(text)
	if err != nil {
		t.Fatal(err)
	}
	if p, s := decimalPrecision(got), decimalScale(got); p != 33 || s != -277 {
		t.Fatalf("precision=%d scale=%d", p, s)
	}
	if !strings.HasPrefix(got.Text, "1.23456789012345678901234567890123E+309") {
		t.Fatalf("text = %.60s", got.Text)
	}
}
