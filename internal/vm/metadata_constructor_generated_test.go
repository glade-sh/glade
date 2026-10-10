package vm

import "testing"

func TestGeneratedMetadataDTOConstructorsUseCompatibilityDefaults(t *testing.T) {
	vm := New(nil)
	cases := []struct {
		name  string
		check func(t *testing.T, value Value)
	}{
		{
			name: "Metadata.AsyncResult",
			check: func(t *testing.T, value Value) {
				if got := value.Fields["done"]; got.Kind != ValueBool || !got.Bool {
					t.Fatalf("done = %#v, want true", got)
				}
				if got := value.Fields["state"]; got.Kind != ValueString || got.Text != "Succeeded" {
					t.Fatalf("state = %#v, want Succeeded", got)
				}
			},
		},
		{
			name: "Metadata.DeployMessage",
			check: func(t *testing.T, value Value) {
				// A44 E001 captures the complete native constructor defaults.
				if got := value.Fields["success"]; got.Kind != ValueNull {
					t.Fatalf("success = %#v, want raw null", got)
				}
				if got := value.Fields["lineNumber"]; got.Kind != ValueNull {
					t.Fatalf("lineNumber = %#v, want raw null", got)
				}
			},
		},
		{
			name: "Metadata.DeployResult",
			check: func(t *testing.T, value Value) {
				// A44 R141/R143 and E002: construction never synthesizes success.
				if got := value.Fields["done"]; got.Kind != ValueNull {
					t.Fatalf("done = %#v, want raw null", got)
				}
				if got := value.Fields["success"]; got.Kind != ValueNull {
					t.Fatalf("success = %#v, want raw null", got)
				}
				if got := value.Fields["messages"]; got.Kind != ValueList || len(got.List) != 0 {
					t.Fatalf("messages = %#v, want empty list", got)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value, handled, err := vm.constructGeneratedPlatformValue(tc.name, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !handled {
				t.Fatal("generated constructor was not handled")
			}
			tc.check(t, value)
		})
	}
}
