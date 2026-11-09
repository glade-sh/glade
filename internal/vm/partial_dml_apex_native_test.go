package vm

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestPartialDMLMultifieldNativeApexRoute(t *testing.T) {
	f, err := os.Open("testdata/partial_dml_multifield/org.tsv")
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
	for _, row := range rows[1:] {
		t.Run(row[1]+"/"+row[0], func(t *testing.T) {
			var p struct {
				Results []struct {
					Errors []struct {
						Fields  []string `json:"getFields"`
						Message string   `json:"getMessage"`
						Status  string   `json:"getStatusCode"`
					} `json:"errors"`
				} `json:"results"`
			}
			if err := json.Unmarshal([]byte(row[3]), &p); err != nil {
				t.Fatal(err)
			}
			native := p.Results[0].Errors[0]
			machine := recordSharingControlMachine(t, "Private")
			state := machine.Org.Objects["Widget__c"]
			for _, field := range []string{"Zeta__c", "Alpha__c", "Mid__c"} {
				state.Definition.Fields[field] = storage.Field{APIName: field, Type: storage.FieldString}
			}
			machine.Org.Objects["Widget__c"] = state
			args := "false"
			if strings.Contains(row[0], "-user-") {
				args += ",AccessLevel.USER_MODE"
			}
			message := strings.ReplaceAll(native.Message, "SynthPartial__c", "Widget__c")
			source := `Widget__c row=new Widget__c(Name='synthetic');row.Zeta__c='z';row.Alpha__c='a';row.Mid__c='m';Database.SaveResult saved=Database.insert(row,` + args + `);System.assertEquals(false,saved.isSuccess());Database.Error e=saved.getErrors()[0];System.assertEquals('` + message + `',e.getMessage());System.assertEquals('` + native.Status + `',String.valueOf(e.getStatusCode()));`
			for i, field := range native.Fields {
				source += `System.assertEquals('` + field + `',e.getFields()[` + string(rune('0'+i)) + `]);`
			}
			program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: row[1]})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
