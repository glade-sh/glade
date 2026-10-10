package vm

import "testing"

// Source contract: apex.behavior.rule.apex-soql-dotted-bind-accept.
// API 67 Object assignments must preserve COUNT()'s Integer result shape.
func TestExecInlineSOQLCountObjectAssignmentAPI67(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
insert new Account(Name = 'Count object probe');

Object boxedIntegerLiteral = 1;
System.assert(boxedIntegerLiteral instanceof Integer, 'integer literal control');

Object nonzeroCount = [SELECT COUNT() FROM Account WHERE Name = 'Count object probe'];
System.assert(nonzeroCount instanceof Integer, 'nonzero COUNT() assigned to Object must be Integer');
System.assertEquals(1, nonzeroCount);

Object zeroCount = [SELECT COUNT() FROM Account WHERE Name = 'no matching account'];
System.assert(zeroCount instanceof Integer, 'zero COUNT() assigned to Object must be Integer');
System.assertEquals(0, zeroCount);

Object countFieldRows = [SELECT COUNT(Id) FROM Account WHERE Name = 'Count object probe'];
System.assert(countFieldRows instanceof List<AggregateResult>, 'COUNT(Id) keeps AggregateResult shape');
List<AggregateResult> aggregates = (List<AggregateResult>)countFieldRows;
System.assertEquals(1, aggregates[0].get('expr0'));
`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if program.APIVersion != "67.0" {
		t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
	}

	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
