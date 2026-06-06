package surfaceledger

import (
	"strings"
	"testing"
)

func TestAreaRegistryNamesInitialParallelAreas(t *testing.T) {
	want := []string{
		"Ledger.Identity",
		"Core.Runtime.System.FeatureManagement",
		"Core.Runtime.Database.Batchable",
		"Core.Runtime.SystemAndStdlib",
		"Query.Runtime.SOQLSOSL",
		"Data.Reference.ObjectsFields",
		"Data.Runtime.SchemaDescribe",
		"Data.Runtime.SOQL",
		"Data.Runtime.DML",
		"Tests.AsyncAndIsolation",
		"UI.ApexPagesControllers",
		"UI.VisualforceComponents",
		"UI.LWCModules",
		"UI.AuraComponents",
		"UI.UIAPI",
		"Server.RESTResources",
		"Server.ToolingObjects",
		"Integration.GraphQL",
		"Integration.PubSub",
		"Integration.BulkAPI",
		"Integration.MetadataAPI",
		"Integration.SOAPAPI",
		"Integration.StreamingAPI",
		"Integration.SalesforceConnect.AmazonRDS",
		"Platform.Events",
		"AI.Agentforce",
		"External.MarketingCloud.AMPscript",
		"External.MarketingCloud.Handlebars",
		"ConnectApi.PassiveDTOs",
	}
	got := map[string]bool{}
	for _, area := range AreaRegistry() {
		got[area.Name] = true
		if area.Owner == "" || area.RowFilter == "" || area.AreaRatchetCommand == "" {
			t.Fatalf("area %q is missing owner, row filter, or ratchet command: %#v", area.Name, area)
		}
		if len(area.DependsOn) == 0 || len(area.MayRunInParallelWith) == 0 || len(area.SharedFiles) == 0 || len(area.ExclusiveFiles) == 0 {
			t.Fatalf("area %q is missing parallel-work boundaries: %#v", area.Name, area)
		}
	}
	for _, name := range want {
		if !got[name] {
			t.Fatalf("area registry missing %q", name)
		}
	}
}

func TestPacketMarkdownIncludesAgentCloseoutRules(t *testing.T) {
	packet, ok := AreaPacketByName("Core.Runtime.System.FeatureManagement")
	if !ok {
		t.Fatal("missing FeatureManagement packet")
	}
	markdown := PacketMarkdown(SurfaceLedger{}, packet)
	for _, want := range []string{
		"## Standard Validation Block",
		"focused tests run",
		"fixture command run",
		"surface refresh run",
		"area ratchet command run",
		"before counts",
		"after counts",
		"next top row",
		"## Docs Defect Path",
		"re-scrape docs",
		"patch the docs parser to read existing docs correctly",
		"## Reviewer Checklist",
		"no corpus-specific runtime hacks",
		"packet area did not expand during work",
		"## Breadth Work Order",
		"Schema.Describe",
	} {
		if !strings.Contains(markdown, want) {
			t.Fatalf("packet markdown missing %q:\n%s", want, markdown)
		}
	}
}

func TestSchemaDescribePacketOwnsOnlySchemaNamespaceDescribeRows(t *testing.T) {
	packet, ok := AreaPacketByName("Data.Runtime.SchemaDescribe")
	if !ok {
		t.Fatal("missing SchemaDescribe packet")
	}
	schemaRow := SurfaceLedgerRow{
		SurfaceID: ApexMemberID("Schema", "DescribeFieldResult", "getName", []string{}),
		Product:   ProductApex,
		Namespace: "Schema",
		TypeName:  "DescribeFieldResult",
		Kind:      KindMethod,
	}
	if !packetOwnsRow(packet, schemaRow) {
		t.Fatalf("SchemaDescribe packet should own %s", schemaRow.SurfaceID)
	}
	foreignDescribe := SurfaceLedgerRow{
		SurfaceID: ApexTypeID("Invocable.Action", "DescribeResult"),
		Product:   ProductApex,
		Namespace: "Invocable.Action",
		TypeName:  "DescribeResult",
		Kind:      KindType,
	}
	if packetOwnsRow(packet, foreignDescribe) {
		t.Fatalf("SchemaDescribe packet should not own %s", foreignDescribe.SurfaceID)
	}
}

