package surfaceledger

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/glade-sh/glade/internal/apexdocs"
)

var apiVersionPattern = regexp.MustCompile(`(?i)Available in API version\s+([0-9]+(?:\.[0-9]+)?)`)

func BuildDocsSnapshot(source string) ([]SurfaceLedgerRow, error) {
	inv, err := apexdocs.BuildInventory(source)
	if err != nil {
		return nil, err
	}
	rows := RowsFromDocsInventory(inv)
	applyApexDeclarationSignatures(rows, source)
	rows = filterUnresolvedApexHeadingRows(rows)
	for i := range rows {
		if rows[i].DocsSource == "" {
			continue
		}
		rows[i].APIVersion = readAPIVersion(filepath.Join(source, filepath.FromSlash(rows[i].DocsSource)))
	}
	return rows, nil
}

func applyApexDeclarationSignatures(rows []SurfaceLedgerRow, source string) {
	cache := map[string]map[string][]string{}
	for i := range rows {
		row := &rows[i]
		if row.Product != ProductApex || row.Kind != KindMethod || row.MemberName == "" || row.DocsSource == "" {
			continue
		}
		signatures, ok := cache[row.DocsSource]
		if !ok {
			signatures = apexDeclarationSignatures(filepath.Join(source, filepath.FromSlash(row.DocsSource)))
			cache[row.DocsSource] = signatures
		}
		signature := matchingSignature(signatures[row.MemberName], len(row.Parameters))
		if signature == "" {
			continue
		}
		params := parametersFromSignature(signature)
		if len(params) == 0 && strings.Contains(signature, "()") {
			params = []string{}
		}
		if params == nil {
			continue
		}
		row.Signature = signature
		row.Parameters = params
		row.SurfaceID = ApexMemberID(row.Namespace, row.TypeName, row.MemberName, params)
	}
	sortRows(rows)
}

func filterUnresolvedApexHeadingRows(rows []SurfaceLedgerRow) []SurfaceLedgerRow {
	out := rows[:0]
	for _, row := range rows {
		if row.Product == ProductApex && row.Kind == KindMethod && isApexHeadingOnlySignature(row.Signature) {
			continue
		}
		out = append(out, row)
	}
	return out
}

func apexDeclarationSignatures(path string) map[string][]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	signatures := map[string][]string{}
	titleMember := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			titleMember = memberNameFromHeading(strings.TrimSpace(strings.TrimPrefix(line, "# ")))
			break
		}
	}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "## Signature" && titleMember != "" {
			if signature := readInlineCodeBlock(lines[i+1:]); signature != "" {
				signatures[titleMember] = append(signatures[titleMember], signature)
			}
			continue
		}
		if !strings.HasPrefix(line, "### ") {
			continue
		}
		member := memberNameFromHeading(strings.TrimSpace(strings.TrimPrefix(line, "### ")))
		if member == "" {
			continue
		}
		for j := i + 1; j < len(lines); j++ {
			next := strings.TrimSpace(lines[j])
			if strings.HasPrefix(next, "### ") {
				break
			}
			if next != "#### Signature" {
				continue
			}
			if signature := readInlineCodeBlock(lines[j+1:]); signature != "" {
				signatures[member] = append(signatures[member], signature)
			}
			break
		}
	}
	return signatures
}

func matchingSignature(signatures []string, parameterCount int) string {
	for _, signature := range signatures {
		if len(parametersFromSignature(signature)) == parameterCount {
			return signature
		}
	}
	if len(signatures) == 1 {
		return signatures[0]
	}
	return ""
}

func memberNameFromHeading(heading string) string {
	if idx := strings.Index(heading, "("); idx > 0 {
		return strings.TrimSpace(heading[:idx])
	}
	return strings.TrimSpace(heading)
}

func readInlineCodeBlock(lines []string) string {
	var b strings.Builder
	inCode := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#### ") || strings.HasPrefix(strings.TrimSpace(line), "### ") {
			break
		}
		for _, r := range line {
			if r == '`' {
				if inCode {
					return strings.Join(strings.Fields(b.String()), " ")
				}
				inCode = true
				continue
			}
			if inCode {
				b.WriteRune(r)
			}
		}
		if inCode {
			b.WriteByte(' ')
		}
	}
	return ""
}

