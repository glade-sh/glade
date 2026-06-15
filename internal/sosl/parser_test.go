package sosl_test

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/sosl"
)

func TestParseSOSLReturningObjectsAndFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  sosl.Query
	}{
		{
			name:  "braced find",
			input: "FIND {Acme} RETURNING Account(Id, Name)",
			want: sosl.Query{Returning: []sosl.ReturningObject{{
				Object: "Account",
				Fields: []string{"Id", "Name"},
			}}},
		},
		{
			name:  "quoted find in all fields",
			input: "FIND 'Acme' IN ALL FIELDS RETURNING Account(Id, Name), Contact(Id)",
			want: sosl.Query{Returning: []sosl.ReturningObject{
				{Object: "Account", Fields: []string{"Id", "Name"}},
				{Object: "Contact", Fields: []string{"Id"}},
			}},
		},
		{
			name:  "bind find with where clause",
			input: "FIND :term RETURNING Invoice__c(Id, Amount__c WHERE Amount__c > 0)",
			want: sosl.Query{Returning: []sosl.ReturningObject{{
				Object: "Invoice__c",
				Fields: []string{"Id", "Amount__c"},
			}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sosl.Parse(tt.input)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Parse() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
