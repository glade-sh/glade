package vm

import "testing"

func TestFrameworkListSObjectTypeDispatch(t *testing.T) {
	for _, populated := range []bool{false, true} {
		rows := List()
		if populated {
			rows = List(Object("Contact"))
		}
		rows.Type = "List<SObject>"
		rows.Runtime = "List<SObject>"
		machine := New(nil)
		value, handled, err := machine.callListValueMember("rows", rows, "getSObjectType", nil, &Result{})
		check := func(route string, value Value, handled bool, err error) {
			t.Helper()
			if err != nil || !handled {
				t.Fatalf("%s populated=%v: handled=%v err=%v", route, populated, handled, err)
			}
			if populated {
				if name := value.Fields["object"]; name.Kind != ValueString || name.Text != "Contact" {
					t.Fatalf("%s token=%#v", route, value)
				}
			} else if value.Kind != ValueNull {
				t.Fatalf("%s empty token=%#v", route, value)
			}
		}
		check("vm", value, handled, err)
		value, _, _, handled, err = callListStdlibMember(rows, "getSObjectType", nil)
		check("stdlib", value, handled, err)
	}
}
