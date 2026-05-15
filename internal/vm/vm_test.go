package vm

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

func TestCommonSObjectTypeNamesIncludesGeneratedStandardObjects(t *testing.T) {
	foundApexClass := false
	foundAccount := false
	for _, name := range CommonSObjectTypeNames() {
		if strings.EqualFold(name, "ApexClass") {
			foundApexClass = true
		}
		if strings.EqualFold(name, "Account") {
			foundAccount = true
		}
	}
	if !foundApexClass {
		t.Fatalf("CommonSObjectTypeNames should include generated standard object ApexClass")
	}
	if !foundAccount {
		t.Fatalf("CommonSObjectTypeNames should preserve prefix-backed standard object Account")
	}
}

func TestExecGeneratedPlatformStaticMethodFallsBackToTypedDefault(t *testing.T) {
	program, err := CompileAnonymous(`
List<Id> similar = Answers.findSimilar(new Account(Name = 'Acme'));
System.assertEquals(0, similar.size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedPlatformMethodReturnsPassiveValueObjectWithProperties(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.QuestionAndAnswersSuggestions suggestions =
	ConnectApi.QuestionAndAnswers.getSuggestions('0DB000000000001', 'reset password', '005000000000001', true, 10);
System.assertEquals(null, suggestions.articles);
suggestions.questions = 'placeholder';
System.assertEquals('placeholder', suggestions.questions);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecGeneratedPlatformConstructorInitializesPassiveProperties(t *testing.T) {
	program, err := CompileAnonymous(`
ConnectApi.QuestionAndAnswersCapabilityInput input =
	new ConnectApi.QuestionAndAnswersCapabilityInput(bestAnswerId = '0D5000000000001');
System.assertEquals('0D5000000000001', input.bestAnswerId);
System.assertEquals(null, input.questionTitle);
input.questionTitle = 'Solved';
System.assertEquals('Solved', input.questionTitle);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedPlatformFallbackSelectsTypeAwareOverload(t *testing.T) {
	original := generatedPlatformMethodIndex
	generatedPlatformMethodIndex = map[string]map[string][]Method{
		"generated.overload": {
			"pick": {
				{
					Name:       "Generated.Overload.pick",
					ClassName:  "Generated.Overload",
					ReturnType: "Integer",
					Params:     []Param{{Name: "value", Type: "Integer"}},
					IsStatic:   true,
				},
				{
					Name:       "Generated.Overload.pick",
					ClassName:  "Generated.Overload",
					ReturnType: "Boolean",
					Params:     []Param{{Name: "value", Type: "Boolean"}},
					IsStatic:   true,
				},
			},
		},
	}
	defer func() { generatedPlatformMethodIndex = original }()

	machine := New(nil)
	value, handled := machine.generatedPlatformStaticDefault("Generated.Overload.pick", []Value{Bool(true)})
	if !handled || value.Kind != ValueBool || value.Bool {
		t.Fatalf("Boolean overload default = %#v, handled %v; want false Boolean", value, handled)
	}

	value, handled = machine.generatedPlatformStaticDefault("Generated.Overload.pick", []Value{Int(1)})
	if !handled || value.Kind != ValueInt || value.Int != 0 {
		t.Fatalf("Integer overload default = %#v, handled %v; want zero Integer", value, handled)
	}
}

func TestGeneratedPlatformInstanceMethodFallsBackToTypedDefault(t *testing.T) {
	machine := New(nil)
	receiver := Object("ApexPages.IdeaStandardSetController")
	result := &Result{}

	value, handled, err := machine.callValueMember("controller", receiver, "getHasNext", nil, result)
	if err != nil {
		t.Fatal(err)
	}
	if !handled || value.Kind != ValueBool || value.Bool {
		t.Fatalf("getHasNext = %#v, handled %v; want false Boolean default", value, handled)
	}

	value, handled, err = machine.callValueMember("controller", receiver, "getResultSize", nil, result)
	if err != nil {
		t.Fatal(err)
	}
	if !handled || value.Kind != ValueInt || value.Int != 0 {
		t.Fatalf("getResultSize = %#v, handled %v; want zero Integer default", value, handled)
	}

	value, handled, err = machine.callValueMember("controller", receiver, "getListViewOptions", nil, result)
	if err != nil {
		t.Fatal(err)
	}
	if !handled || value.Kind != ValueList || value.Type != "List<SelectOption>" {
		t.Fatalf("getListViewOptions = %#v, handled %v; want typed List<SelectOption>", value, handled)
	}
}

func TestGeneratedPlatformFallbackDoesNotMaskExplicitUnsupportedRuntimeMethods(t *testing.T) {
	machine := New(nil)
	result := &Result{}

	_, handled, err := machine.callValueMember("page", newPageReference("/apex/example"), "getContent", nil, result)
	if !handled {
		t.Fatalf("PageReference.getContent was not handled")
	}
	if err == nil || !strings.Contains(err.Error(), "unsupported call") {
		t.Fatalf("PageReference.getContent error = %v, want explicit unsupported", err)
	}

	_, err = machine.call("Crypto.signXml", []Value{String("RSA"), Object("Dom.XmlNode"), String("id"), String("cert")}, nil, result)
	if err == nil || !strings.Contains(err.Error(), "unsupported call") {
		t.Fatalf("Crypto.signXml error = %v, want explicit unsupported", err)
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

func TestExecNumericLiteralSuffixes(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(1.1, 1.1d);
System.assertEquals(2.2, 2.2D);
System.assertEquals(3.3, 3.3f);
System.assertEquals(100.0, 1e2);
System.assertEquals(100.0, 1E2d);
System.assertEquals(10000000000, 10000000000L);
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
System.assertEquals(null, missing?.replace('a', 'b').replace('b', 'c'));
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

func TestExecStringComparisonOperators(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert('B' > 'A');
System.assert('A' < 'B');
System.assert('A' <= 'A');
System.assert('B' >= 'A');
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

func TestExecSObjectListAliasMutatesTypedList(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>{ new Account(Name = 'One') };
List<SObject> records = accounts;
records.add(new Account(Name = 'Two'));
System.assertEquals(2, accounts.size());
System.assertEquals('Two', accounts[1].get('Name'));
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

func TestExecSObjectMembersAreCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
System.assertEquals('Acme', account.GET('name'));
System.assertEquals(false, account.ISSET('Phone'));
account.PUT('phone', '1112223333');
System.assertEquals(true, account.ISSET('PHONE'));
System.assertEquals('1112223333', account.GET('Phone'));
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

func TestExecListSObjectTypePreservesConcreteRuntimeType(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>();
System.assertEquals(Account.SObjectType, accounts.getSObjectType());
List<SObject> records = accounts;
System.assertEquals(Account.SObjectType, records.getSObjectType());
records.add(new Account(Name = 'Acme'));
System.assertEquals(Account.SObjectType, records.getSObjectType());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecShiftOperatorsAndCompoundAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
Integer value = 3;
value <<= 2;
System.assertEquals(12, value);
value >>= 1;
System.assertEquals(6, value);
value %= 4;
System.assertEquals(2, value);
System.assertEquals(16, 1 << 4);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSwitchTypeCaseBindsSObject(t *testing.T) {
	program, err := CompileAnonymous(`
SObject record = new Account(Name = 'Acme');
String name;
switch on record {
    when Contact contact {
        name = contact.LastName;
    }
    when Account account {
        name = account.Name;
    }
    when else {
        name = 'else';
    }
}
System.assertEquals('Acme', name);
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

func TestExecNamedEnumEquals(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(true, Schema.DisplayType.STRING.equals(Schema.DisplayType.STRING));
System.assertEquals(false, Schema.DisplayType.STRING.equals(Schema.DisplayType.DATE));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
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

func TestExecSchemaDisplayTypeReferenceComparesCaseInsensitively(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(Schema.DisplayType.Reference, Contact.AccountId.getDescribe().getType());
System.assert(Contact.AccountId.getDescribe().getType() == Schema.DisplayType.Reference);
System.assert(Contact.FirstName.getDescribe().getType() != Schema.DisplayType.Reference);
System.assertEquals('REFERENCE', Schema.DisplayType.Reference.name());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecUserInfoGetDefaultCurrency(t *testing.T) {
	program, err := CompileAnonymous(`System.assertEquals('USD', UserInfo.getDefaultCurrency());`)
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

func TestExecUserInfoOrganizationName(t *testing.T) {
	program, err := CompileAnonymous(`System.assertEquals('OAER Local Org', UserInfo.getOrganizationName());`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLUpdateRequiredFieldToNullFails(t *testing.T) {
	program, err := CompileAnonymous(`
Account acct = new Account(Name = 'Acme');
insert acct;
acct.Name = null;
try {
    update acct;
    System.assert(false, 'expected required field failure');
} catch (Exception e) {
    System.assert(e.getMessage().contains('REQUIRED_FIELD_MISSING'));
}
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	storage.EnsureStandardObjectFields(&account.Definition)
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecNestedEnumShortNameFromSiblingSubclass(t *testing.T) {
	methodProgram, err := CompileAnonymous(`
System.assertEquals(QConditionGroup.LogicalOperator.OR_x, LogicalOperator.OR_x);
return LogicalOperator.OR_x;
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`System.assertEquals(QConditionGroup.LogicalOperator.OR_x, QOrGroup.value());`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "QConditionGroup"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "QConditionGroup.LogicalOperator", EnumValues: []string{"AND_x", "OR_x"}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "QOrGroup", SuperClass: "QConditionGroup"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "QOrGroup.value", ClassName: "QOrGroup", ReturnType: "QConditionGroup.LogicalOperator", IsStatic: true, Program: methodProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecJSONDeserializeSchemaFieldSetMember(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.FieldSetMember member = (Schema.FieldSetMember)JSON.deserialize('{"fieldPath":"BillingStreet","label":"Billing Street","required":false,"dbRequired":false}', Schema.FieldSetMember.class);
System.assertEquals('BillingStreet', member.getFieldPath());
System.assertEquals('Billing Street', member.getLabel());
System.assertEquals(false, member.getRequired());
System.assertEquals(false, member.getDbRequired());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
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

func TestExecCastsSObjectListMapValuesToConcreteSObjectLists(t *testing.T) {
	program, err := CompileAnonymous(`
Id ownerId = '005000000000001AAA';
Account account = new Account(Id = '001000000000001AAA', OwnerId = ownerId, Name = 'Acme');
Map<Id, List<SObject>> raw = new Map<Id, List<SObject>>();
raw.put(ownerId, new List<SObject>{ account });
Map<Id, List<Account>> typed = (Map<Id, List<Account>>)raw;
System.assertNotEquals(null, typed.get(ownerId));
System.assertEquals('Acme', typed.get(ownerId)[0].Name);
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

func TestExecInsertedListSObjectFieldTokenValuesStayDistinct(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Schema.SObjectField> fields = Group.SObjectType.getDescribe().fields.getMap();
Schema.SObjectField idField = fields.get('Id');
Schema.SObjectField nameField = fields.get('Name');
List<Group> groups = new List<Group>{
  new Group(Name = 'MatcherGroup0', DeveloperName = 'MatcherGroup0', Type = 'Queue'),
  new Group(Name = 'MatcherGroup1', DeveloperName = 'MatcherGroup1', Type = 'Queue')
};
insert groups;
List<Group> rawGroups = new List<Group>{
  new Group(Name = 'ArrayZero', DeveloperName = 'MatcherGroupArray0', Type = 'Queue'),
  new Group(Name = 'ArrayOne', DeveloperName = 'MatcherGroupArray1', Type = 'Queue')
};
System.assertNotEquals(rawGroups[0].get(nameField), rawGroups[1].get(nameField));
SObject[] sobjectArray = rawGroups;
System.assertNotEquals(sobjectArray[0].get(nameField), sobjectArray[1].get(nameField));
System.assertNotEquals(sobjectArray.get(0).get(nameField), sobjectArray.get(1).get(nameField));
insert sobjectArray;
System.assertNotEquals(sobjectArray[0].Id, sobjectArray[1].Id);
System.assertNotEquals(sobjectArray[0].get(nameField), sobjectArray[1].get(nameField));
System.assertNotEquals(groups[0].Id, groups[1].Id);
System.assertNotEquals(groups[0].get(nameField), groups[1].get(nameField));
Map<Schema.SObjectField, Object> expected = new Map<Schema.SObjectField, Object>{
  idField => groups[0].Id,
  nameField => groups[0].get('Name')
};
List<Map<Schema.SObjectField, Object>> expectedList = new List<Map<Schema.SObjectField, Object>>{
  expected,
  new Map<Schema.SObjectField, Object>{
    idField => groups[1].Id,
    nameField => groups[1].get('Name')
  }
};
System.assertEquals(expected.get(idField), expectedList[0].get(idField));
System.assertNotEquals(expectedList[0].get(idField), expectedList[1].get(idField));
System.assertNotEquals(expected.get(idField), groups[1].get(idField));
System.assertNotEquals(expected.get(nameField), groups[1].get(nameField));
List<SObject> swapped = new List<SObject>{groups[1], groups[0]};
System.assertNotEquals(expected.get(idField), swapped[0].get(idField));
System.assertNotEquals(expected.get(nameField), swapped[0].get(nameField));
Boolean firstMatch = true;
for (Schema.SObjectField f : expectedList[0].keySet()) {
  if (swapped[0].get(f) != expectedList[0].get(f)) {
    firstMatch = false;
  }
}
System.assert(!firstMatch);
Boolean mismatch = false;
for (Schema.SObjectField f : expected.keySet()) {
  Object valueToMatch = expected.get(f);
  if (groups[1].get(f) != valueToMatch) {
    mismatch = true;
  }
}
System.assert(mismatch);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDMLListLiteralPreservesSObjectAliases(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account(Name = 'Acme');
Account alias = account;
insert new List<Account>{ account };
System.assertNotEquals(null, account.Id);
System.assertEquals(account.Id, alias.Id);
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

func TestExecStringValueOfSObjectFieldUsesFieldAPIName(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('Name', String.valueOf(Account.Name));
System.assertEquals('AccountNumber', String.valueOf(Account.AccountNumber));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecSObjectFieldTokenUsesStandardOverlayFallback(t *testing.T) {
	program, err := CompileAnonymous("System.assertEquals('FirstName', String.valueOf(Account.FirstName));")
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	definition := org.Objects["Account"].Definition
	storage.EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})
	if _, ok := definition.Fields["FirstName"]; !ok {
		t.Fatalf("standard Account overlay missing FirstName")
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStandardSObjectDescribeUsesGeneratedOverlayWithoutOrgObject(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(Account.SObjectType, Schema.SObjectType.account);
System.assertEquals('Account', Schema.SObjectType.account.getDescribe().getName());
System.assertEquals('AccountNumber', Account.accountnumber.getDescribe().getName());
System.assertEquals('AccountNumber', String.valueOf(Account.SObjectType.fields.accountnumber));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&storage.OrgState{Objects: map[string]storage.ObjectState{}})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStubSObjectDescribeUsesGeneratedOverlayWithoutOrgObject(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(ApexClass.SObjectType, Schema.SObjectType.apexclass);
System.assertEquals('ApexClass', Schema.SObjectType.apexclass.getDescribe().getName());
System.assertEquals('Name', ApexClass.name.getDescribe().getName());
System.assertEquals(Schema.DisplayType.String, ApexClass.Name.getDescribe().getType());
System.assertEquals('Name', String.valueOf(ApexClass.SObjectType.fields.name));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	machine.SetOrg(&storage.OrgState{Objects: map[string]storage.ObjectState{}})
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestStaticSObjectFieldDefaultsToFieldToken(t *testing.T) {
	machine := New(nil)
	if err := machine.RegisterClass(Class{
		Name: "PaymentLine__c",
		StaticFields: map[string]Field{
			"CreatedDate": {Name: "CreatedDate", Type: "Schema.SObjectField", Static: true},
		},
	}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
System.assertEquals('CreatedDate', PaymentLine__c.SObjectType.getDescribe().fields.getMap().get('CreatedDate').getDescribe().getName());
System.assertEquals('CreatedDate', PaymentLine__c.CreatedDate.getDescribe().getName());
System.assertEquals(Schema.DisplayType.Datetime, PaymentLine__c.CreatedDate.getDescribe().getType());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["PaymentLine__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "PaymentLine__c",
			Fields: map[string]storage.Field{
				"CreatedDate": {Type: storage.FieldDateTime},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if err := machine.ResetStatics(); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDescribeMapsMatchUnqualifiedNamespaceTokens(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.SObjectType objectType = Schema.getGlobalDescribe().get('Widget__c');
Map<String, Schema.SObjectField> fields = objectType.getDescribe().fields.getMap();
System.assert(fields.containsKey('Thing__c'));
System.assertEquals('pkg__Thing__c', fields.get('Thing__c').getDescribe().getName());

Schema.SObjectType standardType = Schema.getGlobalDescribe().get('pkg__Opportunity');
System.assertNotEquals(null, standardType);
Map<String, Schema.SObjectField> standardFields = standardType.getDescribe().fields.getMap();
System.assertNotEquals(null, standardFields.get('pkg__ContactId'));
System.assertEquals('ContactId', standardFields.get('pkg__ContactId').getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Namespace = "pkg"
	storage.EnsureStandardObject(&org, "Opportunity")
	org.Objects["pkg__Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName: "pkg__Widget__c",
			Fields: map[string]storage.Field{
				"pkg__Thing__c": {APIName: "pkg__Thing__c", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	machine := New(nil)
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

func TestExecFixedSizeArrayAllocation(t *testing.T) {
	program, err := CompileAnonymous(`
Id[] ids = new Id[2];
System.assertEquals(2, ids.size());
System.assertEquals(null, ids[0]);
ids[1] = '001000000000001AAA';
System.assertEquals('001000000000001AAA', ids[1]);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
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
System.assertEquals(Account.SObjectType, Schema.Account.SObjectType);
System.assertEquals(Schema.SObjectType.Account, Schema.Account.SObjectType);
System.assertEquals(Contact.LastName, Schema.Contact.SObjectType.fields.lastName);
System.assertEquals(Account.AccountNumber, Schema.Account.SObjectType.fields.AccountNumber);
System.assertEquals(Account.AccountNumber, Account.SObjectType.getDescribe().fields.getMap().get('AccountNumber'));
System.assertEquals(Account.AccountNumber, Account.SObjectType.getDescribe().fields.getMap().get('accountnumber'));
System.assertEquals(Account.SObjectType, Schema.Account.getSObjectType());
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

func TestExecDescribeSObjectRecordTypeMapsUseStableKeys(t *testing.T) {
	program, err := CompileAnonymous(`
Schema.DescribeSObjectResult describe = Account.SObjectType.getDescribe();
Map<String, Schema.RecordTypeInfo> byName = describe.getRecordTypeInfosByName();
Map<String, Schema.RecordTypeInfo> byDeveloperName = describe.getRecordTypeInfosByDeveloperName();
Map<Id, Schema.RecordTypeInfo> byId = describe.getRecordTypeInfosById();
Schema.RecordTypeInfo business = byName.get('Business Account');
System.assertEquals('Business', business.getDeveloperName());
System.assertEquals(business, byDeveloperName.get('Business'));
System.assertEquals(business, byDeveloperName.get('business'));
System.assertEquals(business, byId.get(business.getRecordTypeId()));
System.assert(byName.keySet().contains('Business Account'));
System.assert(byId.containsKey(business.getRecordTypeId()));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.RecordTypes = []storage.RecordTypeInfo{
		{
			ID:            "012000000000001AAA",
			DeveloperName: "Business",
			Name:          "Business Account",
			Active:        true,
			Available:     true,
			Default:       true,
		},
		{
			ID:            "012000000000002AAA",
			DeveloperName: "Household",
			Name:          "Household Account",
			Active:        true,
			Available:     true,
		},
	}
	org.Objects["Account"] = account
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

func TestExecSObjectMapGetSObjectType(t *testing.T) {
	program, err := CompileAnonymous(`
Map<Id, Account> accounts = new Map<Id, Account>();
System.assertEquals(Account.SObjectType, accounts.getSObjectType());

Map<Id, SObject> assignedGeneric = accounts;
System.assertEquals(Account.SObjectType, assignedGeneric.getSObjectType());

Map<Id, SObject> genericRecords = new Map<Id, SObject>();
try {
  genericRecords.getSObjectType();
  System.assert(false, 'expected TypeException');
} catch (System.TypeException e) {
  System.assert(true);
}
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
Id typedId = '001000000000001AAA';
System.assert(longValue instanceof Long);
System.assert(!(longValue instanceof Integer));
System.assert(integerValue instanceof Integer);
System.assert(dateValue instanceof Datetime);
System.assert(idString instanceof Id);
System.assert(typedId instanceof String);
System.assert(!('bob' instanceof Id));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecStringEqualityOperatorIsCaseInsensitive(t *testing.T) {
	program, err := CompileAnonymous(`
System.assert('Prepayment refunds are not allowed.' == 'Prepayment Refunds are not allowed.');
System.assert(!('Prepayment refunds are not allowed.' != 'Prepayment Refunds are not allowed.'));
System.assert(new List<String>{'A'}.contains('A'));
System.assert(!new List<String>{'A'}.contains('a'));
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecDecimalArithmeticSuppressesBinaryFloatNoise(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal balance = 100 - 91.63;
System.assertEquals(8.37, balance);
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

func TestExecMultiplicativeCompoundFieldAssignment(t *testing.T) {
	program, err := CompileAnonymous(`
Account account = new Account();
account.AnnualRevenue = 10;
account.AnnualRevenue *= 2;
System.assertEquals(20, account.AnnualRevenue);
account.AnnualRevenue /= 4;
System.assertEquals(5, account.AnnualRevenue);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecIncrementDottedFieldTarget(t *testing.T) {
	program, err := CompileAnonymous(`
Map<String, Account> accounts = new Map<String, Account>{
	'one' => new Account(AnnualRevenue = 2)
};
accounts.get('one').AnnualRevenue++;
System.assertEquals(3, accounts.get('one').AnnualRevenue);
accounts.get('one').AnnualRevenue--;
System.assertEquals(2, accounts.get('one').AnnualRevenue);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecChainedIndexedFieldAssignmentExpression(t *testing.T) {
	program, err := CompileAnonymous(`
List<Account> accounts = new List<Account>{ new Account(), new Account() };
accounts[0].Fax = accounts[1].Fax = '1112223333';
System.assertEquals('1112223333', accounts[0].Fax);
System.assertEquals('1112223333', accounts[1].Fax);
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

func TestExpandSOQLBindsKeepsBooleanAndNullLiterals(t *testing.T) {
	machine := New(nil)
	got, err := machine.expandSOQLBindsWith(
		"SELECT Id FROM Account WHERE TaxExempt__c = : true AND IsDeleted = :false AND ParentId = :NULL AND Id IN :ids",
		func(name string) (Value, error) {
			if name == "ids" {
				return Value{Kind: ValueList, List: []Value{{Kind: ValueString, Text: "001000000000001"}}}, nil
			}
			return Null, errors.New("unexpected lookup")
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT Id FROM Account WHERE TaxExempt__c = true AND IsDeleted = false AND ParentId = null AND Id IN ('001000000000001')"
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestExpandSOQLBindsEvaluatesIndexedMemberExpression(t *testing.T) {
	machine := New(nil)
	first := Object("Account")
	first.Fields["Id"] = platformScalar("Id", "001000000000001AAA")
	second := Object("Account")
	second.Fields["Id"] = platformScalar("Id", "001000000000002AAA")
	machine.Globals["accounts"] = List(first, second)
	got, err := machine.expandSOQLBinds("SELECT Id FROM Account WHERE Id = : accounts [ 1 ] . Id")
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT Id FROM Account WHERE Id = '001000000000002AAA'"
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestExpandSOQLBindsEvaluatesChainedStaticCallExpression(t *testing.T) {
	machine := New(nil)
	machine.fakeNow = time.Date(2026, 5, 13, 10, 30, 0, 0, time.UTC)
	got, err := machine.expandSOQLBinds("SELECT Id FROM FlowdownQueue__c WHERE LastModifiedDate <= :DateTime.now().addDays(-1)")
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT Id FROM FlowdownQueue__c WHERE LastModifiedDate <= 2026-05-12T10:30:00Z"
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
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

func TestExecUnaryPlusNoOpForStringConcatenation(t *testing.T) {
	program, err := CompileAnonymous(`
String value = 'a';
value += + 'b';
System.assertEquals('ab', value);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecMultipleVariableDeclarationStatement(t *testing.T) {
	program, err := CompileAnonymous(`
Integer left = 1, right = 2;
System.assertEquals(3, left + right);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecExplicitDecimalToIntegerCast(t *testing.T) {
	program, err := CompileAnonymous(`
Decimal countValue = 5;
Integer existingCount = (Integer)countValue;
System.assertEquals(5, existingCount);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecAssertEqualsMatchesFifteenAndEighteenCharacterIDs(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('001000000000001', '001000000000001AAA');
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
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

func TestExecEnhancedForNullCollectionIsCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> names;
Boolean caught = false;
try {
	for (String name : names) {
		System.debug(name);
	}
} catch (NullPointerException e) {
	caught = true;
}
System.assertEquals(true, caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(program, nil); err != nil {
		t.Fatal(err)
	}
}

func TestExecListIndexOutOfBoundsIsCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
List<String> names = new List<String>();
Boolean caught = false;
try {
	names.remove(-1);
} catch (ListException e) {
	caught = true;
}
System.assertEquals(true, caught);
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
Object mode = VerificationMode.ModeName.CALLS;
System.assertEquals('calls', String.valueOf(mode));
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "VerificationMode"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(Class{Name: "VerificationMode.ModeName", EnumValues: []string{"times", "calls"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestExecEnumValueOfInvalidValueIsCatchable(t *testing.T) {
	program, err := CompileAnonymous(`
Boolean caught = false;
try {
	Object mode = VerificationMode.ModeName.valueOf('missing');
} catch (Exception e) {
	caught = true;
	System.assertEquals('System.IllegalArgumentException', e.getTypeName());
	System.assert(e.getMessage().contains('No enum constant VerificationMode.ModeName.missing'));
}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "VerificationMode.ModeName", EnumValues: []string{"times", "calls"}}); err != nil {
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
