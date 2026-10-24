package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSOSLTemporalNullBindRouteTwins(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, row := range []struct {
			name, condition string
			want            int
		}{
			{"date literal", "Birthdate > 2020-01-01", 1},
			{"datetime literal", "ObservedAt__c > 2020-01-01T08:00:00Z", 1},
			{"date bind", "Birthdate = :day", 1},
			{"datetime bind", "ObservedAt__c = :stamp", 1},
			{"date IN bind", "Birthdate IN :days", 2},
			{"datetime IN bind", "ObservedAt__c IN :times", 2},
			{"date null bind", "Birthdate = :emptyDay", 1},
			{"datetime null bind", "ObservedAt__c = :emptyStamp", 1},
		} {
			query := "FIND 'Temporal' IN ALL FIELDS RETURNING Contact(Id WHERE " + row.condition + ")"
			for route, expression := range map[string]string{
				"inline": "[" + query + "]",
				"search": "Search.query('" + strings.ReplaceAll(query, "'", "\\'") + "')",
			} {
				t.Run(api+"/"+route+"/"+row.name, func(t *testing.T) {
					source := fmt.Sprintf(`Datetime stamp=Datetime.newInstanceGmt(2020,1,1,8,0,0);
Date day=Date.newInstance(2020,1,1);
Date emptyDay; Datetime emptyStamp;
Set<Date> days=new Set<Date>{day,null};
Set<Datetime> times=new Set<Datetime>{stamp,null};
List<Contact> records=new List<Contact>{
 new Contact(LastName='Temporal probe',Birthdate=day,ObservedAt__c=stamp),
 new Contact(LastName='Temporal probe',Birthdate=day.addDays(1),ObservedAt__c=stamp.addMinutes(1)),
 new Contact(LastName='Temporal probe')
};
insert records;
Test.setFixedSearchResults(new List<Id>(new Map<Id,Contact>(records).keySet()));
List<List<SObject>> matches=%s;
System.assertEquals(%d,matches[0].size());`, expression, row.want)
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := testDataOrg()
					org.APIVersion = api
					storage.EnsureStandardObject(&org, "Contact")
					contact := org.Objects["Contact"]
					contact.Definition.Fields["ObservedAt__c"] = storage.Field{APIName: "ObservedAt__c", Type: storage.FieldDateTime}
					machine := New(nil)
					machine.SetOrg(&org)
					machine.EnableTestContext()
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}
