package vm

import (
	"encoding/csv"
	"fmt"
	"github.com/glade-sh/glade/internal/storage"
	"os"
	"strings"
	"testing"
)

func TestNorwegianDateDatetimeAnonymousAndIsTestNativeRoutes(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/generic_assertion_residue/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		for _, locale := range []struct {
			name           string
			date, datetime int
		}{{"no", 5, 7}, {"no_NO", 6, 8}} {
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(row[0]+"/"+route+"/"+locale.name, func(t *testing.T) {
					source := fmt.Sprintf("System.assertEquals('%s',Date.newInstance(2025,11,23).format()); System.assertEquals('%s',Datetime.newInstance(2025,11,23,14,30,0).format());", strings.ReplaceAll(row[locale.date], "'", "\\'"), strings.ReplaceAll(row[locale.datetime], "'", "\\'"))
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[0]})
					if err != nil {
						t.Fatal(err)
					}
					machine := New(nil)
					machine.SetCurrentUser(storage.Record{ID: "005-synthetic-locale", Object: "User", Fields: map[string]storage.Value{"LocaleSidKey": storage.StringValue(locale.name), "LanguageLocaleKey": storage.StringValue("en_US"), "TimeZoneSidKey": storage.StringValue("GMT")}})
					if route == "istest" {
						machine.EnableTestContext()
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
