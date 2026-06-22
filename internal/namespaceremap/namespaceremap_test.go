package namespaceremap

import (
	"strings"
	"testing"
)

func TestApplyNamespace(t *testing.T) {
	rules := []Rule{{From: "NU", To: "znu"}}
	if got := ApplyNamespace(rules, "NU"); got != "znu" {
		t.Fatalf("ApplyNamespace NU = %q, want znu", got)
	}
	if got := ApplyNamespace(rules, "nu"); got != "znu" {
		t.Fatalf("ApplyNamespace nu = %q, want znu", got)
	}
	if got := ApplyNamespace(rules, "other"); got != "other" {
		t.Fatalf("ApplyNamespace other = %q, want other", got)
	}
}

func TestApplyMetadataName(t *testing.T) {
	rules := []Rule{{From: "NU", To: "znu"}}
	cases := map[string]string{
		"NU__Billing__c": "znu__Billing__c",
		"NU.Helper":      "znu.Helper",
		"NUBilling__c":   "NUBilling__c",
		"Other__c":       "Other__c",
	}
	for input, want := range cases {
		if got := ApplyMetadataName(rules, input); got != want {
			t.Fatalf("ApplyMetadataName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestApplySourceRewritesCodeAndStringTokensOnly(t *testing.T) {
	rules := []Rule{{From: "NU", To: "znu"}}
	src := strings.Join([]string{
		"public class Consumer {",
		"  NU.Helper helper;",
		"  String soql = 'SELECT Id FROM NU__Billing__c WHERE Type = \\'NU.Helper\\'';",
		"  String words = 'NU.Helper NU__Billing__c ANU NUx';",
		"  // NU.Helper NU__Billing__c",
		"  /* NU.Helper NU__Billing__c */",
		"  public void run() { NU.Helper.call(); Integer NUT = 1; }",
		"}",
	}, "\n")

	got := ApplySource(rules, src)
	for _, want := range []string{
		"znu.Helper helper",
		"FROM znu__Billing__c",
		"\\'znu.Helper\\'",
		"'znu.Helper znu__Billing__c ANU NUx'",
		"znu.Helper.call()",
		"Integer NUT = 1",
		"// NU.Helper NU__Billing__c",
		"/* NU.Helper NU__Billing__c */",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rewritten source missing %q:\n%s", want, got)
		}
	}
}

func TestFingerprintDependsOnRules(t *testing.T) {
	first := Fingerprint([]Rule{{From: "NU", To: "znu"}})
	second := Fingerprint([]Rule{{From: "NU", To: "other"}})
	if first == "" {
		t.Fatal("expected non-empty fingerprint")
	}
	if first == second {
		t.Fatalf("fingerprints should differ: %q", first)
	}
}
