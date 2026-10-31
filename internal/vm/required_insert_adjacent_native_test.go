package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRequiredInsertAdjacentNativeException(t *testing.T) {
	f, err := os.Open("../../testdata/conformance/insert_event_child/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil || len(rows) != 5 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		var obs map[string]json.RawMessage
		if err := json.Unmarshal([]byte(row[2]), &obs); err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct{ id, object, records, suffix string }{
			{"D1", "Contact", "new Contact(LastName='One'),new Contact(LastName='Two'),new Contact()", ""},
			{"D4", "Contact", "new Contact(LastName='One'),new Contact(LastName='Two'),new Contact()", ",AccessLevel.USER_MODE"},
			{"D2", "Opportunity", "new Opportunity(Name='One',StageName='Prospecting',CloseDate=Date.today()),new Opportunity(Name='Two',StageName='Prospecting',CloseDate=Date.today()),new Opportunity(StageName='Prospecting',CloseDate=Date.today())", ""},
			{"D5", "Opportunity", "new Opportunity(Name='One',StageName='Prospecting',CloseDate=Date.today()),new Opportunity(Name='Two',StageName='Prospecting',CloseDate=Date.today()),new Opportunity(StageName='Prospecting',CloseDate=Date.today())", ",AccessLevel.USER_MODE"},
		} {
			t.Run(strings.Join([]string{row[0], row[1], c.id}, "/"), func(t *testing.T) {
				var want string
				if err := json.Unmarshal(obs[c.id], &want); err != nil {
					t.Fatal(err)
				}
				_, want, ok := strings.Cut(want, "|")
				if !ok {
					t.Fatal("native exception missing")
				}
				code := `String message; try { Database.insert(new List<` + c.object + `>{` + c.records + `},true` + c.suffix + `); } catch(DmlException e) { message=e.getMessage(); } System.assertEquals('` + want + `',message);`
				program, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				storage.EnsureDeterministicPlatformData(&org)
				org.APIVersion = row[0]
				machine := New(nil)
				machine.SetOrg(&org)
				if _, err := machine.ExecuteInClass(program, "AdjacentRequiredInsert"); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
