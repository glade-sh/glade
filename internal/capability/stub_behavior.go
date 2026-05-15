package capability

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/typesys"
)

const StubBehaviorSchemaVersion = 1

type StubBehaviorStatus string

const (
	StubBehaviorImplemented    StubBehaviorStatus = "implemented"
	StubBehaviorPassiveDefault StubBehaviorStatus = "passive-default"
	StubBehaviorUnsupported    StubBehaviorStatus = "unsupported"
	StubBehaviorUnknown        StubBehaviorStatus = "unknown"
)

type StubBehaviorReport struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Target        string              `json:"target"`
	Totals        StubBehaviorTotals  `json:"totals"`
	Entries       []StubBehaviorEntry `json:"entries"`
}

type StubBehaviorTotals struct {
	Entries        int            `json:"entries"`
	Types          int            `json:"types"`
	Members        int            `json:"members"`
	Implemented    int            `json:"implemented"`
	PassiveDefault int            `json:"passiveDefault"`
	Unsupported    int            `json:"unsupported"`
	Unknown        int            `json:"unknown"`
	ByStatus       map[string]int `json:"byStatus"`
}

type StubBehaviorEntry struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Member     string             `json:"member,omitempty"`
	Kind       string             `json:"kind"`
	Static     bool               `json:"static,omitempty"`
	ReturnType string             `json:"returnType,omitempty"`
	Parameters []string           `json:"parameters,omitempty"`
	Status     StubBehaviorStatus `json:"status"`
	Evidence   []string           `json:"evidence,omitempty"`
	Notes      string             `json:"notes,omitempty"`
}

func BuildStubBehaviorReport() StubBehaviorReport {
	evidence := buildStubBehaviorEvidence()
	report := StubBehaviorReport{
		SchemaVersion: StubBehaviorSchemaVersion,
		Target:        "standard platform stub behavior",
	}
	for _, symbol := range typesys.StandardPlatformSymbols() {
		typeName := stubBehaviorTypeName(symbol)
		typeEntry := StubBehaviorEntry{
			ID:     typeName,
			Type:   typeName,
			Kind:   string(symbol.Kind),
			Status: StubBehaviorPassiveDefault,
			Notes:  "standard platform type is available to parser and semantic analysis",
		}
		if match := evidence.lookup(typeName, ""); match != nil {
			typeEntry.Status = match.status
			typeEntry.Evidence = match.evidence
			typeEntry.Notes = match.notes
		}
		report.Entries = append(report.Entries, typeEntry)
		for _, member := range symbol.Members {
			report.Entries = append(report.Entries, buildStubBehaviorMemberEntry(symbol, typeName, member, evidence))
		}
	}
	sort.Slice(report.Entries, func(i, j int) bool {
		return report.Entries[i].ID < report.Entries[j].ID
	})
	report.Totals = countStubBehaviorTotals(report.Entries)
	return report
}

func WriteStubBehaviorJSON(w io.Writer, report StubBehaviorReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteStubBehaviorMarkdown(w io.Writer, report StubBehaviorReport) error {
	if _, err := fmt.Fprintln(w, "# Stub Behavior Manifest"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\nTarget: %s\n", report.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\n- Entries: %d\n", report.Totals.Entries); err != nil {
		return err
	}
	for _, status := range []StubBehaviorStatus{StubBehaviorImplemented, StubBehaviorPassiveDefault, StubBehaviorUnsupported, StubBehaviorUnknown} {
		if _, err := fmt.Fprintf(w, "- %s: %d\n", status, report.Totals.ByStatus[string(status)]); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w, "\n| ID | Kind | Status | Evidence |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | --- | --- |"); err != nil {
		return err
	}
	for _, entry := range report.Entries {
		if _, err := fmt.Fprintf(w, "| `%s` | %s | `%s` | %s |\n", entry.ID, entry.Kind, entry.Status, strings.Join(entry.Evidence, "; ")); err != nil {
			return err
		}
	}
	return nil
}

func buildStubBehaviorMemberEntry(symbol typesys.TypeSymbol, typeName string, member typesys.MemberSymbol, evidence stubBehaviorEvidence) StubBehaviorEntry {
	entry := StubBehaviorEntry{
		ID:         stubBehaviorMemberID(typeName, member),
		Type:       typeName,
		Member:     member.Name,
		Kind:       string(member.Kind),
		Static:     stubBehaviorMemberStatic(member),
		ReturnType: member.Type,
		Parameters: stubBehaviorParameterTypes(member.Parameters),
		Status:     StubBehaviorUnknown,
		Notes:      "no runtime behavior evidence recorded yet",
	}
	if member.Kind == apexast.DeclarationConstructor || member.Kind == apexast.DeclarationProperty {
		entry.Status = StubBehaviorPassiveDefault
		entry.Notes = "shape is available; behavior is passive/default unless runtime code special-cases it"
	}
	if status, notes, ok := genericStubBehaviorMemberStatus(symbol, member); ok {
		entry.Status = status
		entry.Notes = notes
	}
	if match := evidence.lookup(typeName, member.Name); match != nil {
		entry.Status = match.status
		entry.Evidence = match.evidence
		entry.Notes = match.notes
		if status, notes, ok := localStubBehaviorEvidenceOverride(symbol, member); ok {
			entry.Status = status
			entry.Notes = notes
		}
	} else if member.Kind == apexast.DeclarationConstructor {
		if match := evidence.lookup(typeName, ""); match != nil {
			entry.Status = match.status
			entry.Evidence = match.evidence
			entry.Notes = match.notes
		}
	}
	if entry.Status == StubBehaviorUnknown && member.Kind == apexast.DeclarationMethod {
		entry.Status = StubBehaviorUnsupported
		entry.Notes = "generated platform method has shape only; local runtime should reject it unless implemented or allowlisted as passive DTO behavior"
	}
	return entry
}

func localStubBehaviorEvidenceOverride(symbol typesys.TypeSymbol, member typesys.MemberSymbol) (StubBehaviorStatus, string, bool) {
	if member.Kind != apexast.DeclarationMethod {
		return "", "", false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "Schema":
		if name == "describedatacategorygroups" || name == "describedatacategorygroupstructures" {
			return StubBehaviorImplemented, "local runtime returns deterministic metadata-backed data category describe results", true
		}
	case "Search":
		if name == "query" || name == "find" || name == "suggest" {
			return StubBehaviorImplemented, "local runtime models Search over fixed test search results and empty suggestion DTOs", true
		}
	}
	return "", "", false
}

func genericStubBehaviorMemberStatus(symbol typesys.TypeSymbol, member typesys.MemberSymbol) (StubBehaviorStatus, string, bool) {
	if member.Kind != apexast.DeclarationMethod && member.Kind != apexast.DeclarationConstructor {
		return "", "", false
	}
	if genericObjectBehaviorMethod(member) {
		return StubBehaviorImplemented, "generic Object method is handled by the VM for runtime values", true
	}
	if symbol.Kind == apexast.DeclarationEnum && genericEnumBehaviorMethod(member) {
		return StubBehaviorImplemented, "generic enum method is handled by the VM for enum values", true
	}
	if limitBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "Limits getter is handled by the VM limit counter/default-cap surface", true
	}
	if stringBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "String method is handled by the VM string stdlib surface", true
	}
	if primitiveFieldAddErrorBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "primitive addError overload is handled by the VM for SObject field-context receivers", true
	}
	if corePlatformBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "core platform method is handled by the VM stdlib/runtime surface", true
	}
	if xmlStreamReaderBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "XmlStreamReader cursor and accessor method is handled by the VM XML stream surface", true
	}
	if domXmlNodeBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "Dom.XmlNode method is handled by the VM DOM node surface", true
	}
	if visualEditorDynamicPickListRowsBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "VisualEditor.DynamicPickListRows method is handled by the VM local rows surface", true
	}
	if compressionZipBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "compression ZIP archive type is handled by the VM local ZIP surface", true
	}
	if waveQueryBuilderBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "wave query builder node/projection method is handled by the VM local builder surface", true
	}
	if contextIndustriesBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "Context.IndustriesContext map passthrough/no-op method is handled by the VM local context surface", true
	}
	if orgInstrumentationBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "OrgInstrumentation metric/span/context method is handled by the VM local no-op instrumentation surface", true
	}
	if userProvisioningBatchableBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "UserProvisioning batchable helper method is handled by the VM local no-op/default surface", true
	}
	if callbackInterfaceBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "callback/interface method is supplied by user Apex and dispatched through the VM when the local lifecycle invokes or user code calls it", true
	}
	if localMockHarnessBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "test/mock harness method is handled by the VM local deterministic/no-op surface", true
	}
	if socialInboundDefaultHandlerBehaviorMethod(symbol, member) {
		return StubBehaviorUnsupported, "Social inbound default-handler method depends on the Social Customer Service case/persona lifecycle and remains explicit unsupported until modeled", true
	}
	if databaseResultDTOBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "Database/Approval result DTO accessor is handled by the VM result object surface", true
	}
	if connectAPITestFixtureSetterBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "ConnectApi setTest fixture setter is accepted locally without calling ConnectApi services", true
	}
	if connectAPITestFixtureTargetBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "ConnectApi method returns a matching local setTest fixture when provided; live service calls remain explicitly unsupported without a fixture", true
	}
	if connectAPIReadOnlyHarnessBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "ConnectApi read/search method returns a deterministic local empty typed DTO/result without live service mutation", true
	}
	if ideasFindSimilarBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "Ideas.findSimilar returns a typed empty List<Id> without enabling Ideas reply/read-state service surfaces", true
	}
	if pushUpgradeCustomizationRepositoryBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "PushUpgradeCustomizationRepository uses VM-local deterministic in-memory repository state", true
	}
	if quickActionDescribeBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "QuickAction describe/template/default methods return local read-only metadata/default DTOs without performing action side effects", true
	}
	if localServiceHarnessBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "safe platform service method is handled by a deterministic local no-op/default-result harness; live cloud side effects remain fenced", true
	}
	if appLauncherControllerBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "applauncher controller helper returns deterministic local URLs, booleans, empty provider/field DTO lists, or no-op form redirects without performing authentication or registration services", true
	}
	if industryControllerHarnessBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "industry package controller method returns deterministic local empty/default DTOs or conservative access booleans without invoking managed services", true
	}
	if industryControllerUnsupportedBehaviorMethod(symbol, member) {
		return StubBehaviorUnsupported, "industry package mutation, booking, or transaction service remains explicitly unsupported", true
	}
	if explicitlyUnsupportedCoreBehaviorMethod(symbol, member) {
		return StubBehaviorUnsupported, "local runtime returns an explicit unsupported-feature error for this platform surface", true
	}
	if slackTestHarnessBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "Slack test-harness state/session method is handled locally without Slack transport", true
	}
	if slackLocalClientHarnessBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "Slack client auth/read/info method returns a deterministic local DTO without external transport", true
	}
	if slackPassiveBehaviorMethod(symbol, member) {
		return StubBehaviorPassiveDefault, "Slack DTO, builder, or mock placeholder method returns local passive/default values without performing Slack service calls", true
	}
	if generatedDTOCollectionBehaviorMethod(symbol, member) {
		return StubBehaviorPassiveDefault, "passive generated DTO collection wrapper exposes local empty collection semantics", true
	}
	if generatedOptionalWrapperBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "CartExtension optional wrapper empty/of/isPresent/get is handled by the VM optional-wrapper surface", true
	}
	if cartExtensionTestUtilBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "CartExtension test utility creates local Cart DTOs without running Commerce services", true
	}
	if sfsqlqueryHarnessBehaviorMethod(symbol, member) {
		return StubBehaviorImplemented, "sfsqlquery test harness and mock row iterator methods are handled locally without executing SQL service calls", true
	}
	if generatedDTOAccessorBehaviorMethod(symbol, member) {
		return StubBehaviorPassiveDefault, "passive generated DTO getter/setter returns or mutates the matching property when available, otherwise uses a typed default", true
	}
	if generatedDTOBehaviorType(symbol) && generatedDTOCollectionMethod(member) {
		return StubBehaviorPassiveDefault, "passive generated DTO collection method returns an empty typed collection", true
	}
	if generatedDTOBehaviorType(symbol) || generatedTopLevelPassiveBehaviorType(symbol) {
		return StubBehaviorPassiveDefault, "passive generated platform method returns a typed default value unless runtime code special-cases it", true
	}
	return "", "", false
}

func pushUpgradeCustomizationRepositoryBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod || !strings.EqualFold(stubBehaviorTypeName(symbol), "PushUpgradeCustomizationRepository") {
		return false
	}
	switch strings.ToLower(member.Name) {
	case "create", "deletebyid", "deletebyindex",
		"getcustomupgradeallowedforid", "getcustomupgradeallowedforindex",
		"getcustomupgradetypeforid", "getcustomupgradetypeforindex",
		"setcustomupgradeallowedforid", "setcustomupgradeallowedforindex":
		return true
	default:
		return false
	}
}

func appLauncherControllerBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	name := strings.ToLower(member.Name)
	switch stubBehaviorTypeName(symbol) {
	case "applauncher.LoginFormController":
		switch name {
		case "getforgotpasswordurl", "getisselfregistrationenabled", "getisusernamepasswordenabled",
			"getloginrightframeurl", "getselfregistrationurl", "getusernamepasswordselfregenabled",
			"login", "logingetpagerefurl", "setexperienceid":
			return true
		default:
			return false
		}
	case "applauncher.SelfRegisterController":
		switch name {
		case "commonselfregistergetredirecturl", "getextrafields", "isvalidpassword",
			"selfregistergetredirecturl", "setexperienceid":
			return true
		default:
			return false
		}
	case "applauncher.SocialLoginController":
		switch name {
		case "getauthproviders", "getsamlproviders":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func sfsqlqueryHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod && member.Kind != apexast.DeclarationConstructor {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "sfsqlquery.QueryHandle":
		return member.Kind == apexast.DeclarationConstructor ||
			name == "create" || name == "tostring" || name == "withoffset" || name == "withworkloadname"
	case "sfsqlquery.SqlStatement":
		return member.Kind == apexast.DeclarationConstructor ||
			name == "create" || name == "tostring" || name == "withworkloadname"
	case "sfsqlquery.SqlRowIterator":
		switch name {
		case "cancel", "getcolumnnames", "getmetadata", "getqueryid", "hasnext", "iterator", "next", "tostring":
			return true
		default:
			return member.Kind == apexast.DeclarationConstructor
		}
	case "sfsqlquery.SqlTester":
		switch name {
		case "clearmocks", "enqueuemockrows", "isrunningtest", "setmockmetadata", "setmockrows":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func quickActionDescribeBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	name := strings.ToLower(member.Name)
	switch stubBehaviorTypeName(symbol) {
	case "QuickAction":
		switch name {
		case "describeavailablequickactions", "describequickactions", "retrievequickactiontemplate", "retrievequickactiontemplates",
			"performquickaction", "performquickactions":
			return true
		default:
			return false
		}
	case "Test":
		return name == "newsendemailquickactiondefaults"
	case "QuickAction.SendEmailQuickActionDefaults":
		return strings.HasPrefix(name, "get") || strings.HasPrefix(name, "set")
	default:
		return false
	}
}

func localServiceHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	name := strings.ToLower(member.Name)
	switch stubBehaviorTypeName(symbol) {
	case "ApptBooking.WaitlistController":
		return name == "call" || name == "invokemethod"
	case "BcpProvisionService", "DistributedLedgerService":
		return name == "enablec2c"
	case "BusRuleDtMig.DecisionTableMigrationService":
		return name == "migratedecisiontables"
	case "BusinessRule.CalculationMatrixMigrationService", "BusinessRule.CalculationProcedureMigrationService",
		"BusinessRule.DecisionMatrixRowMigratorService":
		return name == "migrate"
	case "data_mask.DataMaskIntegrationUtil":
		switch name {
		case "iscoreallowed", "islibraryinuse":
			return true
		default:
			return false
		}
	case "Datacloud.FindDuplicates":
		return name == "findduplicates"
	case "Datacloud.FindDuplicatesByIds":
		return name == "findduplicatesbyids"
	case "DomainParser":
		return name == "parse"
	case "KbManagement.PublishingService":
		switch name {
		case "archiveonlinearticle", "assigndraftarticletask", "assigndrafttranslationtask",
			"cancelscheduledarchivingofarticle", "cancelscheduledpublicationofarticle",
			"completetranslation", "editarchivedarticle", "editonlinearticle",
			"editpublishedtranslation", "publisharticle", "restoreoldversion",
			"scheduleforpublication", "settranslationtoincomplete", "submitfortranslation":
			return true
		default:
			return false
		}
	case "Packaging":
		return name == "getcurrentpackageid"
	case "RemoteObjectController":
		switch name {
		case "create", "del", "retrieve", "updat":
			return true
		default:
			return false
		}
	case "SupportPredictiveService":
		return name == "findsimilarcases"
	default:
		return false
	}
}

func industryControllerHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "healthcloudext.AppointmentBookingSelfService":
		switch name {
		case "findassets", "findavailableappointmentslots", "findavailableassetslots", "findproviders",
			"getgeolocationcoordinates", "logselfserviceinstrumentation", "validateslotstatusselfservice":
			return true
		default:
			return false
		}
	case "healthcloudext.IntegratedCareManagementApexHelper":
		switch name {
		case "checkentity", "checkobjectcreationaccess", "convertmultilinetohtml",
			"fetchsuggestedassessmentsforpatient", "getcarebarrierdetails", "getmaxaccesslevel",
			"getmru", "getpicklist", "getsoslsearch":
			return true
		default:
			return false
		}
	case "LoyaltyManagement.LoyaltyResources":
		switch name {
		case "getloyaltypromotionbasedonsalesforcecdp", "getloyaltypromotions", "getpointsbalance", "gettier":
			return true
		default:
			return false
		}
	case "LoyaltyManagement.WidgetCumulativePromotions", "LoyaltyManagement.WidgetMemberBadges", "LoyaltyManagement.WidgetReferMember":
		return name == "call"
	case "LoyaltyManagement.WidgetVisibility":
		return name == "checkvisibility"
	case "industries_docgen.DocGenPermsAndAccessChecksService":
		return strings.HasPrefix(name, "has") || strings.HasPrefix(name, "is")
	case "inventorypricing.GetInventoryPricing":
		switch name {
		case "createresponse", "getinventory", "getinventoryandpricing", "getpricing", "handleinventorypricingserviceexception", "processinput":
			return true
		default:
			return false
		}
	case "ime_mrm.EventManagementBudgetApi", "ime_mrm.EventManagementManagedEventApi",
		"ime_mrm.EventManagementParticipantApi", "ime_mrm.EventManagementProductApi", "ime_mrm.EventManagementSubjectApi":
		return strings.HasPrefix(name, "get")
	default:
		return false
	}
}

func industryControllerUnsupportedBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "healthcloudext.AppointmentBookingSelfService":
		switch name {
		case "bookselfserviceappointment", "cancelselfserviceappointment", "createpatient", "publisheventforpft":
			return true
		default:
			return false
		}
	case "LoyaltyManagement.LoyaltyResources":
		switch name {
		case "changetier", "creditpoints", "debitpoints", "issuevoucher",
			"transfermemberpointstogroups", "updateprogressforcumulativepromotionusage":
			return true
		default:
			return false
		}
	case "ime_mrm.EventManagementBudgetApi", "ime_mrm.EventManagementManagedEventApi",
		"ime_mrm.EventManagementParticipantApi", "ime_mrm.EventManagementProductApi", "ime_mrm.EventManagementSubjectApi":
		return strings.HasPrefix(name, "create") || strings.HasPrefix(name, "update") || strings.HasPrefix(name, "delete")
	case "RevSalesTrxn.PlaceSalesTransactionExecutor":
		return name == "execute"
	default:
		return false
	}
}

func generatedOptionalWrapperBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	if !strings.HasPrefix(typeName, "CartExtension.Optional") || strings.EqualFold(typeName, "CartExtension.OptionalNotCheckedException") {
		return false
	}
	name := strings.ToLower(member.Name)
	switch name {
	case "empty", "ispresent", "get":
		return len(member.Parameters) == 0
	case "of":
		return len(member.Parameters) == 1
	default:
		return false
	}
}

func cartExtensionTestUtilBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod || !strings.EqualFold(stubBehaviorTypeName(symbol), "CartExtension.CartTestUtil") {
		return false
	}
	switch strings.ToLower(member.Name) {
	case "createcart":
		return len(member.Parameters) == 0 || len(member.Parameters) == 1
	case "getcart":
		return len(member.Parameters) == 1
	default:
		return false
	}
}

func limitBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if !strings.EqualFold(stubBehaviorTypeName(symbol), "Limits") {
		return false
	}
	if member.Kind != apexast.DeclarationMethod || len(member.Parameters) != 0 {
		return false
	}
	switch strings.ToLower(member.Name) {
	default:
		return strings.HasPrefix(strings.ToLower(member.Name), "get")
	}
}

func corePlatformBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	typeKey := strings.ToLower(typeName)
	name := strings.ToLower(member.Name)
	if strings.HasPrefix(typeKey, "schema.") && (strings.HasPrefix(name, "get") || strings.HasPrefix(name, "is")) {
		return true
	}
	if typeKey == "schema.sobjecttype" && name == "newsobject" {
		return true
	}
	if typeKey == "comparable" && name == "compareto" {
		return true
	}
	if typeKey == "comparator" && name == "compare" {
		return true
	}
	if apexPagesBehaviorMethod(typeName, name) || messagingBehaviorMethod(typeName, name) {
		return true
	}
	if passiveAccessorBehaviorType(typeName) && accessorBehaviorMethod(member) {
		return true
	}
	if matcherBehaviorMethod(typeName, name) || xmlStreamWriterBehaviorMethod(typeName, name) ||
		calloutMockBehaviorMethod(typeName, name) || searchDTOBehaviorMethod(typeName, name) {
		return true
	}
	if standardExceptionBehaviorMethod(typeName, name) {
		return true
	}
	switch typeName {
	case "Callable":
		return name == "call"
	case "StubProvider":
		return name == "handlemethodcall"
	case "DataWeave.Script":
		return name == "createscript" || name == "execute"
	case "Formula":
		return name == "builder" || name == "recalculateformulas"
	case "BusinessHours":
		switch name {
		case "add", "addgmt", "diff", "iswithin", "nextstartdate":
			return true
		}
	case "Cases":
		switch name {
		case "generatethreadingmessageid", "getcaseidfromemailheaders", "getcaseidfromemailthreadid":
			return true
		}
	case "EmailMessages":
		switch name {
		case "getformattedthreadingtoken", "getrecordidfromemail":
			return true
		}
	case "Collator":
		return name == "getinstance" || name == "compare"
	case "CURRENCY":
		return name == "newinstance" || name == "format" || name == "formatamount"
	case "OrgLimits":
		switch name {
		case "getall", "getmap":
			return true
		}
	case "Id":
		switch name {
		case "getsobjecttype", "to15", "to18", "valueof":
			return true
		}
	case "SelectOption":
		return strings.HasPrefix(name, "get") || strings.HasPrefix(name, "set")
	case "FormulaRecalcResult":
		return name == "geterrors" || name == "getsobject" || name == "issuccess"
	case "SObjectAccessDecision":
		return name == "getrecords" || name == "getremovedfields" || name == "getmodifiedindexes"
	case "InstallContext":
		return name == "previousversion" || name == "ispush" || name == "installerid"
	case "SandboxContext":
		return name == "organizationid" || name == "sandboxid" || name == "sandboxname"
	case "RequestImpl":
		return name == "getcurrent"
	case "UIRequest":
		return name == "getcurrent" || name == "getrequestheader"
	case "QueueableContext", "QueueableContextImpl":
		return name == "getjobid"
	case "SchedulableContext":
		return name == "gettriggerid"
	case "RestResponse":
		return name == "addheader"
	case "Iterable":
		return name == "iterator"
	case "Iterator":
		return name == "hasnext" || name == "next"
	case "FormulaRecalcFieldError":
		return name == "getfieldname" || name == "getfielderror"
	case "Flow.Interview":
		return name == "createinterview" || name == "start" || name == "getvariablevalue"
	case "Canvas.CanvasLifecycleHandler":
		return name == "excludecontexttypes" || name == "onrender"
	case "Continuation":
		return name == "addhttprequest" || name == "getrequests" || name == "getresponse"
	case "ConnectApi.BaseEndpointExtension":
		return strings.HasPrefix(name, "before") || strings.HasPrefix(name, "after")
	case "sfsqlquery.SqlQueueable":
		switch name {
		case "cancel", "chainnextjob", "getcolumnnames", "getmetadata", "getpageoutput", "getqueryid", "getrows", "processdatachunk":
			return true
		}
	case "wave.Templates":
		switch name {
		case "cdpquerymetadata", "getsobject", "getsobjects", "gettemplate", "gettemplateconfig", "gettemplates":
			return true
		}
	case "LIST", "List", "Set", "Map":
		switch name {
		case "add", "addall", "addtorelationship", "clear", "contains", "containsall", "containskey",
			"get", "getaddedtorelationship", "getmarkedfordeletion", "getsobjecttype",
			"deepclone", "indexof", "isempty", "iterator", "put", "putall", "remov", "remove",
			"markfordelete", "removeall", "retainall", "set", "size", "sort", "keyset", "values":
			return true
		}
	case "UserInfo":
		switch name {
		case "getcurrentuvid", "getdefaultcurrency", "getfirstname", "getlanguage",
			"getlastname", "getlocale", "getname", "getorganizationid", "getorganizationname",
			"getprofileid", "getsessionid", "gettimezone", "getuitheme", "getuithemedisplayed",
			"getuseremail", "getuserid", "getusername", "getuserroleid", "getusertype",
			"haspackagelicense", "iscurrentuserlicensed", "iscurrentuserlicensedforpackage",
			"ismulticurrencyorganization":
			return true
		}
	case "UserManagement":
		switch name {
		case "formatphonenumber", "initselfregistration", "verifyselfregistration":
			return true
		}
	case "Site":
		switch name {
		case "getsiteid", "getbaseurl", "getbaserequesturl", "getbasesecureurl",
			"getbasecustomurl", "getdomain", "getname", "gettemplate", "getsitetype",
			"getsitetypelabel", "getpathprefix", "getadminemail", "getadminid",
			"getmasterlabel", "isregistrationenabled", "isloginenabled", "isvalidusername",
			"setexperienceid", "geterrormessage", "geterrordescription", "forgotpassword",
			"login", "changepassword", "validatepassword", "createexternaluser", "createportaluser",
			"getanalyticstrackingcode", "getbaseinsecureurl", "getcurrentsiteurl",
			"getcustomwebaddress", "getexperienceid", "getoriginalurl",
			"getpasswordpolicystatement", "getprefix", "ispasswordexpired":
			return true
		}
	case "Network":
		switch name {
		case "getnetworkid", "getloginurl", "communitieslanding", "forwardtoauthpage",
			"getlogouturl", "getselfregurl":
			return true
		}
	case "Communities":
		switch name {
		case "communitieslanding", "forwardtoauthpage", "getcss", "internallogin", "login":
			return true
		}
	case "AsyncInfo":
		switch name {
		case "hasmaxstackdepth", "getcurrentqueueablestackdepth", "getmaximumqueueablestackdepth", "getminimumqueueabledelayinminutes":
			return true
		}
	case "Assert":
		switch name {
		case "areequal", "arenotequal", "istrue", "isfalse", "isnull", "isnotnull",
			"isinstanceoftype", "isnotinstanceoftype", "fail":
			return true
		}
	case "Apex.Stack":
		switch name {
		case "empty", "peek", "pop", "push":
			return true
		}
	case "EventBus":
		return name == "publish" || name == "getoperationid"
	case "Cache.Org", "Cache.Session", "Cache.Partition", "Cache.OrgPartition", "Cache.SessionPartition",
		"Cache.SecondaryKeyApi",
		"cache.Org", "cache.Session", "cache.Partition", "cache.OrgPartition", "cache.SessionPartition",
		"cache.SecondaryKeyApi":
		return cacheBehaviorMethod(name)
	case "FeatureManagement":
		return strings.HasPrefix(name, "checkpackage") || strings.HasPrefix(name, "setpackage") || name == "changeprotection"
	case "Packaging":
		return name == "getcurrentpackageid"
	case "DomainParser":
		return name == "parse"
	case "Datacloud.FindDuplicates":
		return name == "findduplicates"
	case "Datacloud.FindDuplicatesByIds":
		return name == "findduplicatesbyids"
	case "NLPPredictions.FAQPrediction":
		return name == "predict"
	case "NLPPredictions.PredictionHandler":
		return name == "handlepredictionrequest" || name == "handlepredictionresponse"
	case "Security":
		return name == "stripinaccessible"
	case "DomainCreator":
		return strings.HasPrefix(name, "get")
	case "Crypto":
		switch name {
		case "areequalconstanttime", "decrypt", "decryptwithmanagediv", "encrypt", "encryptwithmanagediv",
			"generatedigest", "generateaeskey", "generatemac", "getrandominteger", "getrandomlong",
			"sign", "signwithcertificate", "verify", "verifyhmac", "verifywithcertificate":
			return true
		}
	case "Messaging":
		switch name {
		case "sendemail", "sendemailmessage", "renderemailtemplate", "renderstoredemailtemplate",
			"extractinboundemail", "reservesingleemailcapacity", "reservemassemailcapacity":
			return true
		}
	case "Metadata.Operations":
		switch name {
		case "enqueuedeployment", "checkdeploystatus", "retrieve":
			return true
		}
	case "reports.ReportManager":
		switch name {
		case "describereport", "getdatatypefilteroperatormap", "getreportinstance",
			"getreportinstances", "runasyncreport", "runreport":
			return true
		}
	case "IsvPartners.AppAnalytics":
		return name == "logcustominteraction"
	case "UserProvisioning.UserProvisioningLog":
		return name == "log"
	case "pref_center.TokenUtility":
		return name == "generatetoken" || name == "generatetokens"
	case "ApexPages":
		switch name {
		case "hasmessages", "addmessage", "addmessages", "getmessages", "currentpage":
			return true
		}
	case "Database":
		switch name {
		case "query", "querywithbinds", "countquery", "countquerywithbinds", "getquerylocator",
			"getquerylocatorwithbinds", "getasynclocator", "getcursor", "getcursorwithbinds", "getpaginationcursor",
			"getpaginationcursorwithbinds", "insert", "update", "upsert", "delete", "undelete",
			"insertasync", "updateasync", "deleteasync", "insertimmediate", "updateimmediate",
			"deleteimmediate", "getasyncsaveresult", "getasyncdeleteresult", "getdeleted", "getupdated",
			"emptyrecyclebin", "lock", "unlock", "merge", "setsavepoint", "releasesavepoint", "rollback",
			"executebatch", "treesave", "convertlead":
			return true
		}
	case "Database.QueryLocator":
		return name == "getquery" || name == "iterator" || name == "querymore"
	case "Database.QueryLocatorIterator", "Database.QueryLocatorChunkIterator":
		return name == "hasnext" || name == "next"
	case "Database.Cursor":
		return name == "fetch" || name == "getnumrecords"
	case "Database.PaginationCursor":
		return name == "fetchpage" || name == "fetchdeleted" || name == "getnumrecords"
	case "Database.CursorFetchResult":
		return name == "getrecords" || name == "getnextindex" || name == "getnumdeletedrecords" || name == "isdone"
	case "Database.BatchableContext", "Database.BatchableContextImpl":
		return name == "getjobid" || name == "getchildjobid"
	case "Database.GetDeletedResult":
		return name == "getdeletedrecords" || name == "getearliestdateavailable" || name == "getlatestdatecovered"
	case "Database.GetUpdatedResult":
		return name == "getids" || name == "getlatestdatecovered"
	case "Database.DeletedRecord":
		return name == "getid" || name == "getdeleteddate"
	case "Database.UnitOfWork":
		switch name {
		case "insertrecord", "insertrecords", "updaterecord", "updaterecords",
			"upsertrecord", "upsertrecords", "deleterecord", "deleterecords",
			"commitwork", "discardwork":
			return true
		}
	case "Approval":
		switch name {
		case "lock", "unlock", "islocked":
			return true
		}
	case "FlexQueue":
		switch name {
		case "moveafterjob", "movebeforejob", "movejobtoend", "movejobtofront":
			return true
		}
	case "System":
		switch name {
		case "now", "today", "currenttimemillis", "currentpagereference", "debug", "assert",
			"assertequals", "assertnotequals", "isrunningtest", "isbactivated", "isbatch",
			"isfuture", "isqueueable", "isscheduled", "isfunctioncallback", "isrunningelasticcompute",
			"getapplicationreadwritemode", "getquiddityshortcode", "requestversion",
			"enqueuejob", "schedule", "runas", "pausejobbyid", "pausejobbyname",
			"purgeoldasyncjobs", "resumejobbyid", "resumejobbyname",
			"setpassword", "abortjob", "attachfinalizer", "schedulebatch":
			return true
		}
	case "Test":
		switch name {
		case "isrunningtest", "getstandardpricebookid", "starttest", "stoptest", "createstub",
			"createsoqlstub", "clearapexpagemessages", "setcurrentpage", "setcurrentpagereference", "setmock",
			"setcreateddate", "setfixedsearchresults", "createstubqueryrow", "issoqlstubdefined",
			"geteventbus", "getexternalservice", "invokepage", "getflexqueueorder", "enqueuebatchjobs", "calculatepermissionsetgroup", "enablechangedatacapture",
			"setreadonlyapplicationmode", "testinstall", "testuninstall", "invokecontinuationmethod",
			"setcontinuationresponse", "testnotificationactionhandler", "testsandboxpostcopyscript":
			return true
		}
	case "Math":
		switch name {
		case "abs", "floor", "ceil", "round", "rint", "roundtolong", "signum", "sqrt", "cbrt",
			"acos", "asin", "atan", "cos", "sin", "tan", "cosh", "sinh", "tanh",
			"exp", "log", "log10", "max", "min", "mod", "pow", "atan2", "random":
			return true
		}
	case "Date":
		switch name {
		case "today", "newinstance", "valueof", "parse", "daysinmonth", "isleapyear",
			"format", "tostring", "adddays", "addmonths", "addyears", "daysbetween",
			"issameday", "monthsbetween", "year", "month", "day", "dayofyear",
			"tostartofmonth", "toendofmonth", "tostartofweek":
			return true
		}
	case "Blob":
		switch name {
		case "size", "topdf", "valueof":
			return true
		}
	case "Pattern":
		switch name {
		case "matcher", "pattern", "quote", "split":
			return true
		}
	case "Matcher":
		return matcherBehaviorMethod(typeName, name)
	case "Type":
		return name == "isassignablefrom"
	case "UUID":
		return name == "fromstring" || name == "randomuuid"
	case "Version":
		return name == "compareto" || name == "major" || name == "minor" || name == "patch"
	case "JSON":
		return name == "creategenerator" || name == "createparser"
	case "Location":
		switch name {
		case "getdistance", "getlatitude", "getlongitude", "newinstance":
			return true
		}
	case "Address":
		return name == "getdistance"
	case "Datetime":
		switch name {
		case "now", "newinstance", "newinstancegmt", "valueof", "valueofgmt", "parse",
			"format", "formatgmt", "formatlong", "tostring", "date", "dategmt", "gettime",
			"time", "timegmt", "adddays", "addmonths", "addyears", "addhours", "addminutes",
			"addseconds", "addmilliseconds", "issameday", "year", "month", "day", "hour",
			"minute", "second", "millisecond", "yeargmt", "monthgmt", "daygmt", "dayofyear",
			"dayofyeargmt", "hourgmt", "minutegmt", "secondgmt", "millisecondgmt":
			return true
		}
	case "Time":
		switch name {
		case "newinstance", "format", "tostring", "hour", "minute", "second", "millisecond",
			"addhours", "addminutes", "addseconds", "addmilliseconds":
			return true
		}
	case "Decimal", "Double", "Integer", "Long":
		switch name {
		case "abs", "format", "intvalue", "longvalue", "doublevalue", "decimalvalue",
			"setscale", "round", "toplainstring", "divide", "scale", "precision",
			"striptrailingzeros", "pow", "valueof":
			return true
		}
	case "Boolean":
		return name == "valueof"
	case "JSONGenerator", "JSONParser":
		return true
	case "HttpRequest":
		switch name {
		case "setendpoint", "getendpoint", "setmethod", "getmethod", "setbody", "setbodyasblob",
			"setbodydocument", "getbodydocument", "getbody", "getbodyasblob", "setheader", "getheaderkeys", "getheader",
			"setcompressed", "getcompressed", "settimeout", "gettimeout":
			return true
		}
	case "HttpResponse":
		switch name {
		case "setbody", "setbodyasblob", "getbody", "getbodyasblob", "getbodydocument",
			"getxmlstreamreader", "setstatuscode", "setstatus", "getstatus", "setheader", "getheaderkeys",
			"getheader", "getstatuscode":
			return true
		}
	case "PageReference":
		switch name {
		case "geturl", "setredirect", "getredirect", "getparameters", "getheaders", "getcookies", "setcookies", "setanchor", "getanchor",
			"setredirectcode", "getredirectcode", "forresource":
			return true
		}
	case "RestRequest":
		switch name {
		case "addheader", "getheader", "getheaderkeys", "addparameter", "addparam",
			"getparameter", "getparam", "getparameterkeys", "getparamkeys":
			return true
		}
	case "Schema":
		switch name {
		case "describetabs", "getappdescribe", "getmoduledescribe":
			return true
		}
	case "Search":
		return name == "query" || name == "find" || name == "suggest"
	case "ConnectApi.Organization":
		return name == "getsettings"
	case "ConnectApi.Communities":
		return name == "getcommunity"
	case "ConnectApi.ChatterUsers":
		return name == "getfollowings"
	case "ConnectApi.UserProfiles":
		return name == "setphoto" || name == "deletephoto"
	case "QueueableDuplicateSignature":
		return name == "builder"
	case "QueueableDuplicateSignature.Builder", "Builder":
		switch name {
		case "addid", "addinteger", "addstring", "build":
			return true
		}
	case "URL", "Url":
		switch name {
		case "getsalesforcebaseurl", "getorgdomainurl", "getcurrentrequesturl", "toexternalform",
			"getprotocol", "gethost", "getauthority", "getpath", "getquery", "getref",
			"getfile", "getport", "getdefaultport", "getuserinfo", "samefile":
			return true
		}
	case "SObject":
		switch name {
		case "clear", "get", "put", "getall", "getinstance", "getorgdefaults", "getvalues",
			"getsobject", "getsobjects", "putsobject", "getsobjecttype", "getquickactionname",
			"getpopulatedfieldsasmap", "isset", "clone", "haserrors", "geterrors",
			"adderror", "recalculateformulas", "getoptions", "setoptions", "isclone",
			"getclonesourceid":
			return true
		}
	}
	return false
}

