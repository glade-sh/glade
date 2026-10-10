package vm

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

// Preserve the B21 fatal-error contract; A37's native readonly rows do not
// authorize making a trigger's governor-limit failure catchable.
func TestTriggerLimitExceptionRemainsUncatchable(t *testing.T) {
	helperProgram, err := CompileAnonymous(`insert new Account(Name = 'Nested fatal trigger');`)
	if err != nil {
		t.Fatal(err)
	}
	indirectProgram, err := CompileAnonymous(`TriggerLimitHelper.insertAccount();`)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []struct {
		name    string
		program ir.Program
	}{
		{"direct", helperProgram},
		{"method", indirectProgram},
	} {
		for _, timing := range []string{triggerTimingBefore, triggerTimingAfter} {
			for _, operation := range []struct{ name, source string }{
				{"atomic", "insert new Account(Name = 'Fatal trigger');"},
				{"partial", "Database.insert(new List<Account>{new Account(Name = 'Fatal trigger')}, false);"},
			} {
				for _, tracing := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%s/trace=%t", body.name, timing, operation.name, tracing), func(t *testing.T) {
						program, err := CompileAnonymous(`
Boolean caught = false;
Boolean continued = false;
try {
` + operation.source + `
} catch (DmlException e) {
    caught = true;
} catch (Exception e) {
    caught = true;
}
continued = true;
`)
						if err != nil {
							t.Fatal(err)
						}
						machine := New(nil)
						org := testDataOrg()
						machine.SetOrg(&org)
						machine.SetLimitMode(LimitModeStrict)
						caps := defaultLimitCaps()
						caps.DMLStatements = 1
						machine.SetLimitCaps(caps)
						machine.SetTraceEnabled(tracing)
						if err := machine.RegisterClass(Class{
							Name: "TriggerLimitHelper", Access: "public",
							Methods: map[string]Method{
								"insertAccount": {
									Name: "TriggerLimitHelper.insertAccount", ClassName: "TriggerLimitHelper",
									ReturnType: "void", IsStatic: true, Access: "public", Program: helperProgram,
								},
							},
						}); err != nil {
							t.Fatal(err)
						}
						if err := machine.RegisterTrigger(Trigger{
							Name: "AccountDMLLimit", Object: "Account", Timing: timing,
							Operation: "insert", Program: body.program,
						}); err != nil {
							t.Fatal(err)
						}
						_, err = machine.Execute(program)
						var limitErr *RuntimeError
						if !errors.As(err, &limitErr) || limitErr.Type != "System.LimitException" || limitErr.Message != "Too many dmlStatements: 2 out of 1" {
							t.Fatalf("error = %v, want original fatal DML LimitException", err)
						}
						for _, name := range []string{"caught", "continued"} {
							if value := machine.Globals[name]; value.Kind != ValueBool || value.Bool {
								t.Fatalf("%s = %#v, want false after fatal trigger failure", name, value)
							}
						}
					})
				}
			}
		}
	}
}
