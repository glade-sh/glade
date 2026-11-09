package vm

import (
	"strings"
	"testing"
)

// Native rows (fresh scratch org, API 62.0 and 67.0): a null Id variable, a
// method returning a null Id and UserInfo.getUserRoleId() with no role all
// pick byId(Id); a null List<SObject> variable picks byId(List<SObject>); a
// literal null is a compile-time ambiguity.
func TestOverloadStaticIdTypeForNullReturns(t *testing.T) {
	for _, tc := range []struct {
		name, call, want string
		ambiguous        bool
	}{
		{"null Id variable", "Id v = null; r = Q.byId(v);", "Id", false},
		{"method returning null Id", "r = Q.byId(Q.nullId());", "Id", false},
		{"UserInfo.getUserRoleId", "r = Q.byId(UserInfo.getUserRoleId());", "Id", false},
		{"null List<SObject> variable", "List<SObject> l = null; r = Q.byId(l);", "ListSObject", false},
		{"literal null", "r = Q.byId(null);", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			if err := machine.RegisterClass(Class{Name: "Q"}); err != nil {
				t.Fatal(err)
			}
			for _, spec := range []struct{ param, value string }{
				{"Id", "Id"}, {"Iterable<Id>", "Iterable"}, {"List<SObject>", "ListSObject"},
			} {
				body, err := CompileAnonymous("return '" + spec.value + "';")
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterMethod(Method{Name: "Q.byId", ClassName: "Q", IsStatic: true, ReturnType: "String", Params: []Param{{Name: "value", Type: spec.param}}, Program: body}); err != nil {
					t.Fatal(err)
				}
			}
			nullID, err := CompileAnonymous("Id x = null; return x;")
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{Name: "Q.nullId", ClassName: "Q", IsStatic: true, ReturnType: "Id", Program: nullID}); err != nil {
				t.Fatal(err)
			}
			program, err := CompileAnonymous("String r; " + tc.call + " System.assertEquals('" + tc.want + "', r);")
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			if tc.ambiguous {
				if err == nil || !strings.Contains(err.Error(), "ambiguous") {
					t.Fatalf("literal null must stay ambiguous: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}
