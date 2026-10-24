package vm

import "testing"

func TestIDOriginalDisplayRepresentationRoutes(t *testing.T) {
	const text = "707000000000001AAA"
	for name, base := range map[string]Value{
		"platform Id": platformScalar("Id", text),
		"scalar Id":   {Kind: ValueString, Type: "Id", Text: text},
	} {
		t.Run(name, func(t *testing.T) {
			// Exercise Id value equality without the shared-object Ref shortcut.
			base.Ref = 0
			original := base
			original.idOriginalDisplay = true
			t.Run("Id equality ignores display provenance", func(t *testing.T) {
				if !base.Equal(original) || !original.Equal(base) {
					t.Fatal("display provenance must not change Id equality")
				}
				for _, item := range []struct {
					value Value
					want  int
				}{{base, 18}, {original, 15}} {
					got, err := stringStatic("String.valueOf", []Value{item.value})
					if err != nil || len(got.Text) != item.want {
						t.Fatalf("display length = %d, want %d: %v", len(got.Text), item.want, err)
					}
				}
			})
			for route, same := range map[string]func(Value, Value) bool{
				"header and backing": valueSameHeaderAndBacking,
				"map coercion":       sameCoercionRepresentation,
			} {
				t.Run(route, func(t *testing.T) {
					if same(base, original) || same(original, base) {
						t.Fatal("different Id display provenance must have different representations")
					}
					if !same(base, base) || !same(original, original) {
						t.Fatal("unchanged display provenance must retain the same representation")
					}
				})
			}
		})
	}
}

func TestIDOriginalDisplayRecordScopeElisionRoute(t *testing.T) {
	before := platformScalar("Id", "707000000000001AAA")
	before.Ref = 41
	after := before
	after.idOriginalDisplay = true
	machine := New(nil)
	if !machine.shouldElideRecordScopeWalk(before, before) {
		t.Fatal("unchanged Id representation should allow scope elision")
	}
	if machine.shouldElideRecordScopeWalk(before, after) || machine.shouldElideRecordScopeWalk(after, before) {
		t.Fatal("changed Id display provenance must retain scope propagation")
	}
}
