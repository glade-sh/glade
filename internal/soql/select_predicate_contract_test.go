package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSelectGrammarFamilyRequiresClauses(t *testing.T) {
	if _, err := Parse("SELECT Id FROM Account"); err != nil {
		t.Fatalf("valid SELECT/FROM control failed: %v", err)
	}
	for _, query := range []struct {
		name string
		text string
	}{
		{name: "missing SELECT", text: "FROM Account SELECT Id"},
		{name: "missing FROM", text: "SELECT Id"},
		{name: "missing projection", text: "SELECT FROM Account"},
		{name: "clause order", text: "SELECT Id FROM Account ORDER BY Name WHERE Name = 'Acme'"},
	} {
		t.Run(query.name, func(t *testing.T) {
			if _, err := Parse(query.text); err == nil {
				t.Fatalf("Parse(%q) succeeded; want SELECT grammar rejection", query.text)
			}
		})
	}
}

func TestSelectAcceptsTypeofOnlyAndChildOnlyProjections(t *testing.T) {
	for _, tt := range []struct {
		name       string
		query      string
		wantChild  bool
		wantTypeof bool
	}{
		{
			name:       "TYPEOF-only",
			query:      "SELECT TYPEOF What WHEN Account THEN Name END FROM Task",
			wantTypeof: true,
		},
		{
			name:      "child-query-only",
			query:     "SELECT (SELECT Id FROM Contacts) FROM Account",
			wantChild: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			query, err := Parse(tt.query)
			if err != nil {
				t.Fatalf("Parse(%q) rejected a nonempty projection collection: %v", tt.query, err)
			}
			if len(query.Fields) != 0 {
				t.Fatalf("ordinary fields = %v, want none for this projection shape", query.Fields)
			}
			if got := len(query.ChildQueries) > 0; got != tt.wantChild {
				t.Fatalf("child-query projection present = %t, want %t", got, tt.wantChild)
			}
			if got := len(query.Typeofs) > 0; got != tt.wantTypeof {
				t.Fatalf("TYPEOF projection present = %t, want %t", got, tt.wantTypeof)
			}
		})
	}
}

func TestSOQLStringEscapeFamilyRules(t *testing.T) {
	valid := []struct {
		name      string
		condition string
		want      string
	}{
		{name: "newline", condition: `Name = 'A\nB'`, want: "A\nB"},
		{name: "uppercase newline", condition: `Name = 'A\NB'`, want: "A\nB"},
		{name: "carriage return", condition: `Name = 'A\rB'`, want: "A\rB"},
		{name: "uppercase carriage return", condition: `Name = 'A\RB'`, want: "A\rB"},
		{name: "tab", condition: `Name = 'A\tB'`, want: "A\tB"},
		{name: "uppercase tab", condition: `Name = 'A\TB'`, want: "A\tB"},
		{name: "bell", condition: `Name = 'A\bB'`, want: "A\aB"},
		{name: "uppercase bell", condition: `Name = 'A\BB'`, want: "A\aB"},
		{name: "form feed", condition: `Name = 'A\fB'`, want: "A\fB"},
		{name: "uppercase form feed", condition: `Name = 'A\FB'`, want: "A\fB"},
		{name: "escaped quote", condition: `Name = 'Bob\'s Shop'`, want: "Bob's Shop"},
		{name: "escaped double quote", condition: `Name = 'A\"B'`, want: `A"B`},
		{name: "escaped backslash", condition: `Name = 'C:\\Trail'`, want: `C:\Trail`},
		{name: "unicode", condition: `Name = '\u0041'`, want: "A"},
		{name: "LIKE-only underscore", condition: `Name LIKE 'A\_B'`, want: `A\_B`},
		{name: "LIKE-only percent", condition: `Name LIKE 'A\%B'`, want: `A\%B`},
	}
	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			query, err := Parse("SELECT Id FROM Account WHERE " + tt.condition)
			if err != nil {
				t.Fatalf("Parse escaped condition: %v", err)
			}
			if query.Where == nil || query.Where.Value.String != tt.want {
				t.Fatalf("parsed value = %#v, want %q", query.Where, tt.want)
			}
		})
	}

	for _, condition := range []string{
		`Name = 'bad\q'`,
		`Name LIKE 'bad\q'`,
		`Name = 'bad\%'`,
		`Name = 'bad\_'`,
	} {
		t.Run("reject "+condition, func(t *testing.T) {
			if _, err := Parse("SELECT Id FROM Account WHERE " + condition); err == nil {
				t.Fatalf("Parse accepted invalid backslash context %q", condition)
			}
		})
	}
	result, err := ParseAndExecute(selectPredicateTestOrg(), `SELECT Id FROM Account WHERE Name LIKE 'Acme\%'`)
	if err != nil {
		t.Fatalf("Execute LIKE literal-percent control: %v", err)
	}
	assertSelectPredicateIDs(t, result, []string{"001000000000003"})
	result, err = ParseAndExecute(selectPredicateTestOrg(), `SELECT Id FROM Account WHERE Name LIKE 'A\_B'`)
	if err != nil {
		t.Fatalf("Execute LIKE literal-underscore control: %v", err)
	}
	assertSelectPredicateIDs(t, result, []string{"001000000000004"})
}

