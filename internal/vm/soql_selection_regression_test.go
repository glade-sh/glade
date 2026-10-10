package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/soql"
)

func TestSOQLSelectionDateMembershipUsesSourceOperators(t *testing.T) {
	// Native API62/67 review controls Q008/Q009 accept lowered date NOT IN.
	for _, queryText := range []string{
		"SELECT Id FROM Contact WHERE LastName='X' OR Birthdate NOT IN (TODAY,YESTERDAY)",
		"SELECT Id FROM Contact WHERE LastName='X' AND Birthdate NOT IN (TODAY,YESTERDAY)",
	} {
		query, err := soql.Parse(queryText)
		if err != nil {
			t.Fatal(err)
		}
		if err := New(nil).validateSOQLSelection(query, true); err != nil {
			t.Fatalf("native-accepted query %q: %v", queryText, err)
		}
	}
	// R150 still rejects an unparenthesized source AND inside an OR.
	query, err := soql.Parse("SELECT Id FROM Contact WHERE LastName='X' OR LastName='Y' AND Birthdate=TODAY")
	if err != nil {
		t.Fatal(err)
	}
	if err := New(nil).validateSOQLSelection(query, true); err == nil {
		t.Fatal("unparenthesized source AND was accepted")
	}
}

// Native R013/R015/R017/R019/R031/R033/R035 and R023-R028,
// identical at APIs 62, 65 and 67. The numeric Name is deliberately 15 chars.
func TestExecSOQLIdTextNativeControlsWithRejectionTwins(t *testing.T) {
	program, err := CompileAnonymous(`
Id primitive='001000000000001';
Id boxed=new Account(Id=primitive).Id;
insert new Account(Name='001000000000001');
insert new Account(Name='001000000000001AAA');
List<Id> wantedList=new List<Id>{boxed}; Set<Id> wantedSet=new Set<Id>{boxed};
System.assert( '1'.equals(String.valueOf(Database.query('SELECT Id FROM Account WHERE Name=:primitive').size())) );
System.assert( '1'.equals(String.valueOf(Database.query('SELECT Id FROM Account WHERE Name=:boxed').size())) );
System.assert( '1'.equals(String.valueOf(Database.query('SELECT Id FROM Account WHERE Name IN :wantedList').size())) );
System.assert( '1'.equals(String.valueOf(Database.query('SELECT Id FROM Account WHERE Name IN :wantedSet').size())) );
System.assert( '1'.equals(String.valueOf(Database.query('SELECT Id FROM Account WHERE Name LIKE :primitive').size())) );
System.assert( '1'.equals(String.valueOf(Database.query('SELECT Id FROM Account WHERE Name LIKE :boxed').size())) );
// R029/R031: Id text conversion does not use 15/18 Id equality for Name.
String shortName='001000000000001';
System.assert( '0'.equals(String.valueOf(Database.query('SELECT Id FROM Account WHERE Name=:shortName AND Name=:boxed').size())) );
System.assert( '0'.equals(String.valueOf([SELECT Id FROM Account WHERE Name=:shortName AND Name=:boxed].size())) );
Object scalar=boxed;
System.assert( '1'.equals(String.valueOf(Database.query('SELECT Id FROM Account WHERE Name=:scalar').size())) );
List<Object> objects=new List<Object>{boxed}; Set<Object> objectSet=new Set<Object>{boxed};
Map<String,Object> bindings=new Map<String,Object>{'wanted'=>objects};
Integer rejections=0;
try {Database.query('SELECT Id FROM Account WHERE Name IN :objects');System.assert(false);}
catch(QueryException e){System.assert('Invalid bind expression type of ANY for column of type String'.equals(e.getMessage()));rejections++;}
try {Database.query('SELECT Id FROM Account WHERE Name IN :objectSet');System.assert(false);}
catch(QueryException e){System.assert('Invalid bind expression type of ANY for column of type String'.equals(e.getMessage()));rejections++;}
try {Database.queryWithBinds('SELECT Id FROM Account WHERE Name IN :wanted',bindings,AccessLevel.USER_MODE);System.assert(false);}
catch(QueryException e){System.assert('Invalid bind expression type of ANY for column of type String'.equals(e.getMessage()));rejections++;}
Boolean invalid=true;
try {Database.query('SELECT Id FROM Account WHERE Name=:invalid');System.assert(false);}
catch(QueryException e){System.assert('Invalid bind expression type of Boolean for column of type String'.equals(e.getMessage()));rejections++;}
System.assert('4'.equals(String.valueOf(rejections)));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestSOQLIdLiteralKeepsIdColumnSemantics(t *testing.T) {
	// Native SOQL selection R093/R099/R100 remain on the Id-column lowering path.
	org := testDataOrg()
	machine := New(nil)
	machine.SetOrg(&org)
	primitive := String("001A00000000001")
	primitive.Type = "Id"
	query := "SELECT Id FROM Account WHERE Name=:wanted AND Id=:wanted"
	expanded, err := machine.expandSOQLBindsWith(query, func(string) (Value, error) { return primitive, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	expected := "SELECT Id FROM Account WHERE Name='" + displayIDText(primitive.Text) + "' AND Id=" + primitive.Text
	if expanded != expected {
		t.Fatalf("expected %q, got %q", expected, expanded)
	}
}
