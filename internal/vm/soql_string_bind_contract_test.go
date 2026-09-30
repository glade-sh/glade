package vm

import "testing"

func TestExecSOQLStringBindEscapesPreserveValues(t *testing.T) {
	program, err := CompileAnonymous(`
String windowsPath = 'C:\\Temp';
String unknownEscape = 'bad\\q';
insert new Account(Name = windowsPath);
insert new Account(Name = unknownEscape);

List<Account> pathRows = Database.query('SELECT Id, Name FROM Account WHERE Name = :windowsPath');
System.assertEquals(1, pathRows.size());
System.assertEquals(windowsPath, pathRows[0].Name);
List<Account> unknownEscapeRows = Database.query('SELECT Id FROM Account WHERE Name = :unknownEscape');
System.assertEquals(1, unknownEscapeRows.size());

List<String> names = new List<String>{windowsPath, unknownEscape};
List<Account> listRows = Database.query('SELECT Id FROM Account WHERE Name IN :names');
System.assertEquals(2, listRows.size());
Set<String> nameSet = new Set<String>{windowsPath, unknownEscape};
List<Account> setRows = Database.query('SELECT Id FROM Account WHERE Name IN :nameSet');
System.assertEquals(2, setRows.size());

Map<String,Object> binds = new Map<String,Object>();
binds.put('wanted', windowsPath);
List<Account> boxedRows = Database.queryWithBinds('SELECT Id, Name FROM Account WHERE Name = :wanted', binds, AccessLevel.SYSTEM_MODE);
System.assertEquals(1, boxedRows.size());
System.assertEquals(windowsPath, boxedRows[0].Name);
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

func TestExecSOSLReturningWhereStringBindEscapesPreserveValues(t *testing.T) {
	for _, tt := range []struct {
		name       string
		bindSource string
	}{
		{name: "string bind", bindSource: "String pathBind = windowsPath;"},
		{name: "boxed string bind", bindSource: "Object pathBind = (Object) windowsPath;"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(`
String windowsPath = 'C:\\Temp';
Account pathAccount = new Account(Name = windowsPath);
insert pathAccount;
Test.setFixedSearchResults(new List<Id>{pathAccount.Id});
` + tt.bindSource + `
List<List<SObject>> rows = Search.query('FIND :pathBind IN ALL FIELDS RETURNING Account(Id, Name WHERE Name = :pathBind)');
System.assertEquals(1, rows[0].size());
System.assertEquals(windowsPath, rows[0][0].Name);
`)
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSOSLLiteralUsesSOSLStringEscapesForValueShapes(t *testing.T) {
	const raw = `C:\Temp`
	const want = `'C:\\Temp'`
	boxed := Object("String")
	boxed.Fields["value"] = String(raw)
	for _, tt := range []struct {
		name  string
		value Value
		want  string
	}{
		{name: "scalar", value: String(raw), want: want},
		{name: "boxed string", value: boxed, want: want},
		{name: "list", value: List(String(raw)), want: "(" + want + ")"},
		{name: "set", value: Set(String(raw)), want: "(" + want + ")"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := soslLiteral(tt.value); got != tt.want {
				t.Fatalf("soslLiteral() = %q, want %q", got, tt.want)
			}
		})
	}

	machine := New(nil)
	_, query, err := machine.parseSOSLQuery("FIND " + soslLiteral(String(raw)) + " RETURNING Account(Id)")
	if err != nil {
		t.Fatalf("SOSL parser rejected an escaped bound search term: %v", err)
	}
	if len(query.Terms) != 1 || query.Terms[0].Text != raw {
		t.Fatalf("SOSL search terms = %#v, want %q", query.Terms, raw)
	}
}

func TestSOQLLiteralEncodesBackslashesAcrossValueShapes(t *testing.T) {
	const raw = `C:\Temp`
	const want = `'C:\u005CTemp'`
	boxed := Object("String")
	boxed.Fields["value"] = String(raw)
	for _, tt := range []struct {
		name  string
		value Value
		want  string
	}{
		{name: "scalar", value: String(raw), want: want},
		{name: "boxed string", value: boxed, want: want},
		{name: "list", value: List(String(raw)), want: "(" + want + ")"},
		{name: "set", value: Set(String(raw)), want: "(" + want + ")"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := soqlLiteral(tt.value); got != tt.want {
				t.Fatalf("soqlLiteral() = %q, want %q", got, tt.want)
			}
		})
	}
}
