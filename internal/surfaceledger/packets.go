package surfaceledger

import (
	"fmt"
	"sort"
	"strings"
)

type AreaPacket struct {
	Name                 string   `json:"name"`
	Title                string   `json:"title"`
	Owner                string   `json:"owner"`
	RowFilter            string   `json:"rowFilter"`
	DependsOn            []string `json:"dependsOn"`
	MayRunInParallelWith []string `json:"mayRunInParallelWith"`
	SharedFiles          []string `json:"sharedFiles"`
	ExclusiveFiles       []string `json:"exclusiveFiles"`
	AllowedFiles         []string `json:"allowedFiles"`
	BlockedFiles         []string `json:"blockedFiles"`
	RequiredFixtures     []string `json:"requiredFixtures"`
	FocusedTests         []string `json:"focusedTests"`
	DoneCriteria         []string `json:"doneCriteria"`
	RatchetTarget        string   `json:"ratchetTarget"`
	AreaRatchetCommand   string   `json:"areaRatchetCommand"`
}

func AreaRegistry() []AreaPacket {
	packets := []AreaPacket{
		{
			Name:      "Ledger.Identity",
			Title:     "Ledger Identity",
			Owner:     "internal/surfaceledger",
			RowFilter: "identity joins across docs/org/glade/evidence",
			DependsOn: []string{"source shelf green"},
			MayRunInParallelWith: []string{
				"External.MarketingCloud.AMPscript",
				"AI.Agentforce",
			},
			SharedFiles:        []string{"docs/generated/salesforce/**"},
			ExclusiveFiles:     []string{"internal/surfaceledger/**"},
			AllowedFiles:       []string{"internal/surfaceledger/**", "internal/gladecli/compat_surface_command.go", "docs/plans/**"},
			BlockedFiles:       []string{"internal/vm/**", "internal/dml/**", "internal/soql/**"},
			RequiredFixtures:   []string{"focused ledger rows for FeatureManagement, Database.Batchable, and ApexPages.Component"},
			FocusedTests:       []string{"go test ./internal/surfaceledger"},
			DoneCriteria:       []string{"false split rows are joined before feature packets start"},
			RatchetTarget:      "paired missing/stale rows decrease for identity examples",
			AreaRatchetCommand: "go test ./internal/surfaceledger && go run ./cmd/glade compat surface refresh --docs \"$GLADE_SALESFORCE_DOCS_SOURCE\" --tooling-completions testdata/generated/tooling_system_symbols.json.gz --out \"$(mktemp -d)\"",
		},
		{
			Name:      "Core.Runtime.System.FeatureManagement",
			Title:     "FeatureManagement",
			Owner:     "internal/vm",
			RowFilter: "product=apex namespace=System typeName=FeatureManagement",
			DependsOn: []string{"Ledger.Identity"},
			MayRunInParallelWith: []string{
				"External.MarketingCloud.AMPscript",
				"Integration.GraphQL",
				"Integration.PubSub",
			},
			SharedFiles:        []string{"internal/capability/**", "docs/generated/salesforce/**"},
			ExclusiveFiles:     []string{"internal/vm/platform_feature_management.go", "internal/vm/platform_test.go"},
			AllowedFiles:       []string{"internal/vm/**", "internal/capability/**", "internal/compat/**", "docs/**"},
			BlockedFiles:       []string{"internal/dml/**", "internal/soql/**", "internal/server/**"},
			RequiredFixtures:   []string{"FeatureManagement methods fixture or explicit unsupported fixture"},
			FocusedTests:       []string{"go test ./internal/vm ./internal/capability ./internal/repoguard"},
			DoneCriteria:       []string{"supported methods have behavior evidence", "unsupported behavior is explicit"},
			RatchetTarget:      "FeatureManagement missing-shape and missing-evidence rows do not increase",
			AreaRatchetCommand: "go run ./cmd/glade compat surface check --ledger \"$tmp/SURFACE_LEDGER.json\" --max-parser-failures 0",
		},
		{
			Name:                 "Core.Runtime.Database.Batchable",
			Title:                "Database Batchable",
			Owner:                "internal/vm async runtime",
			RowFilter:            "product=apex namespace=System typeName=Database|Database.Batchable|Database.BatchableContext",
			DependsOn:            []string{"Ledger.Identity", "Tests.AsyncAndIsolation"},
			MayRunInParallelWith: []string{"Data.Reference.ObjectsFields", "External.MarketingCloud.AMPscript"},
			SharedFiles:          []string{"internal/capability/**", "docs/generated/salesforce/**"},
			ExclusiveFiles:       []string{"internal/vm/async*_runtime.go", "internal/vm/test_support_runtime.go"},
			AllowedFiles:         []string{"internal/vm/**", "internal/apextest/**", "internal/capability/**", "internal/compat/**", "docs/**"},
			BlockedFiles:         []string{"internal/server/**"},
			RequiredFixtures:     []string{"Database.executeBatch lifecycle fixture"},
			FocusedTests:         []string{"go test ./internal/vm ./internal/apextest ./internal/repoguard"},
			DoneCriteria:         []string{"batch lifecycle has fixture evidence", "AsyncApexJob state is explicit"},
			RatchetTarget:        "Database.Batchable missing-shape and missing-evidence rows do not increase",
			AreaRatchetCommand:   "go run ./cmd/glade compat surface check --ledger \"$tmp/SURFACE_LEDGER.json\" --max-parser-failures 0",
		},
		{
			Name:                 "Data.Runtime.SchemaDescribe",
			Title:                "Schema Describe",
			Owner:                "internal/vm describe runtime",
			RowFilter:            "product=apex namespace=Schema or SObject describe rows",
			DependsOn:            []string{"Data.Reference.ObjectsFields"},
			MayRunInParallelWith: []string{"Core.Runtime.System.FeatureManagement"},
			SharedFiles:          []string{"internal/schema/**", "internal/capability/**", "docs/generated/salesforce/**"},
			ExclusiveFiles:       []string{"internal/vm/describe_runtime.go", "internal/sobject/**"},
			AllowedFiles:         []string{"internal/vm/**", "internal/schema/**", "internal/sobject/**", "internal/capability/**", "docs/**"},
			BlockedFiles:         []string{"internal/server/**"},
			RequiredFixtures:     []string{"Schema describe fixture"},
			FocusedTests:         []string{"go test ./internal/vm ./internal/schema ./internal/sobject ./internal/repoguard"},
			DoneCriteria:         []string{"describe shape and behavior evidence stay separated"},
			RatchetTarget:        "Schema describe missing rows do not increase",
			AreaRatchetCommand:   "go run ./cmd/glade compat surface check --ledger \"$tmp/SURFACE_LEDGER.json\" --max-parser-failures 0",
		},
	}
	packets = append(packets,
		genericArea("Core.Runtime.SystemAndStdlib", "System and Stdlib", "internal/vm", "product=apex namespace=System typeName!=FeatureManagement|Database", []string{"internal/vm/**", "internal/capability/**"}),
		genericArea("Query.Runtime.SOQLSOSL", "SOQL SOSL", "internal/soql", "product=apex source=soql-sosl", []string{"internal/soql/**", "internal/sema/**", "internal/vm/soql_runtime.go"}),
		genericArea("Data.Reference.ObjectsFields", "Objects and Fields", "internal/schema", "product=object-reference|field-reference", []string{"internal/schema/**", "internal/storage/standard_fields.go"}),
		genericArea("Data.Runtime.SOQL", "SOQL Runtime", "internal/vm soql runtime", "area=data kind=query", []string{"internal/vm/soql_runtime.go", "internal/soql/**", "internal/storage/**"}),
		genericArea("Data.Runtime.DML", "DML Runtime", "internal/dml", "area=data kind=dml", []string{"internal/dml/**", "internal/vm/dml_runtime.go", "internal/storage/**"}),
		genericArea("Tests.AsyncAndIsolation", "Async and Isolation", "internal/apextest", "area=tests async|isolation", []string{"internal/apextest/**", "internal/vm/async*_runtime.go", "internal/storage/isolation_journal.go"}),
		genericArea("UI.ApexPagesControllers", "ApexPages Controllers", "internal/vm ApexPages", "product=apex namespace=ApexPages", []string{"internal/vm/platform_apexpages*.go", "internal/visualforce/**"}),
		genericArea("UI.VisualforceComponents", "Visualforce Components", "internal/visualforce", "product=visualforce", []string{"internal/visualforce/**", "internal/vm/platform_apexpages*.go"}),
		genericArea("UI.LWCModules", "LWC Modules", "internal/server or explicit unsupported", "product=lwc", []string{"internal/server/**", "internal/capability/**"}),
		genericArea("UI.AuraComponents", "Aura Components", "internal/visualforce or explicit unsupported", "product=lightning", []string{"internal/visualforce/**", "internal/capability/**"}),
		genericArea("UI.UIAPI", "UI API", "internal/server", "product=ui-api", []string{"internal/server/**", "internal/storage/**", "internal/schema/**"}),
		genericArea("Server.RESTResources", "REST Resources", "internal/server", "product=rest", []string{"internal/server/**", "internal/storage/**"}),
		genericArea("Server.ToolingObjects", "Tooling Objects", "internal/server", "product=tooling", []string{"internal/server/**", "internal/storage/**"}),
		genericArea("Integration.GraphQL", "GraphQL", "internal/server or explicit unsupported", "salesforceSurfaceFamily=graphql-api", []string{"internal/server/**", "internal/capability/**"}),
		genericArea("Integration.PubSub", "Pub/Sub", "internal/server or explicit unsupported", "salesforceSurfaceFamily=pub-sub-api", []string{"internal/server/**", "internal/capability/**"}),
		genericArea("Integration.BulkAPI", "Bulk API", "internal/server", "product=bulk-api", []string{"internal/server/**", "internal/storage/**"}),
		genericArea("Integration.MetadataAPI", "Metadata API", "internal/server metadata", "product=metadata-api", []string{"internal/server/**", "internal/metadata/**"}),
		genericArea("Integration.SOAPAPI", "SOAP API", "internal/server", "product=soap-api", []string{"internal/server/**", "internal/storage/**"}),
		genericArea("Integration.StreamingAPI", "Streaming API", "internal/server events", "product=streaming-api", []string{"internal/server/**", "internal/vm/async*_runtime.go"}),
		genericArea("Integration.SalesforceConnect.AmazonRDS", "Salesforce Connect Amazon RDS", "external connector explicit unsupported", "salesforceSurfaceFamily=sf-connect-amazon-rds", []string{"internal/capability/**", "docs/**"}),
		genericArea("Platform.Events", "Platform Events", "event metadata and async hooks", "product=platform-events", []string{"internal/vm/**", "internal/storage/**", "internal/schema/**"}),
		genericArea("AI.Agentforce", "Agentforce", "external product explicit unsupported", "salesforceSurfaceFamily=agentforce", []string{"internal/capability/**", "docs/**"}),
		genericArea("External.MarketingCloud.AMPscript", "Marketing Cloud AMPscript", "external product explicit unsupported", "salesforceSurfaceFamily=marketing-cloud-ampscript", []string{"internal/capability/**", "docs/**"}),
		genericArea("External.MarketingCloud.Handlebars", "Marketing Cloud Handlebars", "external product explicit unsupported", "salesforceSurfaceFamily=handlebars-for-marketing-cloud-next", []string{"internal/capability/**", "docs/**"}),
		genericArea("ConnectApi.PassiveDTOs", "ConnectApi Passive DTOs", "internal/vm passive members", "product=connect-rest-api passive DTO rows", []string{"internal/vm/platform_passive_members.go", "internal/capability/**"}),
	)
	return packets
}