func TestMultiSelectPredicateFamily(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "equals matches the exact selected set",
			query: "SELECT Id FROM Product__c WHERE PriceClasses__c = 'AAA;BBB'",
			want:  []string{"a01000000000001"},
		},
		{
			name:  "semicolon operands are conjunctive",
			query: "SELECT Id FROM Product__c WHERE PriceClasses__c INCLUDES ('AAA;BBB')",
			want:  []string{"a01000000000001", "a01000000000002"},
		},
		{
			name:  "comma operands are alternative groups",
			query: "SELECT Id FROM Product__c WHERE PriceClasses__c INCLUDES ('AAA;BBB','CCC')",
			want:  []string{"a01000000000001", "a01000000000002", "a01000000000003"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAndExecute(selectPredicateTestOrg(), tt.query)
			if err != nil {
				t.Fatal(err)
			}
			assertSelectPredicateIDs(t, result, tt.want)
		})
	}
}

func TestSemiAntiJoinSourceRestrictions(t *testing.T) {
	valid := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "ID semi-join in main WHERE",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE LastName = 'Smith')",
			want:  []string{"001000000000001"},
		},
		{
			name:  "ID anti-join in main WHERE",
			query: "SELECT Id FROM Account WHERE Name = 'Beta' AND Id NOT IN (SELECT AccountId FROM Contact WHERE LastName = 'Smith')",
			want:  []string{"001000000000002"},
		},
		{
			name:  "reference semi-join direction",
			query: "SELECT Id FROM Contact WHERE AccountId IN (SELECT Id FROM Account WHERE Name = 'Acme')",
			want:  []string{"003000000000001"},
		},
		{
			name:  "two joins are permitted",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE LastName = 'Smith') AND Id IN (SELECT AccountId FROM Opportunity)",
			want:  []string{"001000000000001"},
		},
	}
	for _, tt := range valid {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAndExecute(selectPredicateTestOrg(), tt.query)
			if err != nil {
				t.Fatal(err)
			}
			assertSelectPredicateIDs(t, result, tt.want)
		})
	}

	invalid := []struct {
		name  string
		query string
	}{
		{
			name:  "outer operand must be ID or reference",
			query: "SELECT Id FROM Account WHERE Name IN (SELECT AccountId FROM Contact)",
		},
		{
			name:  "outer operand cannot traverse a relationship",
			query: "SELECT Id FROM Contact WHERE Account.Id IN (SELECT Id FROM Account)",
		},
		{
			name:  "subquery key must reference the outer object",
			query: "SELECT Id FROM Opportunity WHERE Id IN (SELECT AccountId FROM Contact)",
		},
		{
			name:  "subquery field must be a foreign key",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT LastName FROM Contact)",
		},
		{
			name:  "subquery selected field cannot traverse a relationship",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT Account.Id FROM Contact)",
		},
		{
			name:  "self semi-join is forbidden",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT Id FROM Account)",
		},
		{
			name:  "nested semi-join is forbidden",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE AccountId IN (SELECT Id FROM Account))",
		},
		{
			name:  "semi-join cannot be negated with NOT",
			query: "SELECT Id FROM Account WHERE NOT (Id IN (SELECT AccountId FROM Contact))",
		},
		{
			name:  "semi-join is not allowed in HAVING",
			query: "SELECT Name, COUNT(Id) FROM Account GROUP BY Name HAVING COUNT(Id) > 0 AND Id IN (SELECT AccountId FROM Contact)",
		},
		{
			name:  "semi-join cannot be combined with outer OR",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact) OR Name = 'Beta'",
		},
		{
			name:  "three joins exceed the per-WHERE maximum",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact) AND Id IN (SELECT AccountId FROM Opportunity) AND Id NOT IN (SELECT AccountId FROM Contact)",
		},
		{
			name:  "subquery cannot select multiple fields",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT AccountId, LastName FROM Contact)",
		},
		{
			name:  "subquery cannot use ORDER BY",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact ORDER BY LastName)",
		},
		{
			name:  "subquery cannot use LIMIT",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact LIMIT 1)",
		},
		{
			name:  "subquery cannot use FOR UPDATE",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact FOR UPDATE)",
		},
		{
			name:  "excluded subquery object",
			query: "SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Event)",
		},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseAndExecute(selectPredicateTestOrg(), tt.query); err == nil {
				t.Fatalf("query accepted despite source restriction: %s", tt.query)
			}
		})
	}
}