func TestDatabaseBatchablePacketUsesCanonicalDatabaseNamespace(t *testing.T) {
	packet, ok := AreaPacketByName("Core.Runtime.Database.Batchable")
	if !ok {
		t.Fatal("missing Database.Batchable packet")
	}
	batchableRow := SurfaceLedgerRow{
		SurfaceID:  ApexMemberID("Database", "Batchable", "start", []string{"Database.BatchableContext"}),
		Product:    ProductApex,
		Namespace:  "Database",
		TypeName:   "Batchable",
		MemberName: "start",
		Kind:       KindMethod,
	}
	if !packetOwnsRow(packet, batchableRow) {
		t.Fatalf("Database.Batchable packet should own %s", batchableRow.SurfaceID)
	}
	stdlibDatabaseRow := SurfaceLedgerRow{
		SurfaceID: ApexMemberID("System", "Database", "getQueryLocatorWithBinds", nil),
		Product:   ProductApex,
		Namespace: "System.Database",
		TypeName:  "getQueryLocatorWithBinds",
		Kind:      KindMethod,
	}
	if !packetOwnsRow(packet, stdlibDatabaseRow) {
		t.Fatalf("Database.Batchable packet should own stdlib matrix row %s", stdlibDatabaseRow.SurfaceID)
	}
	systemPacket, ok := AreaPacketByName("Core.Runtime.SystemAndStdlib")
	if !ok {
		t.Fatal("missing SystemAndStdlib packet")
	}
	stdlibDatabaseMethodRow := SurfaceLedgerRow{
		SurfaceID:  ApexMemberID("System", "Database", "getQueryLocator", []string{"String"}),
		Product:    ProductApex,
		Namespace:  "System",
		TypeName:   "Database",
		MemberName: "getQueryLocator",
		Kind:       KindMethod,
	}
	if packetOwnsRow(systemPacket, stdlibDatabaseMethodRow) {
		t.Fatalf("SystemAndStdlib packet should not own Database batch row %s", stdlibDatabaseMethodRow.SurfaceID)
	}
}

func TestAsyncAndIsolationPacketExcludesServiceAsyncDocs(t *testing.T) {
	packet, ok := AreaPacketByName("Tests.AsyncAndIsolation")
	if !ok {
		t.Fatal("missing Tests.AsyncAndIsolation packet")
	}
	localTestRow := SurfaceLedgerRow{
		SurfaceID: ApexMemberID("System", "Test", "startTest", []string{}),
		Product:   ProductApex,
		Namespace: "System",
		TypeName:  "Test",
		Kind:      KindMethod,
	}
	if !packetOwnsRow(packet, localTestRow) {
		t.Fatalf("Tests.AsyncAndIsolation packet should own %s", localTestRow.SurfaceID)
	}
	for _, row := range []SurfaceLedgerRow{
		{
			SurfaceID: "unknown:asynch_api_batches_create",
			Product:   ProductUnknown,
			Kind:      KindGuide,
		},
		{
			SurfaceID: ApexMemberID("ConnectApi", "CommerceBuyerExperience", "calculateAdjustmentAggregates", []string{"String", "ConnectApi.OrderSummaryAdjustmentAggregatesAsyncInput"}),
			Product:   ProductApex,
			Namespace: "ConnectApi",
			TypeName:  "CommerceBuyerExperience",
			Kind:      KindMethod,
		},
		{
			SurfaceID: "tooling:ContainerAsyncRequest",
			Product:   ProductTooling,
			Kind:      KindType,
		},
	} {
		if packetOwnsRow(packet, row) {
			t.Fatalf("Tests.AsyncAndIsolation packet should not own %s", row.SurfaceID)
		}
	}
}