func genericArea(name, title, owner, filter string, allowed []string) AreaPacket {
	exclusive := append([]string(nil), allowed...)
	return AreaPacket{
		Name:                 name,
		Title:                title,
		Owner:                owner,
		RowFilter:            filter,
		DependsOn:            []string{"source shelf green", "Ledger.Identity"},
		MayRunInParallelWith: []string{"Core.Runtime.System.FeatureManagement", "External.MarketingCloud.AMPscript"},
		SharedFiles:          []string{"docs/generated/salesforce/**", "testdata/generated/tooling_system_symbols.json.gz"},
		ExclusiveFiles:       exclusive,
		AllowedFiles:         append([]string{}, allowed...),
		BlockedFiles:         []string{"unrelated external docs shelves", "corpus-specific runtime exceptions"},
		RequiredFixtures:     []string{title + " focused compatibility fixture or explicit unsupported fixture"},
		FocusedTests:         []string{"go test ./internal/repoguard"},
		DoneCriteria:         []string{"shape, behavior, evidence, capability/docs, and refresh/check are reported in order"},
		RatchetTarget:        name + " missing rows do not increase",
		AreaRatchetCommand:   "go run ./cmd/glade compat surface check --ledger \"$tmp/SURFACE_LEDGER.json\" --max-parser-failures 0",
	}
}

