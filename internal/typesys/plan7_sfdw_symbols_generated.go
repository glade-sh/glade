package typesys

// Code generated from public Salesforce product namespace declarations. DO NOT EDIT.

var plan7SFDWSymbolSpecs = []StandardSymbolSpec{
	{
		Name:         "sfdw.ConsumptionAlert",
		Constructors: [][]string{{"String", "Integer", "Boolean", "Datetime"}},
		Properties: []StandardPropertySpec{
			{Name: "alertTriggerType", Type: "String"},
			{Name: "alertTriggerValue", Type: "Integer"},
			{Name: "createdDate", Type: "Datetime"},
			{Name: "isNotificationSent", Type: "Boolean"},
		},
	},
	{
		Name:         "sfdw.ConsumptionCard",
		Constructors: [][]string{{"String", "String", "String", "Double", "Double", "List<sfdw.ConsumptionAlert>"}},
		Properties: []StandardPropertySpec{
			{Name: "businessEnvType", Type: "String"},
			{Name: "cardDefinitionDeveloperName", Type: "String"},
			{Name: "consumptionAlerts", Type: "List<sfdw.ConsumptionAlert>"},
			{Name: "totalEntitlement", Type: "Double"},
			{Name: "unitsConsumed", Type: "Double"},
			{Name: "usageModel", Type: "String"},
		},
	},
}
