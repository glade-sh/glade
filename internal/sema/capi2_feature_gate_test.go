package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// capi2_feature_gates/org.tsv: these ConnectApi services compile only in a
// scratch shape that enables their feature or setting. A project without a
// scratch definition keeps the permissive default.
func TestCapi2FeatureGatedServices(t *testing.T) {
	cases := []struct {
		service, call, enabled, hidden string
	}{
		{"CommerceCart", "ConnectApi.CommerceCart.abortCartOperation('a','b','c');", `{"features":["OrderManagement"]}`, `{"features":["B2BCommerce"]}`},
		{"OmniDesignerConnect", "ConnectApi.OmniDesignerConnect.clearDatamapperCache(null);", `{"features":["OmniStudioDesigner"]}`, `{"features":["Communities"]}`},
		{"NavigationMenu", "ConnectApi.NavigationMenu.getCommunityNavigationMenu(null,null,null,null,false,false,null);", `{"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`, `{"features":["ServiceCloud"]}`},
		{"Personalization", "ConnectApi.Personalization.createAudience(null,null);", `{"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`, `{"features":["ServiceCloud"]}`},
		{"EinsteinLLM", "ConnectApi.EinsteinLlm.createEmbeddings(null);", `{"features":["Einstein1AIPlatform"]}`, `{"features":["Communities"]}`},
		// The license feature without the setting leaves Knowledge hidden.
		{"Knowledge", "ConnectApi.Knowledge.getTrendingArticles(null,null);", `{"features":["Communities"],"settings":{"knowledgeSettings":{"enableKnowledge":true}}}`, `{"features":["Knowledge","Communities"]}`},
		{"Wave", "ConnectApi.Wave.executeQuery('q');", `{"features":["DevelopmentWave"]}`, `{"features":["AnalyticsAdminPerms"]}`},
		{"CdpQuery", "ConnectApi.CdpQuery.cancelQuerySql('q');", `{"features":["DevelopmentWave"]}`, `{"features":["AnalyticsAdminPerms"]}`},
		// capi3_feature_gates/org.tsv: a declaration names the type the same way.
		{"CdpCatalog", "ConnectApi.CdpCatalog x;", `{"features":["DevelopmentWave"]}`, `{"features":["AnalyticsAdminPerms"]}`},
		{"SmartDataDiscovery", "ConnectApi.SmartDataDiscovery x;", `{"features":["DevelopmentWave"]}`, `{"features":["AnalyticsAdminPerms"]}`},
		{"Sites", "ConnectApi.Sites x;", `{"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`, `{"features":["Communities","ServiceCloud"],"settings":{"communitiesSettings":{"enableNetworksEnabled":false}}}`},
		{"ExternalManagedAccount", "ConnectApi.ExternalManagedAccount x;", `{"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`, `{"features":["ServiceCloud"]}`},
		{"MarketingIntegration", "ConnectApi.MarketingIntegration x;", `{"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`, `{"features":["ServiceCloud"]}`},
		{"CommerceInventory", "ConnectApi.CommerceInventory x;", `{"features":["OrderManagement"]}`, `{"features":["B2BCommerce"]}`},
		{"CommercePromotions", "ConnectApi.CommercePromotions x;", `{"features":["OrderManagement"]}`, `{"features":["B2BCommerce"]}`},
		{"RegisterGuestBuyer", "ConnectApi.RegisterGuestBuyer x;", `{"features":["OrderManagement"]}`, `{"features":["B2BCommerce"]}`},
		{"Repricing", "ConnectApi.Repricing x;", `{"features":["OrderManagement"]}`, `{"features":["B2BCommerce"]}`},
		{"ReturnOrder", "ConnectApi.ReturnOrder x;", `{"features":["OrderManagement"]}`, `{"features":["B2BCommerce"]}`},
		// The Order Management setting adds three more services.
		{"OMSAnalytics", "ConnectApi.OMSAnalytics x;", `{"features":["OrderManagement"],"settings":{"orderManagementSettings":{"enableOrderManagement":true}}}`, `{"features":["OrderManagement"]}`},
		{"PickTicket", "ConnectApi.PickTicket x;", `{"features":["OrderManagement"],"settings":{"orderManagementSettings":{"enableOrderManagement":true}}}`, `{"features":["OrderManagement"]}`},
		{"Routing", "ConnectApi.Routing x;", `{"features":["OrderManagement"],"settings":{"orderManagementSettings":{"enableOrderManagement":true}}}`, `{"features":["OrderManagement"],"settings":{"orderSettings":{"enableEnhancedCommerceOrders":true}}}`},
		// Enhanced orders add three more.
		{"EnsurePaymentCredit", "ConnectApi.EnsurePaymentCredit x;", `{"features":["OrderManagement"],"settings":{"orderSettings":{"enableEnhancedCommerceOrders":true}}}`, `{"features":["OrderManagement"],"settings":{"orderManagementSettings":{"enableOrderManagement":true}}}`},
		{"Exchanges", "ConnectApi.Exchanges x;", `{"features":["OrderManagement"],"settings":{"orderSettings":{"enableEnhancedCommerceOrders":true}}}`, `{"features":["OrderManagement"]}`},
		{"OrderSummaryCreation", "ConnectApi.OrderSummaryCreation x;", `{"settings":{"orderSettings":{"enableEnhancedCommerceOrders":true}}}`, `{"features":["OrderManagement"],"settings":{"communitiesSettings":{"enableNetworksEnabled":true}}}`},
		// Both settings open the order-summary family.
		{"OrderSummary", "ConnectApi.OrderSummary x;", `{"features":["OrderManagement"],"settings":{"orderManagementSettings":{"enableOrderManagement":true},"orderSettings":{"enableEnhancedCommerceOrders":true}}}`, `{"features":["OrderManagement"],"settings":{"orderSettings":{"enableEnhancedCommerceOrders":true}}}`},
		{"FulfillmentOrder", "ConnectApi.FulfillmentOrder x;", `{"features":["OrderManagement"],"settings":{"orderManagementSettings":{"enableOrderManagement":true},"orderSettings":{"enableEnhancedCommerceOrders":true}}}`, `{"features":["OrderManagement"],"settings":{"orderManagementSettings":{"enableOrderManagement":true}}}`},
		{"OrderPaymentSummary", "ConnectApi.OrderPaymentSummary x;", `{"features":["OrderManagement"],"settings":{"orderManagementSettings":{"enableOrderManagement":true},"orderSettings":{"enableEnhancedCommerceOrders":true}}}`, `{"features":["OrderManagement"]}`},
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, c := range cases {
			for _, mode := range []string{"enabled", "hidden", "unspecified"} {
				t.Run(api+"/"+c.service+"/"+mode, func(t *testing.T) {
					root := t.TempDir()
					path := filepath.Join(root, "Probe.cls")
					writeSemaFile(t, path, "public class Probe { public static void run() { "+c.call+" } }")
					if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
						t.Fatal(err)
					}
					switch mode {
					case "enabled":
						writeSemaFile(t, filepath.Join(root, "config/project-scratch-def.json"), c.enabled)
					case "hidden":
						writeSemaFile(t, filepath.Join(root, "config/project-scratch-def.json"), c.hidden)
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
					result := Analyze(index)
					got := ""
					for _, d := range result.Diagnostics {
						if strings.HasPrefix(d.Message, "Type is not visible: ConnectApi.") {
							got = d.Message
						}
					}
					want := mode == "hidden"
					if (got != "") != want {
						t.Fatalf("hidden diagnostic = %q, want hidden=%v: %#v", got, want, result.Diagnostics)
					}
					if want && got != "Type is not visible: ConnectApi."+c.service {
						t.Fatalf("message = %q", got)
					}
				})
			}
		}
	}
}

