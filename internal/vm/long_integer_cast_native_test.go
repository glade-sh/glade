package vm

import "testing"

// Owned check-only and anonymous captures at APIs62/67 accept these casts and
// truncate to a signed low 32-bit word. Assignment without a cast is rejected.
func TestExplicitLongCastRetainsSignedIntegerWidth(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Long wide=4294967289L;
Integer narrow=(Integer)wide;
System.assertEquals(-7,narrow);
System.assertEquals((-2147483647-1),(Integer)2147483648L);
System.assertEquals(-1,(Integer)4294967295L);
System.assertEquals(7,(Integer)7L);
Long absent=null;
System.assertEquals(null,(Integer)absent);
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
