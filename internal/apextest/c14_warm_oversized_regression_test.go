package apextest

import (
	"github.com/glade-sh/glade/internal/testreport"
	"testing"
)

func TestC14WarmOversizedCarryIsExact(t *testing.T) {
	cases := []struct{ class, method, message string }{
		{"DootAY3A1031Test", "oversizedLocator", "Database.executeBatch scope size must be at most 2000"},
		{"DootAY8A1031Test", "oversizedIterable", "System.scheduleBatch scope size must be at most 2000"},
	}
	for _, tc := range cases {
		for _, api := range []string{"62.0", "67.0"} {
			t.Run(api+"/"+tc.method, func(t *testing.T) {
				problem := &testreport.Problem{Type: "RuntimeError", Message: tc.message}
				if !c14WarmOversizedCarry(api, tc.class, tc.method, problem) {
					t.Fatal("observed rejection not recognized")
				}
				for _, wrong := range []*testreport.Problem{nil, {Type: "RuntimeError", Message: "different"}, {Type: "Other", Message: tc.message}} {
					if c14WarmOversizedCarry(api, tc.class, tc.method, wrong) {
						t.Fatal("different rejection accepted")
					}
				}
				if c14WarmOversizedCarry("68.0", tc.class, tc.method, problem) || c14WarmOversizedCarry(api, "Other", tc.method, problem) || c14WarmOversizedCarry(api, tc.class, "other", problem) {
					t.Fatal("unrelated route accepted")
				}
			})
		}
	}
}