// An undeclared project keeps the permissive default even when positive
// evidence (a Person Account record type, a dependency scratch definition)
// puts real features beside the unspecified-shape token.
func TestCapi2FeatureGateUndeclaredWithEvidence(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, kind := range []string{"person-account", "dependency-scratch-def"} {
			t.Run(api+"/"+kind, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "Probe.cls")
				writeSemaFile(t, path, "public class Probe { public static void run() { ConnectApi.CommerceCart.abortCartOperation('a','b','c'); } }")
				p := project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}
				switch kind {
				case "person-account":
					if err := os.MkdirAll(filepath.Join(root, "force-app/main/default/objects/PersonAccount/recordTypes"), 0o755); err != nil {
						t.Fatal(err)
					}
					writeSemaFile(t, filepath.Join(root, "force-app/main/default/objects/PersonAccount/recordTypes/Person.recordType-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><RecordType xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Person</fullName></RecordType>`)
				case "dependency-scratch-def":
					dep := t.TempDir()
					if err := os.MkdirAll(filepath.Join(dep, "config"), 0o755); err != nil {
						t.Fatal(err)
					}
					writeSemaFile(t, filepath.Join(dep, "config/project-scratch-def.json"), `{"edition":"Developer","features":["Communities"]}`)
					p.ManagedPackageDependencies = []project.ManagedPackageDependency{{Status: "loaded", Project: &project.Project{Root: dep}}}
				}
				index := typesys.Build(p, schema.Schema{})
				found := false
				for _, f := range index.OrgShapeFeatures {
					found = found || f == project.UnspecifiedStandardFieldShape
				}
				if len(index.OrgShapeFeatures) < 2 || !found {
					t.Fatalf("setup must give evidence beside the token: %v", index.OrgShapeFeatures)
				}
				for _, d := range Analyze(index).Diagnostics {
					if strings.HasPrefix(d.Message, "Type is not visible: ConnectApi.") {
						t.Fatalf("undeclared project was gated: %v (%v)", d.Message, index.OrgShapeFeatures)
					}
				}
			})
		}
	}
}
