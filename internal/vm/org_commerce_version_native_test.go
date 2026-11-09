package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// Native rows: org_commerce_version/org.tsv and org_commerce_family raw rows.
func TestOrgCommerceVersionAvailabilityAndWarmDescribe(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{"OrderManagement"})
	for _, name := range []string{"PaymentCredit", "PaymentCreditLinePayment", "PaymentCreditTransaction", "WebCartCredit"} {
		storage.EnsureStandardObject(&org, name)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, api := range []struct {
		version string
		major   int
	}{{"67.0", 67}, {"62.0", 62}, {"64.0", 64}, {"65.0", 65}, {"63.0", 63}, {"66.0", 66}, {"67.0", 67}} {
		machine.currentMethod = Method{APIVersion: api.version}
		for _, tc := range []struct {
			name  string
			since int
		}{{"PaymentCredit", 65}, {"PaymentCreditLinePayment", 65}, {"PaymentCreditTransaction", 65}, {"WebCartCredit", 64}} {
			want := api.major >= tc.since
			_, described := machine.schemaGlobalDescribe().Map[mapKey(String(strings.ToLower(tc.name)))]
			if described != want {
				t.Errorf("API %s %s describe=%t want=%t", api.version, tc.name, described, want)
			}
			if got := machine.typeForName("", "Schema."+tc.name, false).Kind != ValueNull; got != want {
				t.Errorf("API %s %s type=%t want=%t", api.version, tc.name, got, want)
			}
			if err := machine.enforceOrgShapeObjectAvailability(soql.Query{Object: tc.name}); (err == nil) != want {
				t.Errorf("API %s %s query error=%v want=%t", api.version, tc.name, err, want)
			}
		}
	}
}

// Distinct VMs borrowing one frozen describe generation must not reuse a map
// built for another caller API version.
func TestOrgCommerceVersionSharedDescribe(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{"OrderManagement"})
	storage.EnsureStandardObject(&org, "WebCartCredit")
	org.RuntimeSchemaStamp = "synthetic-commerce-version"
	share := newDescribeShareCache(org.RuntimeSchemaStamp)
	for _, tc := range []struct {
		api  string
		want bool
	}{{"62.0", false}, {"67.0", true}, {"63.0", false}, {"64.0", true}} {
		machine := New(nil)
		machine.SetOrg(&org)
		machine.ensureMetadataCacheGeneration()
		machine.describeShare = share
		machine.currentMethod = Method{APIVersion: tc.api}
		_, got := machine.schemaGlobalDescribe().Map[mapKey(String("webcartcredit"))]
		if got != tc.want {
			t.Errorf("API %s shared describe=%t want=%t", tc.api, got, tc.want)
		}
	}
}

func TestOrgCommerceVersionUnspecifiedFallback(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{project.UnspecifiedStandardFieldShape})
	storage.EnsureStandardObject(&org, "WebCartCredit")
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "62.0"}
	if _, ok := machine.schemaGlobalDescribe().Map[mapKey(String("webcartcredit"))]; !ok {
		t.Fatal("unspecified project lost compatibility fallback")
	}
}

// B2B version observations are retained in org_commerce_b2b/raw-version.jsonl.
func TestOrgCommerceB2BVersionAvailability(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{"B2BCommerce", "Communities", "NetworksEnabled", "CommerceEnabled"})
	machine := New(nil)
	machine.SetOrg(&org)
	for _, tc := range []struct {
		api  string
		want bool
	}{{"62.0", false}, {"63.0", false}, {"64.0", true}, {"65.0", true}, {"67.0", true}} {
		machine.currentMethod = Method{APIVersion: tc.api}
		if got := machine.allowsStandardObject("WebCartCredit"); got != tc.want {
			t.Errorf("API %s: got %t, want %t", tc.api, got, tc.want)
		}
	}
}
