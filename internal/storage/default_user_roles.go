package storage

// ensureDefaultUserRoles materializes the fresh Developer hierarchy observed in
// no_row_seed_inventory at APIs 62 and 67. Existing identities and fields win.
func ensureDefaultUserRoles(org *OrgState) {
	roles := []struct{ developerName, name, parent string }{
		{"CEO", "CEO", ""},
		{"CFO", "CFO", "CEO"},
		{"COO", "COO", "CEO"},
		{"SVPCustomerServiceSupport", "SVP, Customer Service & Support", "CEO"},
		{"SVPHumanResources", "SVP, Human Resources", "CEO"},
		{"SVPSalesMarketing", "SVP, Sales & Marketing", "CEO"},
		{"CustomerSupportInternational", "Customer Support, International", "SVPCustomerServiceSupport"},
		{"CustomerSupportNorthAmerica", "Customer Support, North America", "SVPCustomerServiceSupport"},
		{"InstallationRepairServices", "Installation & Repair Services", "SVPCustomerServiceSupport"},
		{"VPInternationalSales", "VP, International Sales", "SVPSalesMarketing"},
		{"VPMarketing", "VP, Marketing", "SVPSalesMarketing"},
		{"VPNorthAmericanSales", "VP, North American Sales", "SVPSalesMarketing"},
		{"MarketingTeam", "Marketing Team", "VPMarketing"},
		{"DirectorChannelSales", "Director, Channel Sales", "VPNorthAmericanSales"},
		{"DirectorDirectSales", "Director, Direct Sales", "VPNorthAmericanSales"},
		{"ChannelSalesTeam", "Channel Sales Team", "DirectorChannelSales"},
		{"EasternSalesTeam", "Eastern Sales Team", "DirectorDirectSales"},
		{"WesternSalesTeam", "Western Sales Team", "DirectorDirectSales"},
	}
	ids := make(map[string]ID, len(roles))
	for _, role := range roles {
		parent := NullValue()
		if role.parent != "" {
			parent = IDValue(ids[role.parent])
		}
		ids[role.developerName] = putDefaultSetupRecord(org, "UserRole", "DeveloperName", 1, map[string]Value{
			"DeveloperName": StringValue(role.developerName), "Name": StringValue(role.name), "ParentRoleId": parent,
		})
	}
}