func RowsFromDocsInventory(inv apexdocs.Inventory) []SurfaceLedgerRow {
	var rows []SurfaceLedgerRow
	for _, doc := range inv.Documents {
		product := ProductFromSourcePath(doc.SourcePath)
		if !shouldIncludeDocsSurface(product, doc) {
			continue
		}
		namespace := docsNamespace(product, doc)
		typeName := doc.Name
		if product == ProductDataRef {
			typeName = dataReferenceDocsName(doc)
		}
		if product == ProductApex {
			identity := apexDocsIdentity(doc)
			namespace = identity.namespace
			typeName = identity.typeName
			if identity.memberName != "" {
				rows = append(rows, RowFromDocs(SurfaceLedgerRow{
					SurfaceID:  ApexMemberID(namespace, typeName, identity.memberName, identity.parameters),
					Product:    product,
					Area:       areaForProduct(product),
					Namespace:  namespace,
					TypeName:   typeName,
					MemberName: identity.memberName,
					Kind:       identity.kind,
					Signature:  identity.signature,
					Parameters: identity.parameters,
					DocsSource: doc.SourcePath,
					DocsTitle:  doc.Title,
					Sources:    []string{"docs"},
				}))
				continue
			}
		}
		surfaceID := docsSurfaceID(product, doc, apexdocs.Member{})
		if product == ProductApex {
			surfaceID = ApexTypeID(namespace, typeName)
		}
		if shouldEmitDocsDocumentRow(product, doc) {
			row := RowFromDocs(SurfaceLedgerRow{
				SurfaceID:  surfaceID,
				Product:    product,
				Area:       areaForProduct(product),
				Namespace:  namespace,
				TypeName:   typeName,
				Kind:       docsDocumentKind(product, doc.Kind),
				DocsSource: doc.SourcePath,
				DocsTitle:  doc.Title,
				Sources:    []string{"docs"},
			})
			rows = append(rows, row)
		}
		apexRealSignatures := apexMembersWithRealSignatures(doc.Members)
		for _, member := range doc.Members {
			if product == ProductApex && isApexHeadingOnlySignature(member.Signature) && apexRealSignatures[member.Name] {
				continue
			}
			surfaceID := docsSurfaceID(product, doc, member)
			if product == ProductApex {
				surfaceID = ApexMemberID(namespace, typeName, member.Name, parametersFromSignature(member.Signature))
			}
			rows = append(rows, RowFromDocs(SurfaceLedgerRow{
				SurfaceID:  surfaceID,
				Product:    product,
				Area:       areaForProduct(product),
				Namespace:  namespace,
				TypeName:   typeName,
				MemberName: member.Name,
				Kind:       docsKind(product, member.Kind),
				Signature:  member.Signature,
				Parameters: parametersFromSignature(member.Signature),
				DocsSource: doc.SourcePath,
				DocsTitle:  doc.Title,
				Sources:    []string{"docs"},
			}))
		}
	}
	sortRows(rows)
	return rows
}

type apexDocsDocumentIdentity struct {
	namespace  string
	typeName   string
	memberName string
	signature  string
	parameters []string
	kind       string
}

func apexDocsIdentity(doc apexdocs.Document) apexDocsDocumentIdentity {
	identity := apexDocsDocumentIdentity{
		namespace: docsNamespace(ProductApex, doc),
		typeName:  doc.Name,
		kind:      docsKind(ProductApex, doc.Kind),
	}
	namespace, typeName, memberName, ok := inferApexIdentityFromSource(doc.SourcePath, doc.Name)
	if !ok {
		return identity
	}
	identity.namespace = namespace
	if len(doc.Members) > 0 {
		return identity
	}
	identity.typeName = typeName
	if memberName == "" {
		return identity
	}
	identity.memberName = memberName
	identity.signature = doc.Name
	identity.parameters = parametersFromSignature(doc.Name)
	identity.kind = KindMethod
	return identity
}

