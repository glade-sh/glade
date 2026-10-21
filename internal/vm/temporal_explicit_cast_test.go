package vm

import "testing"

// The owned datetime_date_cast row at API 62/67/68 records this direct cast's exact
// TypeException. Boxed and System-qualified spellings remain local controls.
func TestExecDatetimeDateCastCatchPath(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for name, cast := range map[string]string{
			"direct": "Date discarded = (Date)Datetime.newInstanceGmt(2040, 1, 2, 3, 4, 5);",
			"boxed":  "Object boxed = Datetime.newInstanceGmt(2040, 1, 2, 3, 4, 5); Date discarded = (Date)boxed;",
			"system": "dAtE discarded = (sYsTeM.dAtE)Datetime.newInstanceGmt(2040, 1, 2, 3, 4, 5);",
		} {
			assertNative := ""
			if name == "direct" {
				assertNative = `System.assertEquals('System.TypeException', failure.getTypeName());
System.assertEquals('Invalid conversion from runtime type Datetime to Date', failure.getMessage());`
			}
			for _, testContext := range []bool{false, true} {
				context := "anonymous"
				if testContext {
					context = "test"
				}
				t.Run(api+"/"+name+"/"+context, func(t *testing.T) {
					program, err := CompileAnonymousWithOptions(`
try { `+cast+` }
catch (Exception failure) { `+assertNative+` insert new Account(Name = 'synthetic-caught'); }
Account found = [SELECT Id FROM Account WHERE Name = 'synthetic-caught'];
System.assertNotEquals(null, found.Id);
`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					machine := New(nil)
					if testContext {
						machine.EnableTestContext()
					}
					org := testDataOrg()
					machine.SetOrg(&org)
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

// These are controls for existing local conversion behavior, not new native
// parity assertions. Only non-null Datetime-to-Date explicit casts are changed.
func TestTemporalExplicitCastPreservesOtherConversions(t *testing.T) {
	machine := New(nil)
	stamp := platformScalar("Datetime", "2040-01-02 03:04:05")
	day := platformScalar("Date", "2040-01-02")
	for _, tc := range []struct {
		name, target, wantType, wantText string
		value                            Value
		assignment                       bool
	}{
		{"datetime-date-assignment", "Date", "Date", "2040-01-02", stamp, true},
		{"date-datetime-assignment", "Datetime", "Datetime", "2040-01-02T00:00:00Z", day, true},
		{"date-datetime-cast", "Datetime", "Datetime", "2040-01-02T00:00:00Z", day, false},
		{"date-identity", "Date", "Date", "2040-01-02", day, false},
		{"datetime-identity", "Datetime", "Datetime", "2040-01-02 03:04:05", stamp, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got Value
			var err error
			if tc.assignment {
				got, err = machine.coerceAssignable(tc.target, tc.value)
			} else {
				got, err = machine.coerceCast(tc.target, tc.value)
			}
			if err != nil {
				t.Fatal(err)
			}
			text, err := platformScalarText(got, tc.wantType)
			if err != nil || text != tc.wantText {
				t.Fatalf("conversion = %#v, %v; want %s %q", got, err, tc.wantType, tc.wantText)
			}
		})
	}
	for name, value := range map[string]Value{
		"untyped": Null,
		"typed":   typedNull("Datetime"),
		"boxed":   {Kind: ValueNull, Type: "Datetime", Static: "Object"},
	} {
		t.Run(name+"-null", func(t *testing.T) {
			got, err := machine.coerceCast("Date", value)
			if err != nil || got.Kind != ValueNull {
				t.Fatalf("null cast = %#v, %v", got, err)
			}
		})
	}
}