func TestSemiAntiJoinConstructedQueryValidation(t *testing.T) {
	query, err := Parse("SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE LastName = 'Smith')")
	if err != nil {
		t.Fatal(err)
	}
	query.Where.Field = "Name"
	if _, err := Execute(selectPredicateTestOrg(), query); err == nil {
		t.Fatal("Execute accepted a constructed semi-join with a scalar outer operand")
	}
}

func TestSemiAntiJoinUsesResolvedNamespacedObjectIdentity(t *testing.T) {
	org := storage.NewOrgState()
	org.Namespace = "ns"
	org.Objects["ns__Thing__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "ns__Thing__c",
			Fields:  map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {ID: "a01000000000001", Object: "ns__Thing__c", Fields: map[string]storage.Value{"Name": storage.StringValue("parent")}},
		},
	}
	org.Objects["ns__Child__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "ns__Child__c",
			Fields: map[string]storage.Field{
				"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Thing__c"}},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a02000000000001": {ID: "a02000000000001", Object: "ns__Child__c", Fields: map[string]storage.Value{"Parent__c": storage.IDValue("a01000000000001")}},
		},
	}

	result, err := ParseAndExecute(org, "SELECT Id FROM Thing__c WHERE Id IN (SELECT Parent__c FROM ns__Child__c)")
	if err != nil {
		t.Fatalf("namespaced reference semi-join failed: %v", err)
	}
	assertSelectPredicateIDs(t, result, []string{"a01000000000001"})

	if _, err := ParseAndExecute(org, "SELECT Id FROM Thing__c WHERE Id IN (SELECT Id FROM ns__Thing__c)"); err == nil {
		t.Fatal("namespaced alias allowed a semi-join against the same resolved object")
	}
}

func assertSelectPredicateIDs(t *testing.T, result Result, want []string) {
	t.Helper()
	got := make(map[string]bool, len(result.Records))
	for _, record := range result.Records {
		got[string(record.ID)] = true
	}
	if len(got) != len(want) {
		t.Fatalf("record IDs = %v, want %v", got, want)
	}
	for _, id := range want {
		if !got[id] {
			t.Errorf("record IDs = %v, missing %q", got, id)
		}
	}
}

func selectPredicateTestOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Account",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Beta")}},
			"001000000000003": {ID: "001000000000003", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme%")}},
			"001000000000004": {ID: "001000000000004", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("A_B")}},
		},
	}
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields: map[string]storage.Field{
				"LastName":  {APIName: "LastName", Type: storage.FieldString},
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account"},
			},
		},
		Records: map[storage.ID]storage.Record{
			"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Smith"), "AccountId": storage.IDValue("001000000000001")}},
			"003000000000002": {ID: "003000000000002", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Jones"), "AccountId": storage.IDValue("001000000000002")}},
		},
	}
	org.Objects["Opportunity"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Opportunity",
			Fields: map[string]storage.Field{
				"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
			},
		},
		Records: map[storage.ID]storage.Record{
			"006000000000001": {ID: "006000000000001", Object: "Opportunity", Fields: map[string]storage.Value{"AccountId": storage.IDValue("001000000000001")}},
		},
	}
	org.Objects["Product__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Product__c",
			Fields: map[string]storage.Field{
				"PriceClasses__c": {APIName: "PriceClasses__c", Type: storage.FieldMultiPicklist, DisplayType: "MULTIPICKLIST"},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a01000000000001": {ID: "a01000000000001", Object: "Product__c", Fields: map[string]storage.Value{"PriceClasses__c": storage.StringValue("AAA;BBB")}},
			"a01000000000002": {ID: "a01000000000002", Object: "Product__c", Fields: map[string]storage.Value{"PriceClasses__c": storage.StringValue("AAA;BBB;CCC")}},
			"a01000000000003": {ID: "a01000000000003", Object: "Product__c", Fields: map[string]storage.Value{"PriceClasses__c": storage.StringValue("CCC")}},
			"a01000000000004": {ID: "a01000000000004", Object: "Product__c", Fields: map[string]storage.Value{"PriceClasses__c": storage.StringValue("AAA")}},
			"a01000000000005": {ID: "a01000000000005", Object: "Product__c", Fields: map[string]storage.Value{}},
		},
	}
	return org
}
