package vm

import (
	"fmt"
	"testing"
)

// Owned named observations at API 62/67 distinguish instance equals from ==
// and ordering. Epoch values and clock reads retain millisecond precision.
func TestDatetimeInstanceEqualsPrecision(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, delta := range []int{0, 1, 10, 999, 1000} {
			t.Run(fmt.Sprintf("%s/%d", api, delta), func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
Datetime whole=Datetime.newInstance(1735689600000L);
Datetime later=Datetime.newInstance(1735689600000L+%dL);
System.assertEquals(%t,whole.equals(later));
System.assertEquals(%t,whole==later);
System.assertEquals(%t,whole<later);
System.assertEquals(false,whole>later);
System.assertEquals(%dL,later.getTime()-whole.getTime());
`, delta, delta < 1000, delta == 0, delta > 0, delta), CompileOptions{APIVersion: api})
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
