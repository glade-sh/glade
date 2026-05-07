package vm

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/storage"
	"github.com/open-aer/oaer/internal/trace"
)

func TestExecAssertEquals(t *testing.T) {
	program, err := CompileAnonymous("System.assertEquals(2, 1 + 1);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSystemAssertEqualsIsCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous("system.assertEquals(2, 1 + 1);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSystemAssertClassAliases(t *testing.T) {
	program, err := CompileAnonymous(`
System.Assert.areEqual(2, 1 + 1);
System.Assert.areNotEqual(3, 1 + 1);
System.Assert.isTrue(1 < 2);
System.Assert.isFalse(2 < 1);
System.Assert.isNull(null);
System.Assert.isNotNull('value');
SYSTEM.assert.AREEQUAL('trail', 'trail');
Assert.areEqual('short', 'short');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSystemAssertClassFailures(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "areEqual",
			source: "System.Assert.areEqual('left', 'right', 'mismatch');",
			want:   "expected <left>, actual <right>: mismatch",
		},
		{
			name:   "areNotEqual",
			source: "System.Assert.areNotEqual('same', 'same', 'duplicate');",
			want:   "values should not be equal: <same>: duplicate",
		},
		{
			name:   "isFalse",
			source: "System.Assert.isFalse(true, 'truthy');",
			want:   "assertion failed: truthy",
		},
		{
			name:   "isNull",
			source: "System.Assert.isNull('value', 'not null');",
			want:   "expected null, actual <value>: not null",
		},
		{
			name:   "isNotNull",
			source: "System.Assert.isNotNull(null, 'missing');",
			want:   "value should not be null: missing",
		},
		{
			name:   "fail",
			source: "System.Assert.fail('forced');",
			want:   "assertion failed: forced",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(tt.source)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Execute(program, nil)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) {
				t.Fatalf("error type = %T, want *RuntimeError", err)
			}
			if runtimeErr.Type != "System.AssertException" || runtimeErr.Message != tt.want {
				t.Fatalf("runtime error = (%q, %q), want System.AssertException %q", runtimeErr.Type, runtimeErr.Message, tt.want)
			}
		})
	}
}

func TestExecKeywordsAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Object value = NULL;
System.assertEquals(null, value);
Object built = NEW Account(Name = 'Acme');
System.assert(built != NULL);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNumericEqualityCrossIntegerDecimal(t *testing.T) {
	program, err := CompileAnonymous(`
Object integerValue = 100;
Object decimalValue = 100.0;
System.assert(integerValue == decimalValue);
System.assert(!(integerValue != decimalValue));
System.assertEquals(integerValue, decimalValue);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserTypesAndMembersAreCaseInsensitive(t *testing.T) {
	echo, err := CompileAnonymous("return 'ok';")
	if err != nil {
		t.Fatal(err)
	}
	shout, err := CompileAnonymous("return value;")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
caseprobe.Count = 2;
System.assertEquals(2, CASEPROBE.count);
CaseProbe p = new caseprobe();
p.name = 'Ada';
System.assertEquals('Ada', p.NAME);
System.assertEquals('ok', caseprobe.echo());
System.assertEquals('go', p.SHOUT('go'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "CaseProbe",
		Fields: map[string]Field{
			"Name": {Name: "Name", Type: "String"},
		},
		StaticFields: map[string]Field{
			"Count": {Name: "Count", Type: "Integer"},
		},
		Methods: map[string]Method{
			"echo":  {Name: "CaseProbe.echo", ClassName: "CaseProbe", IsStatic: true, ReturnType: "String", Program: echo},
			"shout": {Name: "CaseProbe.shout", ClassName: "CaseProbe", ReturnType: "String", Params: []Param{{Name: "value", Type: "String"}}, Program: shout},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecPlatformMembersAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> values = new List<Integer>();
values.ADD(1);
System.assertEquals(1, values.SIZE());
try {
	throw new DmlException('blocked');
} catch (Exception e) {
	System.assertEquals('blocked', e.getMEssage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecuteStopsWhenContextCanceled(t *testing.T) {
	program, err := CompileAnonymous("System.assert(true);")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	machine := New(nil)
	machine.SetContext(ctx)
	if _, err := machine.Execute(program); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func traceHas(events []trace.Event, name, category string) bool {
	for _, event := range events {
		if event.Name == name && event.Category == category {
			return true
		}
	}
	return false
}

func TestExecVariablesAndDebug(t *testing.T) {
	program, err := CompileAnonymous(`
Integer x = 1 + 1;
x = x * 3;
System.debug('x=' + x);
System.assertEquals(6, x);
`)
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	result, err := Execute(program, &stdout)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "x=6" {
		t.Fatalf("stdout = %q", got)
	}
	if len(result.Debug) != 1 || result.Debug[0] != "x=6" {
		t.Fatalf("debug = %#v", result.Debug)
	}
}

func TestCompileSkipsCommentsAndSafeNavigation(t *testing.T) {
	program, err := CompileAnonymous(`
String value = 'trail';
// A line comment should not become divide tokens.
/* Nor should a block comment. */
System.assertEquals('trail', value?.toString());
String missing = null;
System.assertEquals(null, missing?.toString());
System.assertEquals(null, missing?.length());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileStringEndingWithEscapedQuote(t *testing.T) {
	program, err := CompileAnonymous(`String value = 'BYELARUS\''; System.assertEquals('BYELARUS''', value);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsLongLiteralSuffix(t *testing.T) {
	program, err := CompileAnonymous(`System.assertEquals(9, 9L);`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsFinalLocalDeclaration(t *testing.T) {
	program, err := CompileAnonymous(`
final Account insertedOpp = new Account(Name = 'Original');
System.assertEquals('Original', insertedOpp.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsApexCastSyntax(t *testing.T) {
	program, err := CompileAnonymous(`
Object raw = 'trail';
String value = (String)raw;
System.assertEquals('trail', value);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsListIndexSyntax(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> values = new List<String>{'spruce', 'birch'};
System.assertEquals('spruce', values[0]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsTernarySyntax(t *testing.T) {
	program, err := CompileAnonymous(`
String value = true ? 'spruce' : null.toString();
System.assertEquals('spruce', value);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecNullCoalescingSyntax(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Integer> values = new Map<String, Integer>();
values.put('spruce', (values.get('spruce') ?? 0) + 1);
System.assertEquals(1, values.get('spruce'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsPrefixIncrementInForUpdate(t *testing.T) {
	program, err := CompileAnonymous(`
Integer total = 0;
for (Integer i = 0; i < 3; ++i) {
	total += i;
}
System.assertEquals(3, total);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecForInitializerWithMultipleVariableDeclarations(t *testing.T) {
	program, err := CompileAnonymous(`
Integer total = 0;
for (Integer i = 0, j = 3; i < j; i++) {
	total += i;
}
System.assertEquals(3, total);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecForUpdateWithMultipleExpressions(t *testing.T) {
	program, err := CompileAnonymous(`
Integer total = 0;
for (Integer i = 0, j = 3; i < j; i++, j--) {
	total += i + j;
}
System.assertEquals(6, total);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecTripleEqualsIdentityOperators(t *testing.T) {
	program, err := CompileAnonymous(`
Object left = new Account(Name = 'Acme');
Object same = left;
Object right = new Account(Name = 'Acme');
System.assert(left === same);
System.assert(left !== right);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecObjectEqualityUsesEqualsOverride(t *testing.T) {
	equalsProgram, err := CompileAnonymous(`
Probe that = (Probe) other;
return this.Key == that.Key;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Probe left = new Probe();
left.Key = 'one';
Probe sameValue = new Probe();
sameValue.Key = 'one';
Probe differentValue = new Probe();
differentValue.Key = 'two';
System.assert(left == sameValue);
System.assert(left != differentValue);
System.assert(left !== sameValue);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:   "Probe",
		Fields: map[string]Field{"Key": {Name: "Key", Type: "String"}},
		Methods: map[string]Method{
			"equals": {
				Name:       "Probe.equals",
				ClassName:  "Probe",
				ReturnType: "Boolean",
				Params:     []Param{{Name: "other", Type: "Object"}},
				Program:    equalsProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringConcatenationUsesToStringOverride(t *testing.T) {
	toStringProgram, err := CompileAnonymous("return 'Probe(' + this.Key + ')';")
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Probe probe = new Probe();
probe.Key = 'one';
System.assertEquals('value=Probe(one).', 'value=' + probe + '.');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:   "Probe",
		Fields: map[string]Field{"Key": {Name: "Key", Type: "String"}},
		Methods: map[string]Method{
			"toString": {
				Name:       "Probe.toString",
				ClassName:  "Probe",
				ReturnType: "String",
				Program:    toStringProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringConcatenationFormatsTypeLists(t *testing.T) {
	program, err := CompileAnonymous(`
List<Type> args = new List<Type>{String.class, Integer.class};
System.assertEquals('call(String, Integer)', 'call' + args);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecBareBlockStatement(t *testing.T) {
	program, err := CompileAnonymous(`
Integer total = 0;
{
  Integer local = 2;
  total += local;
}
System.assertEquals(2, total);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileBareConstructorExpressionStatement(t *testing.T) {
	program, err := CompileAnonymous(`new Account(Name = 'Acme');`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGroupIsSObjectLike(t *testing.T) {
	program, err := CompileAnonymous(`
SObject record = new Group(Name = 'Queue');
SObject[] records = new List<Group>{new Group(Name = 'Queue 1'), new Group(Name = 'Queue 2')};
System.assertEquals(2, records.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPostfixDecrementExpressionInListIndex(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> names = new List<String>{'Account', 'Contact', 'Case'};
Integer objectIdx = names.size() - 1;
System.assertEquals('Case', names[objectIdx--]);
System.assertEquals(1, objectIdx);
System.assertEquals('Contact', names[objectIdx--]);
System.assertEquals(0, objectIdx);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecPrefixDecrementExpressionInListIndex(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> names = new List<String>{'Account', 'Contact', 'Case'};
Integer objectIdx = names.size();
System.assertEquals('Case', names[--objectIdx]);
System.assertEquals(2, objectIdx);
System.assertEquals('Contact', names[--objectIdx]);
System.assertEquals(1, objectIdx);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecMapLiteralInitializer(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, String> params = new Map<String, String> { 'orderId' => '001000000000001AAA', 'L\'ANDORRE' => 'AD' };
System.assertEquals('001000000000001AAA', params.get('orderId'));
System.assertEquals('AD', params.get('L\'ANDORRE'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCompileAcceptsPostfixFieldAccess(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'spruce');
Object raw = account;
System.assertEquals('spruce', ((Account)raw).Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecLocalVariablesAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Id accountId = '001000000000001AAA';
System.assertEquals(accountId, accountid);
accountID = '001000000000002AAA';
System.assertEquals('001000000000002AAA', accountId);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecOverloadResolutionUsesCollectionElementType(t *testing.T) {
	listObjectProgram, err := CompileAnonymous(`return values.size();`)
	if err != nil {
		t.Fatal(err)
	}
	listStringProgram, err := CompileAnonymous(`return values.size() + 10;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<String> names = new List<String>{'spruce'};
System.assertEquals(11, Util.count(names));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<Object>"}}, Program: listObjectProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<String>"}}, Program: listStringProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecOverloadResolutionUsesTypedCollectionArgument(t *testing.T) {
	listProgram, err := CompileAnonymous(`return 'list';`)
	if err != nil {
		t.Fatal(err)
	}
	setProgram, err := CompileAnonymous(`return 'set';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<String> fields = new List<String>{'Name'};
System.assertEquals('list', QueryFactory.selectFields(fields));
System.assertEquals('list', QueryFactory.selectFields(new List<String>()));
System.assertEquals('set', QueryFactory.selectFields(new Set<String>{'Name'}));
System.assertEquals('set', QueryFactory.selectFields(new Set<String>()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for _, method := range []Method{
		{Name: "QueryFactory.selectFields", ReturnType: "String", Params: []Param{{Name: "fields", Type: "Set<String>"}}, Program: setProgram},
		{Name: "QueryFactory.selectFields", ReturnType: "String", Params: []Param{{Name: "fields", Type: "List<String>"}}, Program: listProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecOverloadResolutionUsesSchemaFieldCollectionElementType(t *testing.T) {
	stringSetProgram, err := CompileAnonymous(`return 'string set';`)
	if err != nil {
		t.Fatal(err)
	}
	stringListProgram, err := CompileAnonymous(`return 'string list';`)
	if err != nil {
		t.Fatal(err)
	}
	fieldSetProgram, err := CompileAnonymous(`return 'field set';`)
	if err != nil {
		t.Fatal(err)
	}
	fieldListProgram, err := CompileAnonymous(`return 'field list';`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fieldMap = Schema.SObjectType.Account.fields.getMap();
Set<String> stringSet = new Set<String>{'Name'};
List<String> stringList = new List<String>{'Name'};
Set<Schema.SObjectField> fieldSet = new Set<Schema.SObjectField>{fieldMap.get('Name')};
List<Schema.SObjectField> fieldList = new List<Schema.SObjectField>{fieldMap.get('Name')};

System.assertEquals('string set', QueryFactory.selectFields(stringSet));
System.assertEquals('string list', QueryFactory.selectFields(stringList));
System.assertEquals('field set', QueryFactory.selectFields(fieldSet));
System.assertEquals('field list', QueryFactory.selectFields(fieldList));
System.assertEquals('field set', QueryFactory.selectFields(new Set<Schema.SObjectField>()));
System.assertEquals('field list', QueryFactory.selectFields(new List<Schema.SObjectField>()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	for _, method := range []Method{
		{Name: "QueryFactory.selectFields", ReturnType: "String", Params: []Param{{Name: "fields", Type: "Set<String>"}}, Program: stringSetProgram},
		{Name: "QueryFactory.selectFields", ReturnType: "String", Params: []Param{{Name: "fields", Type: "List<String>"}}, Program: stringListProgram},
		{Name: "QueryFactory.selectFields", ReturnType: "String", Params: []Param{{Name: "fields", Type: "Set<Schema.SObjectField>"}}, Program: fieldSetProgram},
		{Name: "QueryFactory.selectFields", ReturnType: "String", Params: []Param{{Name: "fields", Type: "List<Schema.SObjectField>"}}, Program: fieldListProgram},
	} {
		if err := machine.RegisterMethod(method); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSchemaFieldTokensCompareByObjectAndField(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(Account.Name, Account.Name);
System.assert(Account.Name == Account.Name);
System.assert(Account.Name != Account.Id);
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

func TestExecMapSObjectFieldKeysRoundTripToSObjectGet(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
Schema.SObjectField idField = fields.get('Id');
Schema.SObjectField nameField = fields.get('Name');
Schema.SObjectField createdDateField = fields.get('CreatedDate');
System.assertNotEquals(null, createdDateField);
Account record = new Account(Id = '001000000000001AAA', Name = 'Acme');
Map<Schema.SObjectField, Object> expected = new Map<Schema.SObjectField, Object>{
  idField => record.Id,
  nameField => record.get('Name')
};
for (Schema.SObjectField fieldToken : expected.keySet()) {
  System.assertEquals(expected.get(fieldToken), record.get(fieldToken));
}
System.assert(record.get(createdDateField) != System.now());
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

func TestExecNestedListInitializerPreservesInnerList(t *testing.T) {
	program, err := CompileAnonymous(`
List<List<Contact>> nested = new List<List<Contact>>{ new List<Contact>{ new Contact(LastName = 'One'), new Contact(LastName = 'Two') } };
System.assertEquals(1, nested.size());
System.assertEquals(2, nested[0].size());
System.assertEquals('Two', nested[0][1].LastName);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "Contact",
			Fields:  map[string]storage.Field{"LastName": {APIName: "LastName", Type: storage.FieldString}},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectGetWrongFieldTokenIsCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectType> globalDescribe = Schema.getGlobalDescribe();
Schema.SObjectType accountType = globalDescribe.get('Account');
SObject inserted = accountType.newSObject();
inserted.put('Name', 'Acme');
insert inserted;
Id accountId = inserted.Id;
SObject queried = Database.query('SELECT Id, Name FROM Account WHERE Id = :accountId LIMIT 1');
System.assertNotEquals(null, queried.Id);
System.assertNotEquals(null, queried.get('Name'));
Map<String, Schema.SObjectField> accountFields = accountType.getDescribe().fields.getMap();
Map<Schema.SObjectField, Object> expected = new Map<Schema.SObjectField, Object>{
  accountFields.get('Id') => queried.Id,
  accountFields.get('Name') => queried.get('Name')
};
for (Schema.SObjectField fieldToken : expected.keySet()) {
  System.assertEquals(expected.get(fieldToken), queried.get(fieldToken));
}
Boolean unqueriedCaught = false;
try {
  queried.get(accountFields.get('CreatedDate'));
} catch (Exception e) {
  unqueriedCaught = true;
}
System.assert(unqueriedCaught);
Boolean caught = false;
try {
  globalDescribe.get('Opportunity').newSObject().get(accountFields.get('Id'));
} catch (Exception e) {
  caught = true;
}
System.assert(caught);
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

func TestExecDMLSObjectGetTreatsAuditFieldsAsUnqueried(t *testing.T) {
	program, err := CompileAnonymous(`
Account record = new Account(Name = 'Acme');
insert record;
System.assert(record.CreatedDate != null);
Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
System.assertEquals('Acme', record.get(fields.get('Name')));
Boolean caught = false;
try {
  record.get(fields.get('CreatedDate'));
} catch (Exception e) {
  caught = true;
}
System.assert(caught);
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

func TestExecSchemaSObjectTypeFieldsPathReturnsFieldToken(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(Contact.LastName, Schema.Contact.SObjectType.fields.lastName);
System.assertEquals(Account.AccountNumber, Schema.Account.SObjectType.fields.AccountNumber);
Boolean sawContacts = false;
for (Schema.ChildRelationship relationship : Account.SObjectType.getDescribe().getChildRelationships()) {
  if (relationship.getRelationshipName() == 'Contacts') {
    sawContacts = true;
  }
}
System.assert(sawContacts);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectListGetSObjectType(t *testing.T) {
	program, err := CompileAnonymous(`
List<SObject> records = new List<SObject>{ new Account(Name = 'Test') };
System.assertEquals(Account.SObjectType, records.getSObjectType());
List<Account> accounts = new List<Account>();
System.assertEquals(Account.SObjectType, accounts.getSObjectType());
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

func TestExecInstanceOfSObjectCollectionGenerics(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>{new Account(Name = 'Test')};
Object accountObject = accounts;
System.assert(accountObject instanceof List<SObject>, 'List<Account> should be List<SObject>');
System.assert(accountObject instanceOf List<Account>, 'List<Account> should be List<Account>');
System.assert(!(accountObject instanceof List<Contact>), 'List<Account> should not be List<Contact>');

List<AggregateResult> aggregateRows = new List<AggregateResult>();
Object aggregateObject = aggregateRows;
System.assert(aggregateObject instanceof List<SObject>, 'List<AggregateResult> should be List<SObject>');

List<String> names = new List<String>{'Test'};
Object namesObject = names;
System.assert(!(namesObject instanceof List<SObject>), 'List<String> should not be List<SObject>');
System.assert(namesObject instanceof List<Object>, 'List<String> should be List<Object>');

Map<String, Account> byName = new Map<String, Account>{'Test' => new Account(Name = 'Test')};
Object mapObject = byName;
System.assert(mapObject instanceof Map<String, SObject>, 'Map<String,Account> should be Map<String,SObject>');
System.assert(!(mapObject instanceof Map<Integer, SObject>), 'Map<String,Account> should not be Map<Integer,SObject>');
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

func TestExecInstanceOfHonorsNumericWidening(t *testing.T) {
	program, err := CompileAnonymous(`
Integer count = 3;
Long longer = 3L;
Decimal amount = 3.5;
System.assert(count instanceof Decimal);
System.assert(count instanceof Double);
System.assert(longer instanceof Decimal);
System.assert(amount instanceof Double);
System.assert(!(amount instanceof Integer));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecInstanceOfHonorsScalarRuntimeTypes(t *testing.T) {
	program, err := CompileAnonymous(`
Object longValue = 3L;
Object integerValue = 3;
Object dateValue = Date.today();
Object idString = '001000000000001AAA';
System.assert(longValue instanceof Long);
System.assert(!(longValue instanceof Integer));
System.assert(integerValue instanceof Integer);
System.assert(dateValue instanceof Datetime);
System.assert(idString instanceof Id);
System.assert(!('bob' instanceof Id));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNullableBooleanLogicalOperands(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean flag;
System.assertEquals(false, flag && true);
System.assertEquals(true, !flag);
Boolean other = true;
System.assertEquals(true, flag || other);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAssignmentExpressionInCallArgument(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, List<String>> values = new Map<String, List<String>>();
List<String> current = values.get('items');
if (current == null) {
  values.put('items', current = new List<String>());
}
current.add('one');
System.assertEquals(1, current.size());
System.assertEquals(1, values.get('items').size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecOverloadResolutionUsesTypedNullVariables(t *testing.T) {
	listObjectProgram, err := CompileAnonymous(`return 1;`)
	if err != nil {
		t.Fatal(err)
	}
	listIDProgram, err := CompileAnonymous(`return 2;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Id> ids = null;
System.assertEquals(2, Util.count(ids));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<Object>"}}, Program: listObjectProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<Id>"}}, Program: listIDProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLListAssignmentKeepsDeclaredType(t *testing.T) {
	listObjectProgram, err := CompileAnonymous(`return 1;`)
	if err != nil {
		t.Fatal(err)
	}
	listStringProgram, err := CompileAnonymous(`return 2;`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Account> accounts = [SELECT Id FROM Account LIMIT 1];
System.assertEquals(1, Util.count(accounts));
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account"}, Records: map[storage.ID]storage.Record{}}
	machine := New(nil)
	machine.Org = &org
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<Object>"}}, Program: listObjectProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Util.count", ReturnType: "Integer", Params: []Param{{Name: "values", Type: "List<String>"}}, Program: listStringProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLSingleSObjectAssignmentAndReturn(t *testing.T) {
	selectorProgram, err := CompileAnonymous(`return [SELECT Id, Name FROM Account LIMIT 1];`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
Account assigned;
assigned = [SELECT Id, Name, Custom__c FROM Account LIMIT 1];
System.assertEquals('Acme', assigned.Name);
System.assertEquals('selected', assigned.Custom__c);
Account returned = Selector.get();
System.assertEquals('Acme', returned.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := singleSOQLAssignmentOrg()
	machine.Org = &org
	if err := machine.RegisterClass(Class{Name: "Selector"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Selector.get", ReturnType: "Account", Program: selectorProgram, ClassName: "Selector", IsStatic: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSOQLSingleSObjectNoRowsAndMultiRowsStayExplicit(t *testing.T) {
	noRowsProgram, err := CompileAnonymous(`Account account; account = [SELECT Id FROM Account WHERE Name = 'Missing'];`)
	if err != nil {
		t.Fatal(err)
	}
	multiRowsProgram, err := CompileAnonymous(`Account account; account = [SELECT Id FROM Account];`)
	if err != nil {
		t.Fatal(err)
	}
	org := singleSOQLAssignmentOrg()
	machine := New(nil)
	machine.Org = &org
	if _, err := machine.Execute(noRowsProgram); err == nil || !strings.Contains(err.Error(), "List has no rows") {
		t.Fatalf("no-row err = %v, want List has no rows", err)
	}
	machine = New(nil)
	machine.Org = &org
	if _, err := machine.Execute(multiRowsProgram); err == nil || !strings.Contains(err.Error(), "List has more than 1 row") {
		t.Fatalf("multi-row err = %v, want List has more than 1 row", err)
	}
}

func TestExecSOQLSingleSObjectStaticAndDottedFieldAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
Constants.ORG = [SELECT Id, Name FROM Organization LIMIT 1];
Container c = new Container();
c.account = [SELECT Id, Name FROM Account LIMIT 1];
System.assertEquals('Acme', c.account.Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	multiRowsProgram, err := CompileAnonymous(`
Container c = new Container();
c.account = [SELECT Id FROM Account];
`)
	if err != nil {
		t.Fatal(err)
	}
	org := singleSOQLAssignmentOrg()
	machine := New(nil)
	machine.Org = &org
	registerAssignmentTargetClasses(t, machine)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	field := machine.Classes["Constants"].StaticFields["ORG"]
	if field.Value.Kind != ValueObject || field.Value.Type != "Organization" {
		t.Fatalf("ORG = %#v, want Organization object", field.Value)
	}
	machine = New(nil)
	machine.Org = &org
	registerAssignmentTargetClasses(t, machine)
	if _, err := machine.Execute(multiRowsProgram); err == nil || !strings.Contains(err.Error(), "List has more than 1 row") {
		t.Fatalf("multi-row dotted assignment err = %v, want List has more than 1 row", err)
	}
}

func TestExecImplicitThisSObjectFieldPathUsesInstanceField(t *testing.T) {
	program, err := CompileAnonymous(`return OrderRecord.Entity__c;`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := singleSOQLAssignmentOrg()
	machine.Org = &org
	if err := machine.RegisterClass(Class{Name: "Controller", Fields: map[string]Field{
		"OrderRecord": {Name: "OrderRecord", Type: "Order__c"},
	}}); err != nil {
		t.Fatal(err)
	}
	method := Method{Name: "Controller.entityId", ReturnType: "Id", Program: program, ClassName: "Controller"}
	receiver := Object("Controller")
	receiver.Fields["OrderRecord"] = Object("Order__c")
	receiver.Fields["OrderRecord"].Fields["Entity__c"] = platformScalar("Id", "a00000000000001")
	value, err := machine.callMethodWithReceiver(method, receiver, nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := platformScalarText(value, "Id"); text != "a00000000000001" {
		t.Fatalf("entity id = %#v", value)
	}
}

func TestExecSOQLSingleSObjectImplicitThisDottedFieldAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
c.account = [SELECT Id, Name FROM Account LIMIT 1];
return c.account.Name;
`)
	if err != nil {
		t.Fatal(err)
	}
	multiRowsProgram, err := CompileAnonymous(`c.account = [SELECT Id FROM Account];`)
	if err != nil {
		t.Fatal(err)
	}
	org := singleSOQLAssignmentOrg()
	machine := New(nil)
	machine.Org = &org
	registerAssignmentTargetClasses(t, machine)
	if err := machine.RegisterClass(Class{Name: "Controller", Fields: map[string]Field{
		"c": {Name: "c", Type: "Container"},
	}}); err != nil {
		t.Fatal(err)
	}
	receiver := Object("Controller")
	receiver.Fields["c"] = Object("Container")
	method := Method{Name: "Controller.assignAccount", ReturnType: "String", Program: program, ClassName: "Controller"}
	value, err := machine.callMethodWithReceiver(method, receiver, nil, &Result{})
	if err != nil {
		t.Fatal(err)
	}
	if value.Kind != ValueString || value.Text != "Acme" {
		t.Fatalf("assigned account name = %#v", value)
	}
	machine = New(nil)
	machine.Org = &org
	registerAssignmentTargetClasses(t, machine)
	if err := machine.RegisterClass(Class{Name: "Controller", Fields: map[string]Field{
		"c": {Name: "c", Type: "Container"},
	}}); err != nil {
		t.Fatal(err)
	}
	receiver = Object("Controller")
	receiver.Fields["c"] = Object("Container")
	method = Method{Name: "Controller.assignAccount", Program: multiRowsProgram, ClassName: "Controller"}
	if _, err := machine.callMethodWithReceiver(method, receiver, nil, &Result{}); err == nil || !strings.Contains(err.Error(), "List has more than 1 row") {
		t.Fatalf("multi-row implicit this assignment err = %v, want List has more than 1 row", err)
	}
}

func TestAssignmentTargetTypeWalksStorageReferenceFields(t *testing.T) {
	org := singleSOQLAssignmentOrg()
	org.Objects["Contact"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{
			"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}},
		}},
	}
	machine := New(nil)
	machine.Org = &org
	machine.Globals["contact"] = Object("Contact")
	machine.VarTypes["contact"] = "Contact"
	if got := machine.assignmentTargetType("contact.Account__c.Custom__c"); got != "String" {
		t.Fatalf("target type = %q, want String", got)
	}
}

func singleSOQLAssignmentOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{
			"Name":      {APIName: "Name", Type: storage.FieldString},
			"Custom__c": {APIName: "Custom__c", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{
				"Name":      storage.StringValue("Acme"),
				"Custom__c": storage.StringValue("selected"),
			}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{
				"Name": storage.StringValue("Other"),
			}},
		},
	}
	org.Objects["Organization"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Organization", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{
			"00D000000000001": {ID: "00D000000000001", Object: "Organization", Fields: map[string]storage.Value{"Name": storage.StringValue("Local")}},
		},
	}
	return org
}

func registerAssignmentTargetClasses(t *testing.T, machine *VM) {
	t.Helper()
	if err := machine.RegisterClass(Class{Name: "Constants", StaticFields: map[string]Field{
		"ORG": {Name: "ORG", Type: "Organization", Static: true},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "Container", Fields: map[string]Field{
		"account": {Name: "account", Type: "Account"},
	}}); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteIDsToSObjectsUsesIDPrefix(t *testing.T) {
	machine := New(nil)
	converted, ok := machine.deleteIDsToSObjects(List(platformScalar("Id", "001000000000001AAA")))
	if !ok {
		t.Fatal("delete id list was not converted")
	}
	if converted.Kind != ValueList || len(converted.List) != 1 {
		t.Fatalf("converted = %#v", converted)
	}
	if converted.List[0].Type != "Account" {
		t.Fatalf("type = %q, want Account", converted.List[0].Type)
	}
	if id, _ := platformScalarText(converted.List[0].Fields["Id"], "Id"); id != "001000000000001AAA" {
		t.Fatalf("id = %#v", id)
	}
}

func TestExecCoercesNumericVariablesAndCollections(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal total = 1;
total = 2;
System.assertEquals(2.5, total + 0.5);
List<Decimal> totals = new List<Decimal>();
totals.add(1);
System.assertEquals(1.25, totals.get(0) + 0.25);
Map<String,Decimal> byName = new Map<String,Decimal>();
byName.put('one', 1);
System.assertEquals(1.75, byName.get('one') + 0.75);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecRejectsInvalidCoercions(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"decimal to integer assignment", "Integer count = 1.5;"},
		{"string to boolean assignment", "Boolean ready = 'true';"},
		{"decimal list item", "List<Integer> counts = new List<Integer>(); counts.add(1.5);"},
		{"integer map key", "Map<String,Integer> counts = new Map<String,Integer>(); counts.put(1, 2);"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, err := CompileAnonymous(tt.body)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Execute(program, nil); err == nil {
				t.Fatalf("expected coercion error")
			}
		})
	}
}

func TestExecAssertFailure(t *testing.T) {
	program, err := CompileAnonymous("System.assertEquals(3, 1 + 1);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err == nil {
		t.Fatal("expected assertion failure")
	}
}

func TestExecRuntimeErrorStackUsesStatementSourcePosition(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(3, 1 + 1);
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("err = %#v", err)
	}
	if len(runtimeErr.Stack) == 0 || runtimeErr.Stack[0].Line != 2 || runtimeErr.Stack[0].Column != 1 {
		t.Fatalf("stack = %#v", runtimeErr.Stack)
	}
}

func TestExecNullDereferenceRuntimeErrorIncludesMemberContext(t *testing.T) {
	program, err := CompileAnonymous(`
String name = null;
name.toUpperCase();
`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Execute(program, nil)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("err = %#v, want RuntimeError", err)
	}
	if runtimeErr.Type != "NullPointerException" {
		t.Fatalf("runtime error type = %q", runtimeErr.Type)
	}
	for _, want := range []string{"Attempt to de-reference a null object", "name.toUpperCase", "null receiver name"} {
		if !strings.Contains(runtimeErr.Message, want) {
			t.Fatalf("runtime error message = %q, want %q", runtimeErr.Message, want)
		}
	}
}

func TestExecCollectionsAndTrace(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> xs = new List<Integer>{1, 2};
xs.add(3);
Set<String> names = new Set<String>();
names.add('a');
names.add('a');
Map<String,Integer> counts = new Map<String,Integer>();
counts.put('a', xs.size());
System.assertEquals(3, xs.get(2));
System.assertEquals(1, names.size());
System.assertEquals(3, counts.get('a'));
System.assert(counts.containsKey('a'));
`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Execute(program, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Trace) != len(program.Instructions)+1 {
		t.Fatalf("trace length = %d, want %d", len(result.Trace), len(program.Instructions)+1)
	}
	if result.TraceFormat != "chrome-trace-event" {
		t.Fatalf("trace format = %q", result.TraceFormat)
	}
	first := result.Trace[0]
	if first.Name != "apex.statement.declare" || first.Category != "apex.statement" || first.Phase != "i" {
		t.Fatalf("trace event shape = %#v", first)
	}
	if first.Args["sourceOffset"] == 0 {
		t.Fatalf("trace missing source offset: %#v", first)
	}
	if first.Args["line"] != 2 || first.Args["column"] != 1 {
		t.Fatalf("trace source position = %#v", first.Args)
	}
	last := result.Trace[len(result.Trace)-1]
	if last.Name != "apex.limits" || last.Category != "apex.limits" {
		t.Fatalf("trace missing limits summary: %#v", last)
	}
}

func TestExecForBreakContinueDoWhileSwitchAndEnhancedFor(t *testing.T) {
	program, err := CompileAnonymous(`
List<Integer> xs = new List<Integer>{1, 2, 3, 4};
Integer total = 0;
for (Integer i = 0; i < xs.size(); i++) {
	if (i == 1) {
		continue;
	}
	if (i == 3) {
		break;
	}
	total = total + xs.get(i);
}
Integer seen = 0;
for (Integer x : xs) {
	seen = seen + x;
}
Integer once = 0;
do {
	once++;
} while (once < 1);
String label = '';
switch on total {
	when 1 { label = 'one'; }
	when 4 { label = 'four'; }
	when else { label = 'other'; }
}
System.assertEquals(4, total);
System.assertEquals(10, seen);
System.assertEquals(1, once);
System.assertEquals('four', label);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecSwitchBreakOnlyExitsSwitchAndContinueReachesLoop(t *testing.T) {
	program, err := CompileAnonymous(`
Integer seen = 0;
Integer afterSwitch = 0;
for (Integer i = 0; i < 4; i++) {
	switch on i {
		when 0 {
			seen = seen + 1;
			break;
		}
		when 1 {
			continue;
		}
		when else {
			seen = seen + 10;
		}
	}
	afterSwitch++;
}
System.assertEquals(21, seen);
System.assertEquals(3, afterSwitch);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecFinallyPreservesAndOverridesLoopSignals(t *testing.T) {
	program, err := CompileAnonymous(`
Integer cleaned = 0;
Integer seen = 0;
for (Integer i = 0; i < 4; i++) {
	try {
		if (i == 1) {
			continue;
		}
		if (i == 2) {
			break;
		}
		seen++;
	} finally {
		cleaned++;
	}
}
System.assertEquals(1, seen);
System.assertEquals(3, cleaned);
Integer overridden = 0;
while (overridden < 1) {
	try {
		break;
	} finally {
		overridden++;
		continue;
	}
}
System.assertEquals(1, overridden);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecEnhancedForBreakContinueAndFinallyThrowOverride(t *testing.T) {
	throwingReturn, err := CompileAnonymous(`
try {
	return 7;
} finally {
	throw new DmlException('finally wins');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
List<Integer> values = new List<Integer>{1, 2, 3, 4};
Integer total = 0;
Integer cleaned = 0;
for (Integer value : values) {
	try {
		if (value == 2) {
			continue;
		}
		if (value == 4) {
			break;
		}
		total = total + value;
	} finally {
		cleaned++;
	}
}
String message = '';
try {
	Util.throwingReturn();
} catch (DmlException e) {
	message = e.getMessage();
}
System.assertEquals(4, total);
System.assertEquals(4, cleaned);
System.assertEquals('finally wins', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Util.throwingReturn", ReturnType: "Integer", Program: throwingReturn}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecTryCatchFinallyThrow(t *testing.T) {
	program, err := CompileAnonymous(`
Integer cleaned = 0;
try {
	throw new MyException();
} catch (Exception e) {
	cleaned = cleaned + 1;
} finally {
	cleaned = cleaned + 2;
}
System.assertEquals(3, cleaned);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecRegisteredCustomExceptionUsesMessageConstructor(t *testing.T) {
	program, err := CompileAnonymous(`
String message = '';
try {
	throw new NUException('boom');
} catch (Exception e) {
	message = e.getMessage();
}
System.assertEquals('boom', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "NUException", SuperClass: "Exception"}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecCustomExceptionConstructorCanSetMessage(t *testing.T) {
	ctor, err := CompileAnonymous(`this.setMessage(message);`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String message = '';
try {
	throw new NUException('blocked');
} catch (Exception e) {
	message = e.getMessage();
}
System.assertEquals('blocked', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name:       "NUException",
		SuperClass: "Exception",
		Constructors: []Method{{
			Name:          "NUException.<init>",
			ClassName:     "NUException",
			Params:        []Param{{Name: "message", Type: "String"}},
			Program:       ctor,
			IsConstructor: true,
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedEnumStaticValue(t *testing.T) {
	program, err := CompileAnonymous(`
Object direction = TriggerConstants.Direction.ToCustomer;
System.assertEquals('ToCustomer', String.valueOf(direction));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "TriggerConstants"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "TriggerConstants.Direction", EnumValues: []string{"ToPlatform", "ToCustomer", "Both"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedEnumStaticValueIsCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Object mode = fflib_VerificationMode.ModeName.CALLS;
System.assertEquals('calls', String.valueOf(mode));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "fflib_VerificationMode"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "fflib_VerificationMode.ModeName", EnumValues: []string{"times", "calls"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecChainedAssignmentExpression(t *testing.T) {
	program, err := CompileAnonymous(`
Integer left;
Integer right;
left = right = 3;
System.assertEquals(3, left);
System.assertEquals(3, right);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecMultiCatchAndRethrow(t *testing.T) {
	program, err := CompileAnonymous(`
String message = '';
try {
	try {
		throw new MyException('boom');
	} catch (Exception e) {
		throw;
	}
} catch (OtherException | MyException e) {
	message = e.getMessage();
}
System.assertEquals('boom', message);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecFinallyRunsOnReturnAndCanOverrideReturn(t *testing.T) {
	var stdout strings.Builder
	firstProgram, err := CompileAnonymous(`
try {
	return 1;
} finally {
	System.debug('clean');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	secondProgram, err := CompileAnonymous(`
try {
	return 1;
} finally {
	return 3;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals(1, Util.first());
System.assertEquals(3, Util.second());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(&stdout)
	if err := machine.RegisterMethod(Method{Name: "Util.first", ReturnType: "Integer", Program: firstProgram}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Util.second", ReturnType: "Integer", Program: secondProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "clean\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestExecFinallyRunsBeforeUncaughtThrow(t *testing.T) {
	var stdout strings.Builder
	throwProgram, err := CompileAnonymous(`
try {
	throw new MyException('boom');
} finally {
	System.debug('clean');
}
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
try {
	Util.thrower();
} catch (MyException e) {
	System.assertEquals('boom', e.getMessage());
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(&stdout)
	if err := machine.RegisterMethod(Method{Name: "Util.thrower", ReturnType: "void", Program: throwProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "clean\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestExecCatchInterfaceExceptionType(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = 'no';
try {
	throw new MyException();
} catch (Marker e) {
	caught = 'yes';
}
System.assertEquals('yes', caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "Marker"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "MyException", Interfaces: []string{"Marker"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecExceptionHierarchyMultipleCatchAndMetadata(t *testing.T) {
	program, err := CompileAnonymous(`
String caught = '';
try {
	throw new QueryException('bad query');
} catch (DmlException e) {
	caught = 'wrong';
} catch (System.QueryException e) {
	caught = e.getTypeName() + ':' + e.getMessage();
	System.assert(e.getLineNumber() > 0);
	String trace = e.getStackTraceString();
	System.assert(trace != '');
}
Exception base = new DmlException('blocked');
System.assertEquals('System.DmlException', base.getTypeName());
System.assertEquals('System.QueryException:bad query', caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecRethrowPreservesOriginalExceptionStack(t *testing.T) {
	throwProgram, err := CompileAnonymous(`
throw new DmlException('boom');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
String stack = '';
try {
	try {
		Util.thrower();
	} catch (Exception e) {
		throw;
	}
} catch (DmlException e) {
	stack = e.getStackTraceString();
}
System.assert(stack.contains('Util.thrower'));
System.assert(!stack.contains('rethrow outside catch block'));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterMethod(Method{Name: "Util.thrower", ReturnType: "void", Program: throwProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalArithmetic(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal total = 1.5 + 2;
System.assertEquals('3.5', '' + total);
System.assert(total > 3);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}
