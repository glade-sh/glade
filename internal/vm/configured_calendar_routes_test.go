package vm

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/glade-sh/glade/internal/storage"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNativeCalendarAnonymousAndTestContextRoutes(t *testing.T) {
	f, err := os.Open(filepath.Join("..", "apextest", "testdata", "conformance", "configured_user_clock", "org.tsv"))
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
	for _, row := range rows[1:] {
		if row[1] == "adminDefaults" {
			continue
		}
		var values []string
		if err := json.Unmarshal([]byte(row[2]), &values); err != nil {
			t.Fatal(err)
		}
		for _, route := range []string{"anonymous", "test_context"} {
			t.Run(row[0]+"/"+row[1]+"/"+route, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(fmt.Sprintf(`System.assertEquals('%s', String.valueOf(Date.today())); System.assertEquals('%s', Datetime.newInstanceGmt(2026,7,15,0,30,0).format());`, values[11], values[10]), CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "test_context" {
					machine.EnableTestContext()
				}
				machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"TimeZoneSidKey": storage.StringValue(values[1]), "LocaleSidKey": storage.StringValue(values[2]), "LanguageLocaleKey": storage.StringValue(values[3])}})
				if route == "test_context" {
					machine.testContext.CurrentUser = machine.executionUser
				}
				machine.fakeNow, err = time.Parse("2006-01-02 15:04:05", values[0])
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatalf("native calendar %s: %v", row[2], err)
				}
			})
		}
	}
}