func cacheBehaviorMethod(methodName string) bool {
	switch methodName {
	case "getpartition", "get", "put", "remove", "contains", "getkeys", "getnumkeys",
		"getcapacity", "isavailable", "getname",
		"getavggetsize", "getavggettime", "getavgvaluesize", "getmaxgetsize",
		"getmaxgettime", "getmaxvaluesize", "getmissrate",
		"createfullyqualifiedkey", "createfullyqualifiedpartition", "validatecachebuilder",
		"validatekey", "validatekeyvalue", "validatekeys", "validatepartitionname",
		"putimmediate", "scanforcount", "scanforkeyvalues", "scanformorekeyvalues":
		return true
	default:
		return false
	}
}

func apexPagesBehaviorMethod(typeName, methodName string) bool {
	switch typeName {
	case "ApexPages.Message":
		return strings.HasPrefix(methodName, "get")
	case "ApexPages.Action":
		return methodName == "getexpression"
	case "ApexPages.Component", "ApexPages.ComponentIteration":
		return methodName == "getcomponentbyid"
	case "ApexPages.StandardController":
		switch methodName {
		case "getid", "getrecord", "save", "quicksave", "delete", "view", "edit", "cancel", "reset", "addfields":
			return true
		}
	case "ApexPages.StandardSetController":
		return apexPagesStandardSetControllerMethod(methodName)
	case "ApexPages.IdeaStandardSetController":
		return apexPagesStandardSetControllerMethod(methodName) || methodName == "getrecord" ||
			methodName == "setpagenumber" || methodName == "getidealist" || methodName == "getlistviewoptions"
	case "ApexPages.IdeaStandardController", "ApexPages.KnowledgeArticleVersionStandardController":
		switch methodName {
		case "getid", "getrecord", "save", "quicksave", "delete", "view", "edit", "cancel", "reset", "addfields",
			"getcommentlist", "getsourceid", "selectdatacategory":
			return true
		}
	}
	return false
}