func apexMembersWithRealSignatures(members []apexdocs.Member) map[string]bool {
	out := map[string]bool{}
	for _, member := range members {
		if isApexRealSignature(member.Signature) {
			out[member.Name] = true
		}
	}
	return out
}

func isApexHeadingOnlySignature(signature string) bool {
	signature = strings.TrimSpace(signature)
	if signature == "" || isApexRealSignature(signature) {
		return false
	}
	return strings.Contains(signature, "(") && strings.Contains(signature, ")")
}

func isApexRealSignature(signature string) bool {
	signature = strings.TrimSpace(signature)
	first := strings.Fields(signature)
	if len(first) == 0 {
		return false
	}
	switch strings.ToLower(first[0]) {
	case "public", "private", "protected", "global", "webservice", "static":
		return true
	default:
		return false
	}
}

func shouldIncludeDocsSurface(product string, doc apexdocs.Document) bool {
	if product != ProductApex {
		return true
	}
	if isApexGuideDoc(doc.SourcePath) || isApexNamespaceDoc(doc.SourcePath) || isGeneratedCustomObjectMethodDoc(doc.SourcePath) {
		return false
	}
	return true
}

func shouldEmitDocsDocumentRow(product string, doc apexdocs.Document) bool {
	if product != ProductDataRef {
		return true
	}
	return isDataReferenceObjectDoc(doc.SourcePath)
}

func isDataReferenceObjectDoc(sourcePath string) bool {
	path := filepath.ToSlash(sourcePath)
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[len(parts)-2] != "object-reference" {
		return false
	}
	stem := strings.ToLower(sourceStemBase(sourcePath))
	if strings.HasPrefix(stem, "sforce_api_objects_eventlogfile_") {
		return true
	}
	if !strings.HasPrefix(stem, "sforce_api_objects_") {
		return false
	}
	rest := strings.TrimPrefix(stem, "sforce_api_objects_")
	switch rest {
	case "concepts", "custom_object__c", "custom_objects", "custommetadatatype__mdt", "customobject__feed", "external_objects", "fequently_occurring_fields", "frequently_occurring_fields", "list", "salesforce_surveys_object_model":
		return false
	default:
		return rest != ""
	}
}

func isApexGuideDoc(sourcePath string) bool {
	base := strings.ToLower(sourceStemBase(sourcePath))
	if strings.HasSuffix(base, "_exceptions") || strings.HasPrefix(base, "apex_exceptions_") || strings.HasPrefix(base, "apex_shopping_cart_example") {
		return true
	}
	switch base {
	case "apex_dml_section", "apex_appendices", "apex_qs_conventions", "apex_ref_guide", "apex_releasenotes", "apex_reserved_words", "versioned_behavior_changes":
		return true
	default:
		return false
	}
}

func isApexNamespaceDoc(sourcePath string) bool {
	return strings.HasPrefix(strings.ToLower(sourceStemBase(sourcePath)), "apex_namespace_")
}

func isGeneratedCustomObjectMethodDoc(sourcePath string) bool {
	switch sourceStemBase(sourcePath) {
	case "apex_methods_system_custom_metadata_types", "apex_methods_system_custom_settings":
		return true
	default:
		return false
	}
}

func sourceStemBase(sourcePath string) string {
	return strings.TrimSuffix(filepath.Base(filepath.ToSlash(sourcePath)), filepath.Ext(sourcePath))
}

func ProductFromSourcePath(sourcePath string) string {
	parts := strings.Split(filepath.ToSlash(sourcePath), "/")
	for _, part := range parts {
		switch strings.ToLower(part) {
		case "apex":
			return ProductApex
		case "rest-api", "rest_api":
			return ProductREST
		case "tooling-api", "tooling_api":
			return ProductTooling
		case "visualforce":
			return ProductVisualforce
		case "lightning-aura", "aura":
			return ProductAura
		case "lwc", "lightning-web-components":
			return ProductLWC
		case "object-reference", "field-reference":
			return ProductDataRef
		}
	}
	return ProductUnknown
}

