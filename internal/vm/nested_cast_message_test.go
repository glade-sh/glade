package vm

import (
	"strings"
	"testing"
)

func TestNestedCastMessageTargetScope(t *testing.T) {
	for _, tc := range []struct{ name, context, target, want string }{
		{"lexical nested", "Owner", "Target", "Owner.Target"},
		{"qualified nested", "Owner", "Owner.Target", "Owner.Target"},
		{"top level", "Unrelated", "Target", "Target"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			for _, class := range []Class{{Name: "Owner"}, {Name: "Owner.Target"}, {Name: "Unrelated"}, {Name: "Target"}, {Name: "Source"}} {
				if err := machine.RegisterClass(class); err != nil {
					t.Fatal(err)
				}
			}
			machine.currentClass = tc.context
			_, err := machine.coerceCast(tc.target, Object("Source"))
			want := "Invalid conversion from runtime type Source to " + tc.want
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("cast error=%v, want %q", err, want)
			}
			// Message formatting must not change successful or null casts.
			got, err := machine.coerceCast(tc.target, Null)
			if err != nil || got.Kind != ValueNull {
				t.Fatalf("null cast=%#v, err=%v", got, err)
			}
			got, err = machine.coerceCast(tc.target, Object(tc.want))
			if err != nil || got.Type != tc.want {
				t.Fatalf("valid cast=%#v, err=%v", got, err)
			}
		})
	}
}
