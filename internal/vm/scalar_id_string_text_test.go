package vm

import "testing"

// Owned id_uuid_version I005/I011 capture String.valueOf(Id). A literal String
// remains a String even when its characters form a valid 15-character Id.
// These controls deliberately do not assert Id concatenation, Object(Id)
// formatting, or scheduled-job diagnostic text; those need independent native
// observations.
func TestScalarIdAndLiteralStringTextControls(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, input := range []struct{ short, long string }{
			{"001000000000001", "001000000000001AAA"},
			{"001B000001DVM9t", "001B000001DVM9tIAH"},
		} {
			t.Run(api+"/"+input.short, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
String literal = '`+input.short+`';
Id declared = literal;
Object dynamicString = literal;
String assigned = declared;
System.assert(literal.equals('`+input.short+`'));
System.assert(String.valueOf(literal).equals('`+input.short+`'));
System.assert(String.valueOf(dynamicString).equals('`+input.short+`'));
System.assert(('task_' + literal).equals('task_`+input.short+`'));
System.assert((literal + '_task').equals('`+input.short+`_task'));
System.assert(String.valueOf(declared).equals('`+input.long+`'));
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := New(nil).Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