func AreaPacketByName(name string) (AreaPacket, bool) {
	for _, packet := range AreaRegistry() {
		if packet.Name == name || strings.EqualFold(packet.Title, name) {
			return packet, true
		}
	}
	return AreaPacket{}, false
}

func PacketRows(ledger SurfaceLedger, packet AreaPacket) []SurfaceLedgerRow {
	var rows []SurfaceLedgerRow
	for _, row := range ledger.Rows {
		if packetOwnsRow(packet, row) {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Priority != rows[j].Priority {
			return rows[i].Priority < rows[j].Priority
		}
		return rows[i].SurfaceID < rows[j].SurfaceID
	})
	return rows
}

func PacketMarkdown(ledger SurfaceLedger, packet AreaPacket) string {
	rows := PacketRows(ledger, packet)
	var b strings.Builder
	fmt.Fprintf(&b, "# Salesforce Surface Packet: %s\n\n", packet.Name)
	fmt.Fprintf(&b, "- Area: %s\n", packet.Name)
	fmt.Fprintf(&b, "- Owner: %s\n", packet.Owner)
	fmt.Fprintf(&b, "- Ledger row filter: `%s`\n", packet.RowFilter)
	fmt.Fprintf(&b, "- Ratchet target: %s\n", packet.RatchetTarget)
	fmt.Fprintln(&b)
	writePacketList(&b, "dependsOn", packet.DependsOn)
	writePacketList(&b, "mayRunInParallelWith", packet.MayRunInParallelWith)
	writePacketList(&b, "sharedFiles", packet.SharedFiles)
	writePacketList(&b, "exclusiveFiles", packet.ExclusiveFiles)
	writePacketList(&b, "Allowed files", packet.AllowedFiles)
	writePacketList(&b, "Blocked files", packet.BlockedFiles)
	writePacketList(&b, "Required fixtures", packet.RequiredFixtures)
	writePacketList(&b, "Focused tests", packet.FocusedTests)
	writePacketList(&b, "Done criteria", packet.DoneCriteria)
	fmt.Fprintln(&b, "## Rows To Explain First")
	fmt.Fprintln(&b)
	if len(rows) == 0 {
		fmt.Fprintln(&b, "- No rows matched this packet in the current ledger.")
	} else {
		for _, row := range rows {
			fmt.Fprintf(&b, "- `%s` gap=%s bucket=%s priority=%d\n", row.SurfaceID, row.GapClass, row.Bucket, row.Priority)
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Baseline Command")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```bash")
	fmt.Fprintln(&b, "tmp=\"$(mktemp -d)\"")
	fmt.Fprintln(&b, "go run ./cmd/glade compat surface refresh \\")
	fmt.Fprintln(&b, "  --docs \"$GLADE_SALESFORCE_DOCS_SOURCE\" \\")
	fmt.Fprintln(&b, "  --tooling-completions testdata/generated/tooling_system_symbols.json.gz \\")
	fmt.Fprintln(&b, "  --out \"$tmp\"")
	fmt.Fprintln(&b, "```")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Area ratchet command")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```bash")
	fmt.Fprintln(&b, packet.AreaRatchetCommand)
	fmt.Fprintln(&b, "```")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Handoff Format")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Report focused tests, fixture command, surface refresh, area ratchet command, before counts, after counts, and next top row.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Standard Validation Block")
	fmt.Fprintln(&b)
	for _, item := range []string{
		"focused tests run",
		"fixture command run",
		"surface refresh run",
		"area ratchet command run",
		"before counts",
		"after counts",
		"next top row",
		"go test ./internal/repoguard after code changes",
	} {
		fmt.Fprintf(&b, "- %s:\n", item)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Docs Defect Path")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "If a docs row is missing or malformed, choose one path before runtime work:")
	for _, item := range []string{
		"re-scrape docs",
		"copy improved docs into the external docs source",
		"patch the docs parser to read existing docs correctly",
		"add a small checked fixture if public docs are ambiguous",
	} {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Reviewer Checklist")
	fmt.Fprintln(&b)
	for _, item := range []string{
		"no corpus-specific runtime hacks",
		"public Salesforce behavior cited by docs or fixture",
		"shape and behavior are not claimed without evidence",
		"packet area did not expand during work",
		"generated docs are updated when capability changes",
	} {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Breadth Work Order")
	fmt.Fprintln(&b)
	for _, item := range []string{
		"Ledger.Identity",
		"FeatureManagement",
		"Database.Batchable",
		"Schema.Describe",
		"ApexPages.Controllers",
		"REST.Resources",
		"Tooling.Objects",
	} {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	return b.String()
}

func packetOwnsRow(packet AreaPacket, row SurfaceLedgerRow) bool {
	switch packet.Name {
	case "Ledger.Identity":
		return row.GapClass == GapDocsOrgMismatch || row.GapClass == GapStaleGladeShape || row.GapClass == GapSignatureChanged
	case "Core.Runtime.System.FeatureManagement":
		return row.Product == ProductApex && row.Namespace == "System" && row.TypeName == "FeatureManagement"
	case "Core.Runtime.Database.Batchable":
		return row.Product == ProductApex && row.Namespace == "System" && (row.TypeName == "Database" || strings.Contains(row.TypeName, "Batchable"))
	case "Data.Runtime.SchemaDescribe":
		return row.Product == ProductApex && (row.Namespace == "Schema" || strings.Contains(row.TypeName, "Describe"))
	default:
		return false
	}
}

func writePacketList(b *strings.Builder, title string, values []string) {
	fmt.Fprintf(b, "## %s\n\n", title)
	if len(values) == 0 {
		fmt.Fprintln(b, "- none")
		return
	}
	for _, value := range values {
		fmt.Fprintf(b, "- `%s`\n", value)
	}
	fmt.Fprintln(b)
}
