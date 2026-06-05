package surfaceledger

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/capability"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func BuildGladeSnapshot() []SurfaceLedgerRow {
	byID := map[string]SurfaceLedgerRow{}
	for _, symbol := range typesys.StandardPlatformSymbolView() {
		namespace, typeName := splitTypeName(symbol.Namespace, symbol.Name)
		id := ApexTypeID(namespace, typeName)
		byID[surfaceIDKey(id)] = RowFromGladeShape(SurfaceLedgerRow{
			SurfaceID:     id,
			Product:       ProductApex,
			Area:          AreaRuntime,
			Namespace:     namespace,
			TypeName:      typeName,
			Kind:          KindType,
			GladeBehavior: behaviorForStandardType(namespace, typeName),
			Sources:       []string{"standard-symbols"},
		})
		for _, member := range symbol.Members {
			params := memberParameterTypes(member.Parameters)
			memberName := member.Name
			if memberName == "" {
				memberName = typeName
			}
			kind := gladeMemberKind(string(member.Kind))
			if string(member.Kind) == "constructor" {
				memberName = gladeConstructorMemberName(namespace, typeName, memberName)
			}
			if kind == KindProperty {
				params = nil
			}
			memberID := ApexMemberID(namespace, typeName, memberName, params)
			row := RowFromGladeShape(SurfaceLedgerRow{
				SurfaceID:  memberID,
				Product:    ProductApex,
				Area:       AreaRuntime,
				Namespace:  namespace,
				TypeName:   typeName,
				MemberName: memberName,
				Kind:       kind,
				ReturnType: member.Type,
				Parameters: params,
				Sources:    []string{"standard-symbols"},
			})
			if string(member.Kind) == "constructor" && messagingInboundEmailDTOType(namespace, typeName) {
				row.GladeBehavior = BehaviorPassive
			}
			byID[surfaceIDKey(memberID)] = row
		}
	}
	for _, entry := range capability.StdlibMatrix() {
		id := idFromStdlibAPI(entry.API)
		key := surfaceIDKey(id)
		row := byID[key]
		if row.SurfaceID == "" {
			row = SurfaceLedgerRow{SurfaceID: id, Product: ProductApex, Area: AreaRuntime, Kind: KindMethod, Sources: []string{"stdlib-matrix"}}
			fillFromApexID(&row)
		}
		row.GladeBehavior = behaviorFromCapabilityStatus(entry.Status)
		row.Notes = entry.Notes
		row.Sources = mergeStrings(row.Sources, []string{"stdlib-matrix"})
		byID[key] = withDefaults(row)
	}
	for _, entry := range capability.BuildStubBehaviorReport().Entries {
		id := idFromStubBehavior(entry)
		key := surfaceIDKey(id)
		row := byID[key]
		if row.SurfaceID == "" {
			row = SurfaceLedgerRow{SurfaceID: id, Product: ProductApex, Area: AreaRuntime, Kind: gladeMemberKind(entry.Kind), Sources: []string{"stub-behavior"}}
			fillFromApexID(&row)
		}
		row.GladeBehavior = mergeGladeBehavior(row.GladeBehavior, behaviorFromStubStatus(entry.Status))
		row.ReturnType = firstNonEmpty(row.ReturnType, entry.ReturnType)
		if len(row.Parameters) == 0 {
			row.Parameters = append([]string(nil), entry.Parameters...)
		}
		row.Notes = firstNonEmpty(row.Notes, entry.Notes)
		row.Sources = mergeStrings(row.Sources, []string{"stub-behavior"})
		byID[key] = withDefaults(row)
	}
	addDataReferenceGladeRows(byID)
	addLocalTestLWCGladeRows(byID)
	rows := make([]SurfaceLedgerRow, 0, len(byID))
	for _, row := range byID {
		rows = append(rows, withDefaults(row))
	}
	sortRows(rows)
	return rows
}

