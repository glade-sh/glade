package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Local server/LWC bridge contract over the captured Apex availability rows;
// this does not assert Salesforce hosted wire diagnostics.
func TestOrgFeatureAvailabilityLightningObjectInfo(t *testing.T) {
	for _, tc := range []struct {
		object   string
		features []string
	}{
		{"CurrencyType", []string{"MultiCurrency"}}, {"DatedConversionRate", []string{"MultiCurrency"}},
		{"Territory2", []string{"Territory2"}}, {"Territory2Model", []string{"Territory2"}},
		{"AccountTeamMember", []string{"AccountTeams"}}, {"ForecastingItem", []string{"Forecasts"}},
		{"OpportunityTeamMember", []string{"OpportunityTeam"}}, {"OpportunitySplit", []string{"OpportunityTeam", "OpportunitySplits"}},
		{"ServiceChannel", []string{"OmniChannel"}}, {"Survey", []string{"Surveys"}},
		{"Knowledge__ka", []string{"KnowledgeEnabled"}}, {"Knowledge__kav", []string{"KnowledgeEnabled"}},
		{"Network", []string{"Communities", "NetworksEnabled"}}, {"NetworkMember", []string{"Communities", "NetworksEnabled"}},
		{"QuoteDocument", []string{"Quotes"}}, {"QuoteFeed", []string{"Quotes"}}, {"QuoteHistory", []string{"Quotes"}},
		{"BuyerGroup", []string{"OrderManagement"}}, {"CartItem", []string{"OrderManagement"}}, {"CartTax", []string{"OrderManagement"}},
		{"PickTicket", []string{"OrderManagement", "OrderManagementEnabled"}}, {"UserLocationAssignment", []string{"OrderManagement", "OrderManagementEnabled"}},
		{"PaymentCredit", []string{"OrderManagement"}}, {"PendingOrderSummary", []string{"OrderManagement"}},
		{"AccountBrand", []string{"NetworksEnabled"}}, {"DelegatedAccount", []string{"NetworksEnabled"}},
		{"WebStore", []string{"B2BCommerce", "Communities", "NetworksEnabled", "CommerceEnabled"}},
	} {
		for _, enabled := range []bool{false, true} {
			t.Run(tc.object+map[bool]string{true: "/enabled", false: "/disabled"}[enabled], func(t *testing.T) {
				org := storage.NewOrgState()
				if enabled {
					storage.ApplyProjectOrgShape(&org, tc.features)
				} else {
					storage.ApplyProjectOrgShape(&org, nil)
				}
				storage.EnsureStandardObject(&org, tc.object)
				req := httptest.NewRequest(http.MethodPost, "/lightning/wire/getObjectInfo", strings.NewReader(`{"objectApiName":"`+tc.object+`"}`))
				rec := httptest.NewRecorder()
				New(&org).ServeHTTP(rec, req)
				var result struct {
					Data  any `json:"data"`
					Error any `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if (result.Error == nil) != enabled {
					t.Fatalf("enabled=%t status=%d body=%s", enabled, rec.Code, rec.Body.String())
				}
			})
		}
	}
}

// Anonymous describe is absent in both captured unlicensed shapes.
func TestCampaignInfluenceLightningObjectInfo(t *testing.T) {
	for _, features := range [][]string{nil, {"CampaignInfluence2Enabled"}} {
		org := storage.NewOrgState()
		storage.ApplyProjectOrgShape(&org, features)
		storage.EnsureStandardObject(&org, "CampaignInfluence")
		req := httptest.NewRequest(http.MethodPost, "/lightning/wire/getObjectInfo", strings.NewReader(`{"objectApiName":"CampaignInfluence"}`))
		rec := httptest.NewRecorder()
		New(&org).ServeHTTP(rec, req)
		var result struct {
			Error any `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Error == nil {
			t.Fatalf("unlicensed features=%v body=%s", features, rec.Body.String())
		}
	}
}
