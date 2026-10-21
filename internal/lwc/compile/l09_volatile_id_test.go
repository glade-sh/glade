package compile_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestL09VolatileErrorIDCarry(t *testing.T) {
	raw, err := os.ReadFile("testdata/l09_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []l09Case
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	var numeric, uuid string
	for _, c := range cases {
		if c.ID == "r_getRecord_deleted" {
			numeric, uuid = c.Expected["59.0"], c.Expected["68.0"]
		}
	}
	if numeric == "" || uuid == "" {
		t.Fatal("missing captured error-ID control")
	}
	for _, tc := range []struct {
		name, api, got, want string
		carry, invalidID     bool
	}{
		{"numeric actual ID", "68.0", numeric, uuid, true, false},
		{"coincident UUID stays carried", "68.0", uuid, uuid, true, false},
		{"API59 stays exact", "59.0", numeric, uuid, false, false},
		{"API67 stays exact", "67.0", numeric, uuid, false, false},
		{"numeric oracle stays exact", "68.0", uuid, numeric, false, false},
		{"missing ID", "68.0", strings.Replace(numeric, `"id":"-857233874",`, "", 1), uuid, false, true},
		{"empty ID", "68.0", strings.Replace(numeric, `"id":"-857233874"`, `"id":""`, 1), uuid, false, true},
		{"blank ID", "68.0", strings.Replace(numeric, `"id":"-857233874"`, `"id":" "`, 1), uuid, false, true},
		{"null ID", "68.0", strings.Replace(numeric, `"id":"-857233874"`, `"id":null`, 1), uuid, false, true},
		{"non-string ID", "68.0", strings.Replace(numeric, `"id":"-857233874"`, `"id":123`, 1), uuid, false, true},
		{"message differs", "68.0", strings.Replace(numeric, "The requested resource does not exist", "another message", 1), uuid, false, false},
		{"status differs", "68.0", strings.Replace(numeric, `"statusCode":404`, `"statusCode":400`, 1), uuid, false, false},
		{"headers differ", "68.0", strings.Replace(numeric, `"headers":{}`, `"headers":{"extra":true}`, 1), uuid, false, false},
		{"error path differs", "68.0", strings.Replace(numeric, `"returned":`, `"thrown":`, 1), uuid, false, false},
		{"another ID is preserved", "68.0", strings.Replace(numeric, `"returned":{`, `"returned":{"id":"-857233874",`, 1), uuid, false, false},
		{"UUID outside fetchResponse", "68.0", numeric, strings.Replace(uuid, "fetchResponse", "anotherError", 1), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			carry, err := l09VolatileErrorIDCarry(tc.api, tc.got, tc.want)
			if carry != tc.carry || (err != nil) != tc.invalidID {
				t.Fatalf("carry=%v error=%v; want carry=%v invalidID=%v", carry, err, tc.carry, tc.invalidID)
			}
		})
	}
}