func mergeGladeBehavior(existing, next BehaviorState) BehaviorState {
	if existing == BehaviorUnsupported {
		return existing
	}
	if existing == BehaviorSupported && (next == BehaviorPassive || next == BehaviorUnsupported) {
		return existing
	}
	if existing == BehaviorPartial && next == BehaviorUnsupported {
		return existing
	}
	if next == "" || next == BehaviorNone {
		return existing
	}
	return next
}

func behaviorForStandardType(namespace, typeName string) BehaviorState {
	if strings.EqualFold(namespace, "Database") && strings.EqualFold(typeName, "Stateful") {
		return BehaviorSupported
	}
	return BehaviorNone
}

func addDataReferenceGladeRows(byID map[string]SurfaceLedgerRow) {
	for _, objectName := range storage.KnownStandardObjectNames() {
		definition, ok := storage.StandardObjectDefinition(objectName)
		if !ok {
			continue
		}
		objectID := DataObjectID(definition.APIName)
		byID[surfaceIDKey(objectID)] = RowFromGeneratedDataReferenceShape(SurfaceLedgerRow{
			SurfaceID:     objectID,
			Product:       ProductDataRef,
			Area:          AreaData,
			TypeName:      definition.APIName,
			Kind:          KindType,
			GladeBehavior: BehaviorSupported,
			Sources:       []string{SourceStandardSObjectGeneratedShape},
		})
		for _, field := range definition.Fields {
			fieldID := DataFieldID(definition.APIName, field.APIName)
			byID[surfaceIDKey(fieldID)] = RowFromGeneratedDataReferenceShape(SurfaceLedgerRow{
				SurfaceID:     fieldID,
				Product:       ProductDataRef,
				Area:          AreaData,
				TypeName:      definition.APIName,
				FieldName:     field.APIName,
				Kind:          KindField,
				ReturnType:    string(field.Type),
				GladeBehavior: BehaviorSupported,
				Sources:       []string{SourceStandardSObjectGeneratedShape},
			})
		}
	}
}

func addLocalTestLWCGladeRows(byID map[string]SurfaceLedgerRow) {
	for _, module := range localTestUnsupportedLWCModules {
		id := LWCModuleID(module)
		byID[surfaceIDKey(id)] = RowFromGladeShape(SurfaceLedgerRow{
			SurfaceID:     id,
			Product:       ProductLWC,
			Area:          AreaUI,
			TypeName:      module,
			Kind:          KindModule,
			GladeBehavior: BehaviorUnsupported,
			Sources:       []string{"uicontroller-import-shape"},
			Notes:         "local Apex tests can index this LWC import shape; browser or service execution is not modeled locally",
		})
	}
}

var localTestUnsupportedLWCModules = []string{
	"Decorators",
	"HTML",
	"LWC",
	"PageReference",
	"Salesforce",
	"Standard",
	"XML",
	"`@salesforce`",
	"`experience/blockBuilderApi`",
	"`experience/cms*Api`",
	"`experience/cmsEditorApi`",
	"`lightning/analyticsWaveApi`",
	"`lightning/graphql`",
	"`lightning/industriesEducationPublicApi`",
	"`lightning/mobileCapabilities`",
	"`lightning/serviceKnowledgeApi`",
	"`lightning/ui*Api`",
	"`lightning/uiGraphQLApi`",
	"`notifyRecordUpdateAvailable(recordIds)`",
	"lightning/cmsDeliveryApi",
}

func splitTypeName(namespace, name string) (string, string) {
	if namespace != "" {
		return namespace, strings.TrimPrefix(name, namespace+".")
	}
	if idx := strings.LastIndex(name, "."); idx > 0 {
		return name[:idx], name[idx+1:]
	}
	return "System", name
}

func gladeConstructorMemberName(namespace, typeName, memberName string) string {
	if !strings.EqualFold(memberName, lastTypeSegment(typeName)) {
		return memberName
	}
	if strings.EqualFold(namespace, "Messaging.InboundEmail") {
		return "InboundEmail." + typeName
	}
	return typeName
}

func messagingInboundEmailDTOType(namespace, typeName string) bool {
	if !strings.EqualFold(namespace, "Messaging.InboundEmail") {
		return false
	}
	switch typeName {
	case "AuthenticationResult", "AuthenticationResultField", "BinaryAttachment", "TextAttachment":
		return true
	default:
		return false
	}
}