func docsSurfaceID(product string, doc apexdocs.Document, member apexdocs.Member) string {
	switch product {
	case ProductApex:
		ns := docsNamespace(product, doc)
		if member.Name == "" {
			return ApexTypeID(ns, doc.Name)
		}
		return ApexMemberID(ns, doc.Name, member.Name, parametersFromSignature(member.Signature))
	case ProductTooling:
		if member.Name == "" {
			return ToolingObjectID(doc.Name)
		}
		return ToolingFieldID(doc.Name, member.Name)
	case ProductDataRef:
		if member.Name == "" {
			return DataObjectID(dataReferenceDocsName(doc))
		}
		return DataFieldID(dataReferenceDocsName(doc), member.Name)
	case ProductREST:
		if member.Name == "" {
			return RestResourceID(sourceStem(doc.SourcePath), "get")
		}
		return RestResourceID(sourceStem(doc.SourcePath), member.Name)
	case ProductVisualforce:
		if member.Name == "" {
			return "visualforce:" + sourceStem(doc.SourcePath)
		}
		return VisualforceAttrID(docsNamespace(product, doc), strings.TrimPrefix(strings.ToLower(doc.Name), "apex:"), member.Name)
	case ProductAura:
		return AuraID(sourceStem(doc.SourcePath))
	case ProductLWC:
		return LWCModuleID(doc.Name)
	default:
		if member.Name == "" {
			return product + ":" + sourceStem(doc.SourcePath)
		}
		return product + ":" + sourceStem(doc.SourcePath) + "." + member.Name
	}
}

func dataReferenceDocsName(doc apexdocs.Document) string {
	stem := sourceStemBase(doc.SourcePath)
	if strings.HasPrefix(stem, "sforce_api_objects_eventlogfile_") {
		return "EventLogFile"
	}
	if isDataReferenceObjectDoc(doc.SourcePath) {
		if name := dataReferenceNameFromTitle(doc.Title); name != "" {
			return name
		}
	}
	return cleanIdentityPart(doc.Name)
}

func dataReferenceNameFromTitle(title string) string {
	title = cleanIdentityPart(title)
	if idx := strings.IndexByte(title, '('); idx >= 0 {
		title = strings.TrimSpace(title[:idx])
	}
	title = strings.TrimSuffix(title, " Object")
	title = strings.TrimSuffix(title, " object")
	parts := strings.Fields(title)
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	for _, part := range parts {
		part = strings.Trim(part, ",.;:")
		if part == "" {
			continue
		}
		b.WriteString(part)
	}
	return b.String()
}

func docsNamespace(product string, doc apexdocs.Document) string {
	if doc.Namespace != "" {
		if product == ProductApex {
			return canonicalApexNamespaceName(doc.Namespace)
		}
		return doc.Namespace
	}
	if product == ProductApex {
		return inferApexNamespace(doc.SourcePath, doc.Name)
	}
	if product == ProductVisualforce {
		return "apex"
	}
	return ""
}

func inferApexNamespace(sourcePath, name string) string {
	path := strings.ToLower(filepath.ToSlash(sourcePath))
	if strings.Contains(path, "apex_fsccashflow") {
		return "fsccashflow"
	}
	if strings.Contains(path, "apex_canvas") || strings.Contains(path, "/canvas") {
		return "Canvas"
	}
	if strings.Contains(path, "system_") || strings.Contains(path, "/system") || name == "Object" || name == "String" || name == "Label" {
		return "System"
	}
	if strings.Contains(path, "connectapi") {
		return "ConnectApi"
	}
	if strings.Contains(path, "schema") {
		return "Schema"
	}
	return "System"
}

