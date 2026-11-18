package vm

import (
	"strings"
	"testing"
)

func runDynamicBindProgram(t *testing.T, source string) error {
	t.Helper()
	program, err := CompileAnonymous(source)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	machine.EnableTestContext()
	_, err = machine.Execute(program)
	return err
}

// Apex names are case-insensitive: an ended catch variable must not shadow a
// live local spelled differently, and a live catch variable is found by any
// spelling.
func TestDynamicBindExceptionPrecheckIsCaseInsensitive(t *testing.T) {
	const invalid = "Invalid bind expression type of System.DmlException"
	cases := []struct {
		name, body string
		wantErr    bool
	}{
		{"ended catch var shadowed by live local", "try { throw new DmlException('x'); } catch (DmlException ex) {} String EX = 'x'; Integer n = Database.countQuery('SELECT COUNT() FROM Account WHERE Name = :ex');", false},
		{"live catch var other case count", "try { throw new DmlException('x'); } catch (DmlException ex) { Integer n = Database.countQuery('SELECT COUNT() FROM Account WHERE Name = :EX'); }", true},
		{"live catch var other case locator", "try { throw new DmlException('x'); } catch (DmlException ex) { Database.QueryLocator q = Database.getQueryLocator('SELECT Id FROM Account WHERE Name = :EX'); }", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := runDynamicBindProgram(t, c.body)
			if c.wantErr != (err != nil && strings.Contains(err.Error(), invalid)) {
				t.Fatalf("wantErr=%v err=%v", c.wantErr, err)
			}
		})
	}
}

// Validation and expansion select the same bindMap key for a dotted bind.
func TestQueryWithBindsDottedKeyValidationMatchesExpansion(t *testing.T) {
	const query = "Database.queryWithBinds('SELECT Id FROM Account WHERE Name = :acc.Name', m, AccessLevel.SYSTEM_MODE);"
	head := "Account acc = new Account(Name='a'); Map<String,Object> m = new Map<String,Object>();"
	if err := runDynamicBindProgram(t, head+"m.put('acc.Name', 123); m.put('Name', 'x'); List<Account> r = "+query); err != nil {
		t.Fatalf("member key holds a String: %v", err)
	}
	err := runDynamicBindProgram(t, head+"m.put('acc.Name', 'x'); m.put('Name', 123); List<Account> r = "+query)
	if err == nil || !strings.Contains(err.Error(), "Invalid bind expression type of Integer") {
		t.Fatalf("member key holds an Integer: %v", err)
	}
}