func TestQuerySOQLSOSLPacketExcludesToolingRows(t *testing.T) {
	packet, ok := AreaPacketByName("Query.Runtime.SOQLSOSL")
	if !ok {
		t.Fatal("missing Query.Runtime.SOQLSOSL packet")
	}
	queryGuide := SurfaceLedgerRow{
		SurfaceID:  "unknown:sforce_api_calls_soql_select_limit",
		Product:    ProductUnknown,
		Area:       AreaRuntime,
		Kind:       KindGuide,
		DocsSource: "soql-sosl",
	}
	if !packetOwnsRow(packet, queryGuide) {
		t.Fatalf("Query.Runtime.SOQLSOSL packet should own %s", queryGuide.SurfaceID)
	}
	toolingRow := SurfaceLedgerRow{
		SurfaceID:  ToolingObjectID("SOQL"),
		Product:    ProductTooling,
		Area:       AreaServer,
		Kind:       KindType,
		DocsSource: "soql-sosl",
	}
	if packetOwnsRow(packet, toolingRow) {
		t.Fatalf("Query.Runtime.SOQLSOSL packet should not own tooling row %s", toolingRow.SurfaceID)
	}
}

func TestAuraComponentsPacketExcludesNonLocalLightningRows(t *testing.T) {
	packet, ok := AreaPacketByName("UI.AuraComponents")
	if !ok {
		t.Fatal("missing AuraComponents packet")
	}
	auraDoc := SurfaceLedgerRow{
		SurfaceID: "unknown:ref_aura_application",
		Product:   ProductUnknown,
		Area:      AreaUI,
		Kind:      KindGuide,
	}
	if !packetOwnsRow(packet, auraDoc) {
		t.Fatalf("AuraComponents packet should own %s", auraDoc.SurfaceID)
	}
	for _, row := range []SurfaceLedgerRow{
		{
			SurfaceID: "rest:resources_lightning_usagebypagemetrics.get",
			Product:   ProductREST,
			Area:      AreaServer,
			Kind:      KindResource,
		},
		{
			SurfaceID: ToolingObjectID("LightningComponentBundle"),
			Product:   ProductTooling,
			Area:      AreaServer,
			Kind:      KindType,
		},
		{
			SurfaceID: ApexTypeID("ConnectApi", "LightningExtensionInformation"),
			Product:   ProductApex,
			Namespace: "ConnectApi",
			TypeName:  "LightningExtensionInformation",
			Kind:      KindType,
		},
		{
			SurfaceID: LWCModuleID("`lightning/graphql`"),
			Product:   ProductLWC,
			Area:      AreaUI,
			Kind:      KindModule,
		},
	} {
		if packetOwnsRow(packet, row) {
			t.Fatalf("AuraComponents packet should not own %s", row.SurfaceID)
		}
	}
}

func TestVisualforceComponentsPacketExcludesAPIAndMetadataRows(t *testing.T) {
	packet, ok := AreaPacketByName("UI.VisualforceComponents")
	if !ok {
		t.Fatal("missing VisualforceComponents packet")
	}
	componentDoc := SurfaceLedgerRow{
		SurfaceID: "visualforce:pages_compref_page",
		Product:   ProductVisualforce,
		Area:      AreaUI,
		Kind:      KindGuide,
	}
	if !packetOwnsRow(packet, componentDoc) {
		t.Fatalf("VisualforceComponents packet should own %s", componentDoc.SurfaceID)
	}
	for _, row := range []SurfaceLedgerRow{
		{
			SurfaceID: "unknown:ui_api_responses_visualforce_layout_component",
			Product:   ProductUnknown,
			Area:      AreaServer,
			Kind:      KindGuide,
		},
		{
			SurfaceID: ApexMemberID("ConnectApi", "UserProfileTabType", "CustomVisualForce", nil),
			Product:   ProductApex,
			Namespace: "ConnectApi",
			TypeName:  "UserProfileTabType",
			Kind:      KindProperty,
		},
		{
			SurfaceID: ApexMemberID("Metadata", "FeedLayoutComponentType", "Visualforce", nil),
			Product:   ProductApex,
			Namespace: "Metadata",
			TypeName:  "FeedLayoutComponentType",
			Kind:      KindProperty,
		},
	} {
		if packetOwnsRow(packet, row) {
			t.Fatalf("VisualforceComponents packet should not own %s", row.SurfaceID)
		}
	}
}