func apexPagesStandardSetControllerMethod(methodName string) bool {
	switch methodName {
	case "getrecords", "getselected", "setselected", "getpagesize", "setpagesize",
		"getpagenumber", "first", "last", "next", "previous", "gethasnext",
		"gethasprevious", "getcompleteresult", "getresultsize", "setfilterid",
		"getfilterid", "getlistviewoptions", "getrecord", "setpagenumber",
		"save", "cancel", "addfields":
		return true
	default:
		return false
	}
}

func passiveAccessorBehaviorType(typeName string) bool {
	switch typeName {
	case "Address", "Approval.ProcessRequest", "Approval.ProcessSubmitRequest",
		"Approval.ProcessWorkitemRequest", "Approval.ProcessResult",
		"Cookie",
		"Database.LeadConvert", "Database.LeadConvertResult", "Database.MergeRequest",
		"Database.UpsertResult", "Database.DuplicateError", "Database.CursorFetchResult",
		"Database.PaginationCursor", "Database.UnitOfWork", "FinalizerContext",
		"FinalizerContextImpl", "InstallContext", "Messaging.ActionResult",
		"Messaging.ActionResult.Builder", "Messaging.ActionableNotification",
		"Messaging.ActionableNotification.Builder", "Messaging.Builder",
		"Messaging.CustomNotification", "Messaging.PushNotification", "OrgLimit",
		"OrgInstrumentationContext", "OrgInstrumentationOperation",
		"OrgInstrumentationService", "QuickAction", "QuickAction.SendEmailQuickActionDefaults",
		"Builder", "Domain", "Request", "ResetPasswordResult", "SandboxContext", "Version":
		return true
	default:
		return false
	}
}

func accessorBehaviorMethod(member typesys.MemberSymbol) bool {
	name := strings.ToLower(member.Name)
	return strings.HasPrefix(name, "get") ||
		strings.HasPrefix(name, "set") ||
		strings.HasPrefix(name, "is") ||
		strings.HasPrefix(name, "with") ||
		name == "build"
}

func matcherBehaviorMethod(typeName, methodName string) bool {
	if typeName != "Matcher" {
		return false
	}
	switch methodName {
	case "matches", "lookingat", "find", "group", "groupcount", "start", "end",
		"replaceall", "replacefirst", "reset", "region", "regionstart", "regionend",
		"usepattern", "hasanchoringbounds", "hastransparentbounds", "useanchoringbounds",
		"usetransparentbounds", "hitend", "pattern", "quotereplacement", "requireend":
		return true
	default:
		return false
	}
}

func xmlStreamWriterBehaviorMethod(typeName, methodName string) bool {
	if typeName != "XmlStreamWriter" {
		return false
	}
	switch methodName {
	case "close", "getxmlstring", "setdefaultnamespace", "writeattribute", "writecdata",
		"writecharacters", "writecomment", "writedefaultnamespace", "writeemptyelement",
		"writeenddocument", "writeendelement", "writenamespace", "writeprocessinginstruction",
		"writestartdocument", "writestartelement":
		return true
	default:
		return false
	}
}

func calloutMockBehaviorMethod(typeName, methodName string) bool {
	switch typeName {
	case "StaticResourceCalloutMock", "MultiStaticResourceCalloutMock":
		switch methodName {
		case "respond", "setheader", "setstaticresource", "setstatus", "setstatuscode":
			return true
		}
	}
	return false
}

func searchDTOBehaviorMethod(typeName, methodName string) bool {
	switch typeName {
	case "Search.KnowledgeSuggestionFilter", "Search.QuestionSuggestionFilter":
		return strings.HasPrefix(methodName, "add") || strings.HasPrefix(methodName, "set")
	case "Search.SuggestionOption":
		return methodName == "setfilter" || methodName == "setlimit"
	case "Search.SearchResult", "Search.SuggestionResult":
		return strings.HasPrefix(methodName, "get")
	case "Search.SearchResults":
		return methodName == "get"
	case "Search.SuggestionResults":
		return methodName == "getsuggestionresults" || methodName == "hasmoreresults"
	default:
		return false
	}
}

func messagingBehaviorMethod(typeName, methodName string) bool {
	if !strings.HasPrefix(typeName, "Messaging.") {
		return false
	}
	switch typeName {
	case "Messaging.Email", "Messaging.EmailAttachment", "Messaging.EmailFileAttachment",
		"Messaging.SingleEmailMessage", "Messaging.MassEmailMessage",
		"Messaging.SendEmailResult", "Messaging.SendEmailError",
		"Messaging.RenderEmailTemplateBodyResult", "Messaging.RenderEmailTemplateError",
		"Messaging.ActionResult", "Messaging.ActionableNotification":
		return strings.HasPrefix(methodName, "get") ||
			strings.HasPrefix(methodName, "set") ||
			strings.HasPrefix(methodName, "is")
	case "Messaging.CustomNotification", "Messaging.PushNotification",
		"Messaging.ActionResult.Builder", "Messaging.ActionableNotification.Builder", "Messaging.Builder":
		return strings.HasPrefix(methodName, "get") ||
			strings.HasPrefix(methodName, "set") ||
			strings.HasPrefix(methodName, "is") ||
			strings.HasPrefix(methodName, "with") ||
			methodName == "send" || methodName == "build"
	case "Messaging.PushNotificationPayload":
		return methodName == "apple"
	default:
		return false
	}
}

func standardExceptionBehaviorMethod(typeName, methodName string) bool {
	if !strings.HasSuffix(typeName, "Exception") {
		return false
	}
	switch methodName {
	case "getmessage", "setmessage", "getcause", "initcause", "getlineNumber", "getlinenumber",
		"getstacktrace", "getstacktracestring", "gettypeName", "gettypename",
		"getnumdml", "getdmltype", "getdmlmessage", "getdmlstatuscode", "getdmlfieldnames", "getdmlfields",
		"getdmlid", "getdmlindex", "getinaccessiblefields":
		return true
	default:
		return false
	}
}

func explicitlyUnsupportedCoreBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "String", "Id", "Boolean", "Date", "Datetime", "Decimal", "Double", "Integer", "Long", "Time":
		return name == "adderror"
	case "LIST":
		return true
	case "Approval":
		return name != "lock" && name != "unlock" && name != "islocked"
	case "Crypto":
		switch name {
		case "signxml":
			return true
		}
	case "System":
		switch name {
		case "changeownpassword", "getapplicationreadwritemode", "getquiddityshortcode",
			"isfunctioncallback", "isrunningelasticcompute", "movepassword", "requestversion", "resetpassword",
			"resetpasswordwithemailtemplate":
			return true
		}
	case "UserManagement":
		switch name {
		case "formatphonenumber", "initselfregistration", "verifyselfregistration":
			return false
		default:
			return true
		}
	case "Test":
		switch name {
		case "getexternalservice", "invokepage":
			return true
		}
	case "QuickAction":
		switch name {
		case "performquickaction", "performquickactions":
			return true
		}
	case "Site":
		switch name {
		case "createpersonaccountportaluser", "passwordlesslogin", "setportaluserasauthprovider":
			return true
		}
	case "Network":
		switch name {
		case "createexternaluserasync", "createrecordasync",
			"loadallpackagedefaultnetworkdashboardsettings",
			"loadallpackagedefaultnetworkpulsesettings",
			"loadallpackagedefaultnetworkworkspacemetricsettings":
			return true
		}
	case "OrgInstrumentationOperation":
		return true
	case "Search":
		return name != "query"
	case "HttpRequest":
		return name == "setclientcertificatename" || name == "setclientcertificate"
	case "PageReference":
		return name == "getcontent" || name == "getcontentaspdf" || name == "setcookies"
	case "Database":
		switch name {
		case "getasynclocator":
			return true
		}
	default:
		if connectAPIExternalServiceBehaviorMethod(symbol, member) {
			return true
		}
		return false
	}
	return false
}

func primitiveFieldAddErrorBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod || !strings.EqualFold(member.Name, "addError") {
		return false
	}
	switch stubBehaviorTypeName(symbol) {
	case "String", "Id", "Boolean", "Date", "Datetime", "Decimal", "Double", "Integer", "Long", "Time":
		return true
	default:
		return false
	}
}

func xmlStreamReaderBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if !strings.EqualFold(stubBehaviorTypeName(symbol), "XmlStreamReader") ||
		(member.Kind != apexast.DeclarationMethod && member.Kind != apexast.DeclarationConstructor) {
		return false
	}
	if member.Kind == apexast.DeclarationConstructor {
		return true
	}
	switch strings.ToLower(member.Name) {
	case "<init>", "xmlstreamreader",
		"getattributecount", "getattributelocalname", "getattributenamespace",
		"getattributeprefix", "getattributetype", "getattributevalue", "getattributevalueat",
		"geteventtype", "getlocalname", "getlocation", "getnamespace", "getnamespacecount",
		"getnamespaceprefix", "getnamespaceuri", "getnamespaceuriat", "getpidata", "getpitarget",
		"getprefix", "gettext", "getversion", "hasname", "hasnext", "hastext",
		"ischaracters", "isendelement", "isstartelement", "iswhitespace", "next", "nexttag",
		"setcoalescing", "setnamespaceaware":
		return true
	default:
		return false
	}
}

func domXmlNodeBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if !strings.EqualFold(stubBehaviorTypeName(symbol), "Dom.XmlNode") || member.Kind != apexast.DeclarationMethod {
		return false
	}
	switch strings.ToLower(member.Name) {
	case "addchildelement", "addcommentnode", "addtextnode",
		"getattribute", "getattributecount", "getattributekeyat", "getattributekeynsat",
		"getattributevalue", "getattributevaluens", "getchildelement", "getchildelements",
		"getchildren", "getname", "getnamespace", "getnamespacefor", "getnodetype",
		"getparent", "getprefixfor", "gettext", "insertbefore", "removeattribute",
		"removechild", "setattribute", "setattributens", "setnamespace":
		return true
	default:
		return false
	}
}

func visualEditorDynamicPickListRowsBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	typeName := stubBehaviorTypeName(symbol)
	if member.Kind != apexast.DeclarationMethod && member.Kind != apexast.DeclarationConstructor {
		return false
	}
	if strings.EqualFold(typeName, "VisualEditor.DataRow") {
		if member.Kind == apexast.DeclarationConstructor {
			return true
		}
		switch strings.ToLower(member.Name) {
		case "getlabel", "getvalue", "isselected", "compareto", "setlabel", "setvalue":
			return true
		default:
			return false
		}
	}
	if !strings.EqualFold(typeName, "VisualEditor.DynamicPickListRows") {
		return false
	}
	if member.Kind == apexast.DeclarationConstructor {
		return true
	}
	switch strings.ToLower(member.Name) {
	case "addallrows", "addrow", "containsallrows", "get", "getdatarows", "setcontainsallrows", "size", "sort":
		return true
	default:
		return false
	}
}

func compressionZipBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	typeName := stubBehaviorTypeName(symbol)
	if member.Kind == apexast.DeclarationConstructor {
		return typeName == "compression.ZipWriter" || typeName == "compression.ZipReader"
	}
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	switch typeName {
	case "compression.ZipWriter":
		switch strings.ToLower(member.Name) {
		case "addentry", "getarchive", "getentries", "getentry", "getentrynames", "getlevel", "getmethod", "removeentry", "setlevel", "setmethod":
			return true
		}
	case "compression.ZipReader":
		switch strings.ToLower(member.Name) {
		case "extract", "getentries", "getentriesmap", "getentry", "getentrynames":
			return true
		}
	}
	return false
}

func waveQueryBuilderBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "wave.QueryBuilder":
		switch name {
		case "load", "loadbydevelopername", "get", "count", "union", "cogroup":
			return true
		}
	case "wave.QueryNode":
		switch name {
		case "build", "cap", "filter", "foreach", "group", "order":
			return true
		}
	case "wave.ProjectionNode":
		switch name {
		case "build", "alias", "avg", "count", "max", "min", "sum", "unique":
			return true
		}
	}
	return false
}

func contextIndustriesBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if !strings.EqualFold(stubBehaviorTypeName(symbol), "Context.IndustriesContext") || member.Kind != apexast.DeclarationMethod {
		return false
	}
	switch strings.ToLower(member.Name) {
	case "addrecordstocontext", "buildcontext", "deletecontext", "evictcontextdefinition",
		"filteringcontext", "getcontext", "getcontexttranslation", "leanerquerytags",
		"persistcontext", "querycontextrecordsandchildren", "queryrecordstatus",
		"querytags", "updatecontextattributes":
		return true
	default:
		return false
	}
}

func orgInstrumentationBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	switch strings.ToLower(member.Name) {
	case "starttime":
		return strings.EqualFold(typeName, "OrgInstrumentationContext")
	case "end":
		return strings.EqualFold(typeName, "OrgInstrumentationContext") || strings.EqualFold(typeName, "OrgInstrumentationOperation")
	case "propagatecontext":
		return strings.EqualFold(typeName, "OrgInstrumentationService")
	case "createnewspan", "endwithstatus", "publishcustomhistogramvalues",
		"publishcustomincrementalvalue", "publishcustompercentileset",
		"publishincrementalvalue", "publishpercentileset",
		"publishrequestcountandduration", "start":
		return strings.EqualFold(typeName, "OrgInstrumentationOperation")
	default:
		return false
	}
}

func userProvisioningBatchableBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	switch typeName {
	case "UserProvisioning.ProvisioningBatchable", "UserProvisioning.CollectingBatchable",
		"UserProvisioning.PluginBatchable", "UserProvisioning.LinkingBatchable":
	default:
		return false
	}
	switch strings.ToLower(member.Name) {
	case "execute", "finish", "flowinputpreprocessing", "flowpostprocessing",
		"geteventprefix", "getflowname", "getflownamespace", "getperbatchupl",
		"getperbatchupr", "getuprtonewuplmap", "hasflow", "hasfloworapex",
		"postbatchprocessing", "start":
		return true
	default:
		return false
	}
}

func callbackInterfaceBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := strings.ToLower(stubBehaviorTypeName(symbol))
	name := strings.ToLower(member.Name)
	switch typeName {
	case "commerceextension.resolutionstrategy":
		return name == "resolve"
	case "database.batchable":
		return name == "start" || name == "execute" || name == "finish"
	case "queueable", "finalizer":
		return name == "execute"
	case "metadata.deploycallback":
		return name == "handleresult"
	case "process.plugin":
		return name == "describe" || name == "invoke"
	case "quickaction.quickactiondefaultshandler":
		return name == "oninitdefaults"
	case "userprovisioning.userprovisioningplugin":
		return name == "invoke"
	case "readiness.productevaluator":
		return name == "evaluatereadiness" || name == "isactive"
	default:
		return false
	}
}

func localMockHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "Canvas.Test":
		return stubBehaviorMemberStatic(member) && (name == "mockrendercontext" || name == "testcanvaslifecycle")
	case "HttpCalloutMock":
		return name == "respond"
	case "WebServiceMock":
		return name == "doinvoke"
	case "SoqlStubProvider":
		return name == "handlesoqlquery"
	case "eventbus.TestBroker":
		return name == "deliver" || name == "fail"
	case "eventbus.TestEventService":
		return stubBehaviorMemberStatic(member) && name == "publishevent"
	case "ExternalServiceTest":
		return name == "sendcallback"
	case "TestAsyncHttp":
		return name == "executehttprequest"
	case "functions.FunctionInvokeMock":
		return name == "respond"
	case "functions.MockFunctionInvocationFactory":
		return stubBehaviorMemberStatic(member) && (name == "createerrorresponse" || name == "createsuccessresponse")
	case "SubMgmt.Test":
		return stubBehaviorMemberStatic(member) && (name == "create" || name == "modify" || name == "remove")
	case "UserProvisioning.ConnectorTestUtil":
		return stubBehaviorMemberStatic(member) && name == "createconnectedapp"
	case "CartExtension.CartCalculateExecutorMock":
		return strings.EqualFold(member.Type, "void")
	case "CartExtension.SplitShipmentServiceMock":
		return strings.EqualFold(member.Type, "void")
	default:
		return false
	}
}

func socialInboundDefaultHandlerBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod || genericObjectBehaviorMethod(member) {
		return false
	}
	switch stubBehaviorTypeName(symbol) {
	case "Social.DefaultInboundSocialPostHandler", "Social.InboundSocialPostHandlerImpl":
		return true
	default:
		return false
	}
}

func connectAPIExternalServiceBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	typeName := stubBehaviorTypeName(symbol)
	if !strings.HasPrefix(typeName, "ConnectApi.") || !stubBehaviorMemberStatic(member) {
		return false
	}
	if connectAPITestFixtureSetterBehaviorMethod(symbol, member) {
		return false
	}
	if len(member.Parameters) == 0 &&
		(strings.EqualFold(member.Type, typeName) ||
			(strings.EqualFold(member.Name, "builder") && strings.HasSuffix(member.Type, ".Builder"))) {
		return false
	}
	return true
}

func connectAPITestFixtureSetterBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	return member.Kind == apexast.DeclarationMethod &&
		stubBehaviorMemberStatic(member) &&
		strings.HasPrefix(stubBehaviorTypeName(symbol), "ConnectApi.") &&
		strings.HasPrefix(strings.ToLower(member.Name), "settest") &&
		strings.EqualFold(member.Type, "void")
}

func connectAPITestFixtureTargetBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod ||
		!stubBehaviorMemberStatic(member) ||
		!strings.HasPrefix(stubBehaviorTypeName(symbol), "ConnectApi.") ||
		strings.HasPrefix(strings.ToLower(member.Name), "settest") ||
		strings.EqualFold(member.Type, "void") {
		return false
	}
	setterName := "settest" + strings.ToLower(member.Name)
	for _, candidate := range symbol.Members {
		if !connectAPITestFixtureSetterBehaviorMethod(symbol, candidate) ||
			strings.ToLower(candidate.Name) != setterName ||
			len(candidate.Parameters) != len(member.Parameters)+1 {
			continue
		}
		last := candidate.Parameters[len(candidate.Parameters)-1]
		if !strings.EqualFold(last.Type, member.Type) {
			continue
		}
		matched := true
		for i, param := range member.Parameters {
			if !strings.EqualFold(candidate.Parameters[i].Type, param.Type) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func connectAPIReadOnlyHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod ||
		!stubBehaviorMemberStatic(member) ||
		!connectAPIReadOnlyHarnessBehaviorType(stubBehaviorTypeName(symbol)) ||
		!connectAPIReadOnlyHarnessBehaviorReturn(member.Type) {
		return false
	}
	return connectAPIReadOnlyHarnessBehaviorMethodName(member.Name) && !connectAPIMutationBehaviorMethodName(member.Name)
}

func connectAPIReadOnlyHarnessBehaviorType(typeName string) bool {
	switch typeName {
	case "ConnectApi.ChatterFeeds",
		"ConnectApi.ChatterGroups",
		"ConnectApi.ChatterMessages",
		"ConnectApi.ChatterUsers",
		"ConnectApi.Topics",
		"ConnectApi.Recommendations",
		"ConnectApi.ManagedContent",
		"ConnectApi.ManagedContentDelivery",
		"ConnectApi.ManagedTopics",
		"ConnectApi.ManagedContentSpaces":
		return true
	default:
		return false
	}
}

func connectAPIReadOnlyHarnessBehaviorMethodName(methodName string) bool {
	name := strings.ToLower(methodName)
	return strings.HasPrefix(name, "get") ||
		strings.HasPrefix(name, "search") ||
		strings.HasPrefix(name, "find") ||
		strings.HasPrefix(name, "list") ||
		strings.HasPrefix(name, "query")
}

func connectAPIMutationBehaviorMethodName(methodName string) bool {
	name := strings.ToLower(methodName)
	for _, prefix := range []string{
		"add", "assign", "ban", "block", "create", "delete", "edit", "follow",
		"join", "leave", "like", "mute", "pin", "post", "publish", "remove",
		"send", "set", "subscribe", "unfollow", "unpublish", "update",
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func connectAPIReadOnlyHarnessBehaviorReturn(returnType string) bool {
	return returnType != "" && !strings.EqualFold(returnType, "void")
}

func databaseResultDTOBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "Database.SaveResult", "Database.DeleteResult", "Database.UndeleteResult",
		"Database.EmptyRecycleBinResult", "Database.LockResult", "Database.UnlockResult",
		"Approval.LockResult", "Approval.UnlockResult":
		return name == "issuccess" || name == "getid" || name == "geterrors"
	case "Database.UpsertResult":
		return name == "issuccess" || name == "getid" || name == "geterrors" || name == "iscreated"
	case "Database.MergeResult":
		return name == "issuccess" || name == "getid" || name == "geterrors" ||
			name == "getmergedrecordids" || name == "getupdatedrelatedids"
	case "Database.NestedSaveResult":
		return name == "issuccess" || name == "getid" || name == "geterrors" || name == "getrelationshipsaveresults"
	case "Database.RelationshipSaveResult":
		return name == "getrelationshipname" || name == "getsaveresults"
	case "Database.Error":
		return name == "getmessage" || name == "getstatuscode" || name == "getfields" ||
			name == "getextendederrordetails"
	default:
		return false
	}
}

func stringBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if !strings.EqualFold(stubBehaviorTypeName(symbol), "String") || member.Kind != apexast.DeclarationMethod {
		return false
	}
	switch strings.ToLower(member.Name) {
	case "abbreviate", "capitalize", "center", "charat", "codepointat", "codepointbefore",
		"codepointcount", "compareto", "containsany", "containsignorecase", "containsnone",
		"containsonly", "containswhitespace", "countmatches", "deletewhitespace", "difference",
		"endswithignorecase", "escapecsv", "escapeecmascript", "escapehtml3", "escapehtml4",
		"escapejava", "escapesinglequotes", "escapeunicode", "escapexml", "format",
		"fromchararray", "getchars", "getcommonprefix", "getlevenshteindistance", "indexofany",
		"indexofanybut", "indexofchar", "indexofdifference", "indexofignorecase",
		"isalllowercase", "isalluppercase", "isalpha", "isalphaspace",
		"isalphanumeric", "isalphanumericspace", "isasciiprintable", "isempty", "isnotempty",
		"isnumeric", "isnumericspace", "iswhitespace", "lastindexofchar",
		"lastindexofignorecase", "left", "leftpad", "mid", "normalizespace",
		"offsetbycodepoints", "overlay", "remove", "removeend", "removeendignorecase",
		"removestart", "removestartignorecase", "repeat", "replaceall", "replacefirst",
		"reverse", "right", "rightpad", "splitbycharactertype", "splitbycharactertypecamelcase",
		"startswithignorecase", "substringafter",
		"striphtmltags", "substringafterlast", "substringbefore", "substringbeforelast",
		"substringbetween", "swapcase", "uncapitalize", "unescapecsv", "unescapeecmascript",
		"unescapehtml3", "unescapehtml4", "unescapejava", "unescapeunicode", "unescapexml",
		"valueofgmt":
		return true
	default:
		return false
	}
}

func genericObjectBehaviorMethod(member typesys.MemberSymbol) bool {
	switch strings.ToLower(member.Name) {
	case "clone", "equals", "hashcode", "tostring":
		return true
	default:
		return false
	}
}

func genericEnumBehaviorMethod(member typesys.MemberSymbol) bool {
	switch strings.ToLower(member.Name) {
	case "equals", "hashcode", "name", "ordinal", "tostring", "valueof", "values":
		return true
	default:
		return false
	}
}

func generatedDTOAccessorBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if !generatedDTOBehaviorType(symbol) {
		return false
	}
	name := member.Name
	if len(name) <= 3 {
		return false
	}
	switch {
	case strings.HasPrefix(strings.ToLower(name), "get"):
		return true
	case strings.HasPrefix(strings.ToLower(name), "set") && strings.EqualFold(member.Type, "void"):
		return true
	case strings.HasPrefix(strings.ToLower(name), "is") && strings.EqualFold(member.Type, "Boolean"):
		return true
	default:
		return false
	}
}

func generatedDTOCollectionBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if !generatedDTOCollectionBehaviorType(symbol) || member.Kind != apexast.DeclarationMethod {
		return false
	}
	return generatedDTOCollectionBehaviorMethodShape(member)
}

func generatedDTOCollectionBehaviorMethodShape(member typesys.MemberSymbol) bool {
	name := strings.ToLower(member.Name)
	if genericObjectBehaviorMethod(member) {
		return true
	}
	switch name {
	case "clear", "size", "isempty", "iterator", "getiterator":
		return len(member.Parameters) == 0
	case "get", "getfromlist", "indexof", "getindexof":
		return len(member.Parameters) == 1
	case "add", "remove":
		return len(member.Parameters) == 1 && strings.EqualFold(member.Type, "void")
	default:
		return false
	}
}

func slackPassiveBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	if !strings.HasPrefix(typeName, "Slack.") || slackServiceBehaviorType(typeName) {
		return false
	}
	name := strings.ToLower(member.Name)
	return slackPassiveMethodShape(typeName, name, member.Type, member.Parameters, stubBehaviorMemberStatic(member))
}

func slackTestHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "Slack.State":
		return strings.HasPrefix(name, "clear") ||
			strings.HasPrefix(name, "create")
	case "Slack.UserSession":
		switch name {
		case "closeallmodals", "closemodal",
			"getapphome", "getmessagecount", "getmessages", "getmodalstack", "getopenchannel", "getstate", "gettopmodal", "getuser",
			"openapphome", "openchannel", "postmessage":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func slackLocalClientHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod ||
		strings.EqualFold(member.Type, "void") ||
		member.Type == "" {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	if typeName != "Slack.BotClient" && typeName != "Slack.UserClient" {
		return false
	}
	name := strings.ToLower(member.Name)
	if strings.Contains(name, "post") || strings.Contains(name, "open") || strings.Contains(name, "update") {
		return false
	}
	for _, part := range []string{"add", "archive", "close", "create", "delete", "disable", "enable", "invite", "join", "kick", "leave", "mark", "publish", "push", "remove", "rename", "revoke", "schedule", "send", "set", "share", "unarchive", "uninstall"} {
		if strings.Contains(name, part) {
			return false
		}
	}
	return strings.HasPrefix(name, "auth") ||
		strings.HasSuffix(name, "info") ||
		strings.HasSuffix(name, "list") ||
		strings.HasSuffix(name, "history") ||
		strings.HasSuffix(name, "members") ||
		strings.HasSuffix(name, "replies") ||
		strings.HasSuffix(name, "conversations") ||
		strings.HasSuffix(name, "profileget") ||
		strings.HasSuffix(name, "getpresence") ||
		strings.HasSuffix(name, "lookupbyemail")
}

func slackServiceBehaviorType(typeName string) bool {
	short := typeName[strings.LastIndex(typeName, ".")+1:]
	if strings.HasSuffix(short, "Client") && !strings.HasSuffix(short, "ClientMock") {
		return true
	}
	return strings.HasSuffix(short, "Dispatcher") || strings.HasSuffix(short, "Provider")
}

func slackPassiveMethodShape(typeName, methodName, returnType string, params []apexast.Parameter, static bool) bool {
	if static && methodName == "builder" && strings.HasSuffix(returnType, ".Builder") {
		return true
	}
	if strings.HasPrefix(methodName, "get") || strings.HasPrefix(methodName, "set") || strings.HasPrefix(methodName, "is") {
		return true
	}
	if strings.HasSuffix(typeName, ".Builder") && (methodName == "build" || strings.EqualFold(returnType, typeName)) {
		return true
	}
	if typeName == "Slack.Builder" && strings.HasSuffix(returnType, ".Builder") {
		return true
	}
	if strings.HasSuffix(typeName, "ClientMock") && strings.HasPrefix(returnType, "Slack.") {
		return true
	}
	if strings.EqualFold(returnType, typeName) {
		return true
	}
	return generatedDTOCollectionReturnType(returnType) || len(params) == 0 && strings.HasPrefix(returnType, "Slack.")
}

func generatedDTOCollectionMethod(member typesys.MemberSymbol) bool {
	return generatedDTOCollectionReturnType(member.Type)
}

func generatedDTOCollectionReturnType(returnType string) bool {
	return strings.HasPrefix(returnType, "List<") ||
		strings.HasPrefix(returnType, "Set<") ||
		strings.HasPrefix(returnType, "Map<")
}

func generatedDTOBehaviorType(symbol typesys.TypeSymbol) bool {
	typeName := stubBehaviorTypeName(symbol)
	if typeName == "" || !strings.Contains(typeName, ".") || symbol.Kind != apexast.DeclarationClass {
		return false
	}
	if generatedExecutionSurfaceType(typeName) {
		return false
	}
	if strings.HasPrefix(typeName, "ConnectApi.") {
		return generatedPassiveDTOShape(symbol)
	}
	if safeSchemaPassiveDTOBehaviorType(typeName) {
		return generatedPassiveDTOShape(symbol)
	}
	if strings.HasPrefix(typeName, "Schema.") || strings.HasPrefix(typeName, "ApexPages.") ||
		strings.HasPrefix(typeName, "Messaging.") || strings.HasPrefix(typeName, "Dom.") ||
		strings.HasPrefix(typeName, "System.") || strings.HasPrefix(typeName, "Database.") ||
		strings.HasPrefix(typeName, "Test.") || strings.HasPrefix(typeName, "UserInfo.") ||
		strings.HasPrefix(typeName, "Site.") || strings.HasPrefix(typeName, "Network.") ||
		strings.HasPrefix(typeName, "Search.") || strings.HasPrefix(typeName, "Approval.") ||
		strings.HasPrefix(typeName, "Security.") || strings.HasPrefix(typeName, "EventBus.") ||
		strings.HasPrefix(typeName, "RestContext.") || strings.HasPrefix(typeName, "RestRequest.") ||
		strings.HasPrefix(typeName, "RestResponse.") {
		return false
	}
	return generatedPassiveDTOShape(symbol)
}

func safeSchemaPassiveDTOBehaviorType(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "schema.datacategory",
		"schema.datacategorygroupsobjecttypepair",
		"schema.describecolorresult",
		"schema.describedatacategorygroupresult",
		"schema.describedatacategorygroupstructureresult":
		return true
	default:
		return false
	}
}

