package vm

import "testing"

func TestJSONPublicSerializerKeepsRecordCycleCut(t *testing.T) {
	record := Object("Account")
	record.Fields["Name"] = String("cycle")
	record.Fields["Parent"] = record
	for _, tc := range []struct {
		name, want string
		value      Value
	}{
		{"record", `{"attributes":{"type":"Account"},"Name":"cycle","Parent":null}`, record},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := New(nil).call("JSON.serialize", []Value{tc.value}, nil, &Result{})
			if err != nil || got.Kind != ValueString || got.Text != tc.want {
				t.Fatalf("public serialization=%v err=%v; want %s", got, err, tc.want)
			}
		})
	}
}
