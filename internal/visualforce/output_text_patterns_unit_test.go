package visualforce

import (
	"encoding/csv"
	"errors"
	"html"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestOutputTextPatternsNativeArgumentUnitRoute(t *testing.T) {
	read := func(path string) [][]string {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.Comma = '\t'
		rows, err := r.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows[1:]
	}
	cases := read("testdata/output_text_patterns/cases.tsv")
	native := read("testdata/output_text_patterns/api67/org.tsv")
	expected := map[string]string{}
	for _, row := range native {
		expected[row[1]] = html.UnescapeString(row[3])
	}
	for _, row := range cases {
		t.Run(row[0], func(t *testing.T) {
			values := map[string]vm.Value{"amount": vm.Decimal(12345.625), "negative": vm.Decimal(-12345.625), "ratio": vm.Decimal(0.125), "zero": vm.Decimal(0)}
			stamp := vm.Object("Datetime")
			stamp.Fields["value"] = vm.String("2024-02-09T17:06:03Z")
			values["stamp"] = stamp
			day := vm.Object("Date")
			day.Fields["value"] = vm.String("2024-02-09")
			values["day"] = day
			got, err := formatOutputTextArgument(values[row[3]], []string{row[1], row[2]}, nil)
			if err != nil {
				t.Fatal(err)
			}
			want, ok := expected[row[0]]
			if !ok {
				t.Fatal("missing native row")
			}
			if got != want {
				t.Fatalf("value=%q, native=%q", got, want)
			}
		})
	}
}

// Numeric fields and quoted literals retain their existing locale-independent
// rendering. Newly captured English names stay behind the locale boundary.
func TestOutputTextPatternsQuotedDateLocaleUnitRoute(t *testing.T) {
	machine := vm.New(nil)
	user := storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue("de_DE")}}
	machine.SetOrg(&storage.OrgState{Objects: map[string]storage.ObjectState{"User": {Records: map[storage.ID]storage.Record{user.ID: user}}}})
	machine.SetCurrentUser(user)
	stamp := vm.Object("Datetime")
	stamp.Fields["value"] = vm.String("2024-02-09T17:06:03Z")
	ctx := &RenderContext{VM: machine}
	if locale := controllerFieldLocale(ctx); locale != "de_DE" {
		t.Fatalf("locale=%q", locale)
	}
	got, err := formatOutputTextArgument(stamp, []string{"date", "yyyy-MM-dd 'at' HH:mm 'MMM E'"}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != "2024-02-09 at 17:06 MMM E" {
		t.Fatalf("quoted date=%q", got)
	}
	for _, pattern := range []string{"MMM d, yyyy", "EEEE", "h:mm a"} {
		_, err := formatOutputTextArgument(stamp, []string{"date", pattern}, ctx)
		var runtime *vm.RuntimeError
		if !errors.As(err, &runtime) || runtime.Type != "UnsupportedFeature" {
			t.Fatalf("pattern=%q error=%v, want locale boundary", pattern, err)
		}
	}
}