func lastTypeSegment(typeName string) string {
	if idx := strings.LastIndexByte(typeName, '.'); idx >= 0 && idx < len(typeName)-1 {
		return typeName[idx+1:]
	}
	return typeName
}

func memberParameterTypes(params []apexast.Parameter) []string {
	out := make([]string, 0, len(params))
	for _, param := range params {
		out = append(out, param.Type)
	}
	return cleanList(out)
}

func gladeMemberKind(kind string) string {
	switch kind {
	case "method", "constructor":
		return KindMethod
	case "property":
		return KindProperty
	case "field":
		return KindField
	default:
		return KindType
	}
}

func idFromStdlibAPI(api string) string {
	api = strings.TrimSpace(api)
	parts := strings.SplitN(api, ".", 2)
	if len(parts) != 2 {
		return ApexTypeID("System", api)
	}
	params := []string(nil)
	member := parts[1]
	if open := strings.IndexByte(member, '('); open >= 0 && strings.HasSuffix(member, ")") {
		rawParams := strings.TrimSuffix(member[open+1:], ")")
		member = member[:open]
		params = splitSurfaceParameterList(rawParams)
	} else if member == "contains" {
		params = []string{"String"}
	}
	return ApexMemberID("System", parts[0], member, params)
}

func splitSurfaceParameterList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var params []string
	depth := 0
	start := 0
	for i, r := range raw {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				params = append(params, strings.TrimSpace(raw[start:i]))
				start = i + len(string(r))
			}
		}
	}
	params = append(params, strings.TrimSpace(raw[start:]))
	return params
}

func idFromStubBehavior(entry capability.StubBehaviorEntry) string {
	namespace, typeName := splitTypeName("", entry.Type)
	if entry.Member == "" {
		return ApexTypeID(namespace, typeName)
	}
	if gladeMemberKind(entry.Kind) == KindProperty {
		return ApexMemberID(namespace, typeName, entry.Member, nil)
	}
	return ApexMemberID(namespace, typeName, entry.Member, entry.Parameters)
}

func behaviorFromCapabilityStatus(status capability.Status) BehaviorState {
	switch status {
	case capability.StatusSupported:
		return BehaviorSupported
	case capability.StatusPartial:
		return BehaviorPartial
	case capability.StatusUnsupported:
		return BehaviorUnsupported
	case capability.StatusStub:
		return BehaviorPassive
	default:
		return BehaviorNone
	}
}

func behaviorFromStubStatus(status capability.StubBehaviorStatus) BehaviorState {
	switch status {
	case capability.StubBehaviorImplemented:
		return BehaviorSupported
	case capability.StubBehaviorPassiveDefault:
		return BehaviorPassive
	case capability.StubBehaviorUnsupported:
		return BehaviorUnsupported
	default:
		return BehaviorNone
	}
}

func fillFromApexID(row *SurfaceLedgerRow) {
	if row == nil || !strings.HasPrefix(row.SurfaceID, "apex:") {
		return
	}
	rest := strings.TrimPrefix(row.SurfaceID, "apex:")
	if dot := strings.LastIndex(rest, "."); dot > 0 {
		row.Namespace = rest[:dot]
		member := rest[dot+1:]
		if paren := strings.Index(member, "("); paren >= 0 {
			row.MemberName = member[:paren]
			typePart := rest[:dot]
			if typeDot := strings.LastIndex(typePart, "."); typeDot > 0 {
				row.Namespace = typePart[:typeDot]
				row.TypeName = typePart[typeDot+1:]
			}
			return
		}
		if row.Kind == KindProperty || row.Kind == KindField {
			row.MemberName = member
			typePart := rest[:dot]
			if typeDot := strings.LastIndex(typePart, "."); typeDot > 0 {
				row.Namespace = typePart[:typeDot]
				row.TypeName = typePart[typeDot+1:]
			}
			return
		}
		row.TypeName = member
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
