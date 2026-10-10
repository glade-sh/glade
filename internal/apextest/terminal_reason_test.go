package apextest

import (
	"context"
	"encoding/json"
	"testing"
)

func TestCanceledCaseStructuredReason(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"cancelled", context.Canceled, "cancelled"},
		{"deadline", context.DeadlineExceeded, "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := canceledCase(TestCase{ClassName: "PacketTest", MethodName: "run"}, tc.err)
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]any
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if fields["reason"] != tc.want {
				t.Fatalf("reason = %v, want %s; report %s", fields["reason"], tc.want, data)
			}
		})
	}
}
