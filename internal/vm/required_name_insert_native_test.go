package vm

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestRequiredNameInsertNativeException(t *testing.T) {
	file, err := os.Open("../../testdata/conformance/required_name_insert/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 9 {
		t.Fatalf("native rows=%d err=%v", len(rows), err)
	}
	for _, row := range rows[1:] {
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			call := "Database.insert(records, true)"
			if row[2] == "C137_029" || row[2] == "C137_113" {
				call = "Database.insert(records, true, AccessLevel.USER_MODE)"
			}
			program, err := CompileAnonymousWithOptions(`
List<Account> records = new List<Account>{new Account(Name='first'), new Account(Name='second'), new Account()};
String message;
try { `+call+`; } catch (DmlException e) { message=e.getMessage(); }
System.assertEquals('`+row[3]+`', message);
System.assertEquals(0, [SELECT COUNT() FROM Account]);
`, CompileOptions{APIVersion: row[0]})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			account := org.Objects["Account"]
			account.Definition.Fields["Name"] = storage.Field{APIName: "Name", Type: storage.FieldString, Required: true}
			org.Objects["Account"] = account
			org.APIVersion = row[0]
			machine.SetOrg(&org)
			if _, err := machine.ExecuteInClass(program, "RequiredNameInsert"); err != nil {
				t.Fatal(err)
			}
		})
	}
}