func inferApexIdentityFromSource(sourcePath, name string) (string, string, string, bool) {
	base := sourceStemBase(sourcePath)
	lower := strings.ToLower(base)
	for _, prefix := range []struct {
		key       string
		namespace string
	}{
		{key: "apex_system_", namespace: "System"},
		{key: "apex_messaging_", namespace: "Messaging"},
		{key: "apex_industriesnlpsvc_", namespace: "industriesNlpSvc"},
		{key: "apex_commercepayments_", namespace: "commercepayments"},
		{key: "apex_canvas_", namespace: "Canvas"},
	} {
		if !strings.HasPrefix(lower, prefix.key) {
			continue
		}
		rest := base[len(prefix.key):]
		typeName, memberName := apexTypeAndMemberFromStem(rest, name)
		if typeName == "" {
			return "", "", "", false
		}
		return prefix.namespace, typeName, memberName, true
	}
	return "", "", "", false
}

func apexTypeAndMemberFromStem(rest, name string) (string, string) {
	for _, suffix := range []string{"_methods", "_properties", "_constructors"} {
		if strings.HasSuffix(rest, suffix) {
			return strings.TrimSuffix(rest, suffix), ""
		}
	}
	if idx := strings.Index(rest, "_ctor"); idx > 0 {
		typeName := rest[:idx]
		return typeName, typeName
	}
	open := strings.Index(name, "(")
	if open <= 0 {
		return rest, ""
	}
	if idx := strings.IndexByte(rest, '_'); idx > 0 {
		return rest[:idx], strings.TrimSpace(name[:open])
	}
	return rest, ""
}

func docsKind(product, kind string) string {
	switch product {
	case ProductREST:
		return KindResource
	case ProductDataRef:
		if strings.TrimSpace(kind) == "" || strings.EqualFold(kind, "type") {
			return KindType
		}
		return KindField
	case ProductLWC:
		return KindModule
	case ProductAura:
		return KindGuide
	}
	switch strings.ToLower(kind) {
	case "method", "constructor":
		return KindMethod
	case "property", "member":
		return KindProperty
	case "field":
		return KindField
	default:
		return KindType
	}
}

func docsDocumentKind(product, kind string) string {
	if product == ProductDataRef {
		return KindType
	}
	return docsKind(product, kind)
}

func areaForProduct(product string) string {
	switch product {
	case ProductREST, ProductTooling:
		return AreaServer
	case ProductDataRef:
		return AreaData
	case ProductVisualforce, ProductAura, ProductLWC:
		return AreaUI
	default:
		return AreaRuntime
	}
}

func parametersFromSignature(signature string) []string {
	open := strings.Index(signature, "(")
	close := strings.LastIndex(signature, ")")
	if open < 0 || close < open {
		return nil
	}
	inside := strings.TrimSpace(signature[open+1 : close])
	if inside == "" {
		return []string{}
	}
	parts := splitSurfaceParameterList(inside)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		paramType := parameterTypeFromSignaturePart(part)
		if paramType == "" {
			continue
		}
		out = append(out, paramType)
	}
	return out
}

func parameterTypeFromSignaturePart(part string) string {
	part = strings.TrimSpace(part)
	if part == "" {
		return ""
	}
	depth := 0
	split := -1
	for i, r := range part {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 && unicode.IsSpace(r) {
				split = i
			}
		}
	}
	if split < 0 {
		if genericEnd := stuckGenericParameterNameSplit(part); genericEnd >= 0 {
			return strings.TrimSpace(part[:genericEnd+1])
		}
		return part
	}
	return strings.TrimSpace(part[:split])
}

func stuckGenericParameterNameSplit(part string) int {
	depth := 0
	for i, r := range part {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
			if depth == 0 && i+1 < len(part) {
				next := rune(part[i+1])
				if next == '_' || unicode.IsLetter(next) {
					return i
				}
			}
		}
	}
	return -1
}

func sourceStem(sourcePath string) string {
	sourcePath = strings.TrimSuffix(filepath.ToSlash(sourcePath), filepath.Ext(sourcePath))
	parts := strings.Split(sourcePath, "/")
	if len(parts) > 1 {
		sourcePath = strings.Join(parts[1:], "/")
	}
	return sourcePath
}

func readAPIVersion(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	match := apiVersionPattern.FindStringSubmatch(string(data))
	if len(match) < 2 {
		return ""
	}
	return match[1]
}
