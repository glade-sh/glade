package namespaceremap

import (
	"strings"
	"testing"
)

func TestApplyNamespace(t *testing.T) {
	rules := []Rule{{From: "BasePkg", To: "stagepkg"}}
	if got := ApplyNamespace(rules, "BasePkg"); got != "stagepkg" {
		t.Fatalf("ApplyNamespace BasePkg = %q, want stagepkg", got)
	}
	if got := ApplyNamespace(rules, "basepkg"); got != "stagepkg" {
		t.Fatalf("ApplyNamespace basepkg = %q, want stagepkg", got)
	}
	if got := ApplyNamespace(rules, "other"); got != "other" {
		t.Fatalf("ApplyNamespace other = %q, want other", got)
	}
}

func TestApplyMetadataName(t *testing.T) {
	rules := []Rule{{From: "BasePkg", To: "stagepkg"}}
	cases := map[string]string{
		"BasePkg__Billing__c": "stagepkg__Billing__c",
		"BasePkg.Helper":      "stagepkg.Helper",
		"BasePkgBilling__c":   "BasePkgBilling__c",
		"Other__c":            "Other__c",
	}
	for input, want := range cases {
		if got := ApplyMetadataName(rules, input); got != want {
			t.Fatalf("ApplyMetadataName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestApplySourceRewritesCodeAndStringTokensOnly(t *testing.T) {
	rules := []Rule{{From: "BasePkg", To: "stagepkg"}}
	src := strings.Join([]string{
		"public class Consumer {",
		"  BasePkg.Helper helper;",
		"  String soql = 'SELECT Id FROM BasePkg__Billing__c WHERE Type = \\'BasePkg.Helper\\'';",
		"  String words = 'BasePkg.Helper BasePkg__Billing__c ABasePkg BasePkgx';",
		"  // BasePkg.Helper BasePkg__Billing__c",
		"  /* BasePkg.Helper BasePkg__Billing__c */",
		"  public void run() { BasePkg.Helper.call(); Integer BasePkgT = 1; }",
		"}",
	}, "\n")

	got := ApplySource(rules, src)
	for _, want := range []string{
		"stagepkg.Helper helper",
		"FROM stagepkg__Billing__c",
		"\\'stagepkg.Helper\\'",
		"'stagepkg.Helper stagepkg__Billing__c ABasePkg BasePkgx'",
		"stagepkg.Helper.call()",
		"Integer BasePkgT = 1",
		"// BasePkg.Helper BasePkg__Billing__c",
		"/* BasePkg.Helper BasePkg__Billing__c */",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rewritten source missing %q:\n%s", want, got)
		}
	}
}

func TestFingerprintDependsOnRules(t *testing.T) {
	first := Fingerprint([]Rule{{From: "BasePkg", To: "stagepkg"}})
	second := Fingerprint([]Rule{{From: "BasePkg", To: "other"}})
	if first == "" {
		t.Fatal("expected non-empty fingerprint")
	}
	if first == second {
		t.Fatalf("fingerprints should differ: %q", first)
	}
}