func generatedDTOCollectionBehaviorType(symbol typesys.TypeSymbol) bool {
	typeName := stubBehaviorTypeName(symbol)
	if typeName == "" || !strings.Contains(typeName, ".") || symbol.Kind != apexast.DeclarationClass {
		return false
	}
	short := typeName[strings.LastIndex(typeName, ".")+1:]
	if !(strings.HasSuffix(short, "Collection") || strings.HasSuffix(short, "List")) {
		return false
	}
	if generatedExecutionSurfaceType(typeName) {
		return false
	}
	hasCollectionMethod := false
	for _, member := range symbol.Members {
		switch member.Kind {
		case apexast.DeclarationConstructor:
			continue
		case apexast.DeclarationMethod:
			if !generatedDTOCollectionBehaviorMethodShape(member) {
				return false
			}
			if !genericObjectBehaviorMethod(member) {
				hasCollectionMethod = true
			}
		default:
			return false
		}
	}
	return hasCollectionMethod
}

func ideasFindSimilarBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	return stubBehaviorTypeName(symbol) == "Ideas" &&
		member.Kind == apexast.DeclarationMethod &&
		strings.EqualFold(member.Name, "findSimilar")
}

func generatedExecutionSurfaceType(typeName string) bool {
	for _, prefix := range []string{
		"Flow.",
		"Cache.",
		"cache.",
		"Continuation.",
		"ExternalService.",
		"ExternalServiceTest.",
	} {
		if strings.HasPrefix(typeName, prefix) {
			return true
		}
	}
	return false
}

func generatedPassiveDTOShape(symbol typesys.TypeSymbol) bool {
	typeName := stubBehaviorTypeName(symbol)
	if strings.EqualFold(typeName, "Invocable.Action") || strings.EqualFold(typeName, "Flow.Interview") {
		return true
	}
	hasDataShape := false
	for _, member := range symbol.Members {
		switch member.Kind {
		case apexast.DeclarationConstructor, apexast.DeclarationProperty:
			hasDataShape = true
		case apexast.DeclarationMethod:
			if generatedPassiveDTOMethod(member, typeName) {
				continue
			}
			return false
		}
	}
	return hasDataShape
}

func generatedPassiveDTOMethod(member typesys.MemberSymbol, typeName string) bool {
	name := strings.ToLower(member.Name)
	if genericObjectBehaviorMethod(member) {
		return true
	}
	switch {
	case name == "getbuildversion":
		return true
	case len(member.Parameters) == 1 && strings.EqualFold(member.Type, "void") &&
		(strings.HasPrefix(name, "add") || strings.HasPrefix(name, "remove")):
		return true
	case len(member.Parameters) == 2 && strings.EqualFold(member.Type, "void") && strings.HasPrefix(name, "add"):
		return true
	case len(member.Parameters) == 3 && strings.EqualFold(member.Type, "void") && strings.HasPrefix(name, "add"):
		return true
	case strings.EqualFold(member.Type, typeName):
		return true
	case strings.HasPrefix(name, "get"), strings.HasPrefix(name, "set"), strings.HasPrefix(name, "is"):
		return true
	case strings.HasPrefix(name, "with"), name == "build":
		return true
	default:
		return false
	}
}

func generatedTopLevelPassiveBehaviorType(symbol typesys.TypeSymbol) bool {
	if symbol.Kind != apexast.DeclarationClass {
		return false
	}
	switch stubBehaviorTypeName(symbol) {
	case "Answers", "AppExchangeTrialTemplate", "AppExchangeUserPerms":
		return true
	case "licensing.UserLicenseDefinition", "licensing.PlatformLicenseDefinition":
		return true
	default:
		return false
	}
}

type stubBehaviorEvidence map[string]stubBehaviorEvidenceEntry

type stubBehaviorEvidenceEntry struct {
	status   StubBehaviorStatus
	evidence []string
	notes    string
}

func buildStubBehaviorEvidence() stubBehaviorEvidence {
	out := stubBehaviorEvidence{}
	for _, entry := range StdlibMatrix() {
		status, ok := stubBehaviorStatusFromCapability(entry.Status)
		if !ok {
			continue
		}
		api := strings.TrimSpace(entry.API)
		if strings.Contains(api, " ") && strings.HasSuffix(strings.Fields(api)[0], ".*") {
			api = strings.Fields(api)[0]
		}
		if api == "" || strings.Contains(api, " ") || strings.EqualFold(api, "unimplemented platform/stdlib calls") {
			continue
		}
		out.add(api, status, "stdlib matrix", entry.Notes)
		if strings.HasSuffix(api, ".*") {
			out.add(strings.TrimSuffix(api, ".*"), status, "stdlib matrix", entry.Notes)
		}
	}
	return out
}

func (e stubBehaviorEvidence) add(api string, status StubBehaviorStatus, source, notes string) {
	key := normalizeStubBehaviorKey(api)
	if key == "" {
		return
	}
	existing, ok := e[key]
	if !ok || stubBehaviorStatusRank(status) < stubBehaviorStatusRank(existing.status) {
		e[key] = stubBehaviorEvidenceEntry{status: status, evidence: []string{source + ": " + api}, notes: notes}
		return
	}
	if ok && status == existing.status {
		existing.evidence = append(existing.evidence, source+": "+api)
		if existing.notes == "" {
			existing.notes = notes
		}
		e[key] = existing
	}
}

func (e stubBehaviorEvidence) lookup(typeName, member string) *stubBehaviorEvidenceEntry {
	candidates := []string{typeName}
	if member != "" {
		candidates = []string{typeName + "." + member}
		if idx := strings.LastIndex(typeName, "."); idx >= 0 {
			candidates = append(candidates, typeName[idx+1:]+"."+member)
		}
	}
	for _, candidate := range candidates {
		if match, ok := e[normalizeStubBehaviorKey(candidate)]; ok {
			return &match
		}
		if member != "" {
			if match, ok := e[normalizeStubBehaviorKey(typeName+".*")]; ok {
				return &match
			}
		}
	}
	return nil
}

func stubBehaviorStatusFromCapability(status Status) (StubBehaviorStatus, bool) {
	switch status {
	case StatusSupported, StatusPartial:
		return StubBehaviorImplemented, true
	case StatusUnsupported:
		return StubBehaviorUnsupported, true
	default:
		return "", false
	}
}

func stubBehaviorStatusRank(status StubBehaviorStatus) int {
	switch status {
	case StubBehaviorImplemented:
		return 0
	case StubBehaviorUnsupported:
		return 1
	case StubBehaviorPassiveDefault:
		return 2
	default:
		return 3
	}
}

func countStubBehaviorTotals(entries []StubBehaviorEntry) StubBehaviorTotals {
	totals := StubBehaviorTotals{Entries: len(entries), ByStatus: map[string]int{}}
	for _, status := range []StubBehaviorStatus{StubBehaviorImplemented, StubBehaviorPassiveDefault, StubBehaviorUnsupported, StubBehaviorUnknown} {
		totals.ByStatus[string(status)] = 0
	}
	for _, entry := range entries {
		if entry.Member == "" {
			totals.Types++
		} else {
			totals.Members++
		}
		totals.ByStatus[string(entry.Status)]++
		switch entry.Status {
		case StubBehaviorImplemented:
			totals.Implemented++
		case StubBehaviorPassiveDefault:
			totals.PassiveDefault++
		case StubBehaviorUnsupported:
			totals.Unsupported++
		default:
			totals.Unknown++
		}
	}
	return totals
}

func stubBehaviorTypeName(symbol typesys.TypeSymbol) string {
	if symbol.Namespace == "" || strings.EqualFold(symbol.Namespace, "System") {
		return symbol.Name
	}
	return symbol.Namespace + "." + symbol.Name
}

func stubBehaviorMemberID(typeName string, member typesys.MemberSymbol) string {
	if member.Kind == apexast.DeclarationConstructor {
		return typeName + ".<init>(" + strings.Join(stubBehaviorParameterTypes(member.Parameters), ",") + ")"
	}
	return typeName + "." + member.Name + "(" + strings.Join(stubBehaviorParameterTypes(member.Parameters), ",") + ")"
}

func stubBehaviorMemberStatic(member typesys.MemberSymbol) bool {
	for _, modifier := range member.Modifiers {
		if strings.EqualFold(modifier, "static") {
			return true
		}
	}
	return false
}

func stubBehaviorParameterTypes(params []apexast.Parameter) []string {
	out := make([]string, 0, len(params))
	for _, param := range params {
		out = append(out, param.Type)
	}
	return out
}

func normalizeStubBehaviorKey(api string) string {
	api = strings.TrimSpace(api)
	api = strings.TrimSuffix(api, "()")
	return strings.ToLower(api)
}
