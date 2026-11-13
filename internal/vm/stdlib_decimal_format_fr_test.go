package vm

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE testdata/decimal_format_fr/org.tsv rows 2–5 capture Decimal.format
// in named tests using runAs with the stated user locales.
func TestDecimalFormatFrenchLocaleNative(t *testing.T) {
	data, err := os.ReadFile("testdata/decimal_format_fr/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 5 || lines[0] != "api\troute\tlocale\tinput\toutput" {
		t.Fatal("invalid native table")
	}
	for _, line := range lines[1:] {
		row := strings.Split(line, "\t")
		if len(row) != 5 || row[1] != "istest-runAs" {
			t.Fatalf("invalid row %q", line)
		}
		t.Run(row[0]+"/"+row[2], func(t *testing.T) {
			source := fmt.Sprintf(`
System.runAs(new User(Id='005-locale-inner', LocaleSidKey='%s', LanguageLocaleKey='en_US')) {
 System.assertEquals('%s', UserInfo.getLocale());
 System.assertEquals('%s', Decimal.valueOf('%s').format());
}
System.assertEquals('en_US', UserInfo.getLocale());
System.assertEquals('1,234,567.89', Decimal.valueOf('1234567.89').format());
`, row[2], row[2], row[4], row[3])
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetCurrentUser(storage.Record{ID: "005-locale-outer", Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue("en_US"), "LanguageLocaleKey": storage.StringValue("en_US")}})
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// NATIVE testdata/decimal_format_fr/numeric.tsv rows 2–49 capture the
// named runAs numeric controls, including the half-even fractional cases.
func TestFrenchNumericFormatNative(t *testing.T) {
	data, err := os.ReadFile("testdata/decimal_format_fr/numeric.tsv")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 49 || lines[0] != "api\troute\tlocale\ttype\tinput\toutput" {
		t.Fatal("invalid native table")
	}
	for _, line := range lines[1:] {
		row := strings.Split(line, "\t")
		if len(row) != 6 || row[1] != "istest-runAs" {
			t.Fatalf("invalid row %q", line)
		}
		t.Run(strings.Join([]string{row[0], row[2], row[3], row[4]}, "/"), func(t *testing.T) {
			if row[3] != "Decimal" && row[3] != "Integer" && row[3] != "Long" {
				t.Fatalf("invalid numeric type %q", row[3])
			}
			source := fmt.Sprintf(`System.runAs(new User(Id='005-locale-inner', LocaleSidKey='%s', LanguageLocaleKey='en_US')) { System.assertEquals('%s', %s.valueOf('%s').format()); }`, row[2], row[5], row[3], row[4])
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetCurrentUser(storage.Record{ID: "005-locale-outer", Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue("en_US"), "LanguageLocaleKey": storage.StringValue("en_US")}})
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
