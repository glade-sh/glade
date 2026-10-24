package visualforce

import (
	"encoding/csv"
	"errors"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Isolate numeric formatting from controller and markup rendering. Expected
// values come from the same native Direct route as the project regression.
func TestOutputTextParameterNativeNumberUnitRoute(t *testing.T) {
	f, err := os.Open("testdata/output_text_parameters/api67/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		value   vm.Value
		pattern string
	}{
		"integer":  {vm.Decimal(12345.625), " ###,###,###,##0"},
		"decimal":  {vm.Decimal(12345.625), " ###,###,###,##0.00"},
		"negative": {vm.Decimal(-12345.625), " ###,###,###,##0.00"},
		"zero":     {vm.Decimal(0), " 000,000.00"},
		"tie":      {vm.Decimal(2.5), "0"},
	}
	seen := 0
	for _, row := range rows[1:] {
		if row[1] != "Direct" {
			continue
		}
		c, ok := cases[row[2]]
		if !ok {
			continue
		}
		seen++
		t.Run(row[2], func(t *testing.T) {
			got, err := formatOutputTextNumber(c.value, c.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if got != row[3] {
				t.Fatalf("number=%q, native=%q", got, row[3])
			}
		})
	}
	if seen != len(cases) {
		t.Fatalf("native numeric rows=%d, want %d", seen, len(cases))
	}
}

// The date-pattern helper requires a concrete pattern. Public default and named
// styles are resolved before entering it.
func TestOutputTextParameterDefaultDateBoundaryUnitRoute(t *testing.T) {
	value := vm.Object("Date")
	value.Fields["value"] = vm.String("2024-02-09")
	_, err := formatOutputTextDate(value, "")
	var runtime *vm.RuntimeError
	if !errors.As(err, &runtime) || runtime.Type != "UnsupportedFeature" {
		t.Fatalf("default date error=%v, want unsupported boundary", err)
	}
}
