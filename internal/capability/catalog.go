package capability

import (
	"encoding/json"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/apexdocs"
)

const CatalogSchemaVersion = 1

type SupportTarget string

const (
	TargetExecutableParity SupportTarget = "executable-parity"
	TargetLocalModel       SupportTarget = "local-model"
	TargetTypedStub        SupportTarget = "typed-stub"
	TargetUnsupported      SupportTarget = "unsupported"
)

type Catalog struct {
	SchemaVersion   int              `json:"schemaVersion"`
	SourceDocuments int              `json:"sourceDocuments"`
	SourceMembers   int              `json:"sourceMembers"`
	Entries         []CatalogEntry   `json:"entries"`
	Summary         []CatalogSummary `json:"summary"`
}

type CatalogEntry struct {
	ID         string        `json:"id"`
	Area       string        `json:"area"`
	Namespace  string        `json:"namespace,omitempty"`
	TypeName   string        `json:"typeName,omitempty"`
	MemberName string        `json:"memberName,omitempty"`
	Symbol     string        `json:"symbol"`
	Kind       string        `json:"kind"`
	Signature  string        `json:"signature,omitempty"`
	Target     SupportTarget `json:"target"`
	Status     Status        `json:"status"`
	Owner      string        `json:"owner,omitempty"`
	DocsSource string        `json:"docsSource,omitempty"`
	Evidence   []string      `json:"evidence,omitempty"`
	Notes      string        `json:"notes,omitempty"`
}

type CatalogSummary struct {
	Area      string        `json:"area"`
	Target    SupportTarget `json:"target"`
	Status    Status        `json:"status"`
	Entries   int           `json:"entries"`
	Documents int           `json:"documents,omitempty"`
	Members   int           `json:"members,omitempty"`
}

func BuildCatalog(inv apexdocs.Inventory) Catalog {
	known := knownStdlibEntries()
	entries := make([]CatalogEntry, 0, inv.TotalFiles+inv.TotalMembers)
	for _, doc := range inv.Documents {
		classification := classifyDoc(doc)
		docSymbol := catalogSymbol(doc.Namespace, doc.Name, "")
		docEntry := CatalogEntry{
			ID:         catalogID(docSymbol, doc.Kind, ""),
			Area:       classification.area,
			Namespace:  emptyNone(doc.Namespace),
			TypeName:   doc.Name,
			Symbol:     docSymbol,
			Kind:       doc.Kind,
			Target:     classification.target,
			Status:     StatusUnknown,
			Owner:      classification.owner,
			DocsSource: doc.SourcePath,
		}
		if match, ok := known[doc.Name]; ok && (doc.Namespace == "" || doc.Namespace == "System") {
			docEntry.Status = match.Status
			docEntry.Notes = match.Notes
		}
		entries = append(entries, docEntry)
		for _, member := range doc.Members {
			symbol := catalogSymbol(doc.Namespace, doc.Name, member.Name)
			entry := CatalogEntry{
				ID:         catalogID(symbol, member.Kind, member.Signature),
				Area:       classification.area,
				Namespace:  emptyNone(doc.Namespace),
				TypeName:   doc.Name,
				MemberName: member.Name,
				Symbol:     symbol,
				Kind:       member.Kind,
				Signature:  member.Signature,
				Target:     classification.target,
				Status:     StatusUnknown,
				Owner:      classification.owner,
				DocsSource: doc.SourcePath,
			}
			if match, ok := known[knownKey(doc.Name, member.Name)]; ok {
				entry.Status = match.Status
				entry.Notes = match.Notes
			}
			entries = append(entries, entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})
	catalog := Catalog{
		SchemaVersion:   CatalogSchemaVersion,
		SourceDocuments: inv.TotalFiles,
		SourceMembers:   inv.TotalMembers,
		Entries:         entries,
	}
	catalog.Summary = summarizeCatalog(entries)
	return catalog
}

func WriteCatalogJSON(w io.Writer, catalog Catalog) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(catalog)
}

func ReadCatalog(path string) (Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Catalog{}, err
	}
	var catalog Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

type docClassification struct {
	area   string
	target SupportTarget
	owner  string
}

func classifyDoc(doc apexdocs.Document) docClassification {
	ns := canonicalNamespace(doc.Namespace)
	name := canonicalName(doc.Name)
	switch {
	case isCoreStdlib(ns, name):
		return docClassification{area: "Core stdlib", target: TargetExecutableParity, owner: "internal/vm"}
	case isDataPlatform(ns, name):
		return docClassification{area: "Data platform", target: TargetLocalModel, owner: ownerForDataPlatform(name)}
	case isTestsAsyncLimits(ns, name):
		return docClassification{area: "Tests, async, and limits", target: TargetLocalModel, owner: "internal/vm"}
	case isIntegrationSecurityUI(ns, name):
		return docClassification{area: "Integration, security, and UI", target: TargetLocalModel, owner: "internal/vm"}
	case ns == "System":
		return docClassification{area: "Core stdlib", target: TargetExecutableParity, owner: "internal/vm"}
	case ns == "":
		return docClassification{area: "Language and guide docs", target: TargetUnsupported, owner: "internal/apexast"}
	default:
		return docClassification{area: "Product namespaces", target: TargetTypedStub, owner: "generated declarations"}
	}
}

func isCoreStdlib(ns, name string) bool {
	if ns != "System" && ns != "" {
		return false
	}
	return stringIn(name, []string{
		"Blob", "Boolean", "Date", "Datetime", "Decimal", "Double", "EncodingUtil", "Enum", "Exception",
		"Id", "Integer", "JSON", "JSONGenerator", "JSONParser", "List", "Long", "Map", "Math", "Matcher",
		"Object", "Pattern", "Set", "String", "System", "Time", "TimeZone", "Type", "URL", "Version",
	})
}

func isDataPlatform(ns, name string) bool {
	if stringIn(ns, []string{"Database", "DataSource", "Schema", "Search"}) {
		return true
	}
	return stringIn(name, []string{
		"AggregateResult", "Approval", "Custom", "Database", "DescribeFieldResult", "DescribeSObjectResult",
		"DescribeTabResult", "DMLOptions", "DuplicateResult", "EmptyRecycleBinResult", "Error", "FieldSet",
		"FieldSetMember", "LeadConvert", "LeadConvertResult", "MergeResult", "QueryLocator", "SaveResult",
		"Schema", "Search", "SObject", "UndeleteResult", "UpsertResult",
	})
}

func isTestsAsyncLimits(ns, name string) bool {
	if stringIn(name, []string{
		"AsyncInfo", "AsyncOptions", "Batchable", "BatchableContext", "Finalizer", "FinalizerContext",
		"Future", "Limits", "Queueable", "QueueableContext", "Schedulable", "SchedulableContext", "Test",
	}) {
		return true
	}
	return strings.Contains(name, "Mock")
}

func isIntegrationSecurityUI(ns, name string) bool {
	if stringIn(ns, []string{"ApexPages", "Auth", "Canvas", "EventBus", "Messaging", "QuickAction", "Site", "VisualEditor"}) {
		return true
	}
	return stringIn(name, []string{
		"ApexPages", "Continuation", "FeatureManagement", "Http", "HttpRequest", "HttpResponse", "PageReference",
		"RestContext", "RestRequest", "RestResponse", "StaticResourceCalloutMock", "UserInfo",
	})
}

func ownerForDataPlatform(name string) string {
	switch {
	case strings.Contains(name, "Query") || name == "Search":
		return "internal/soql"
	case strings.Contains(name, "Result") || name == "Database" || name == "DMLOptions":
		return "internal/dml"
	case strings.Contains(name, "Describe") || name == "Schema" || name == "FieldSet" || name == "FieldSetMember":
		return "internal/schema"
	default:
		return "internal/storage"
	}
}

func summarizeCatalog(entries []CatalogEntry) []CatalogSummary {
	type key struct {
		area   string
		target SupportTarget
		status Status
	}
	counts := map[key]*CatalogSummary{}
	for _, entry := range entries {
		k := key{area: entry.Area, target: entry.Target, status: entry.Status}
		summary := counts[k]
		if summary == nil {
			summary = &CatalogSummary{Area: entry.Area, Target: entry.Target, Status: entry.Status}
			counts[k] = summary
		}
		summary.Entries++
		if entry.MemberName == "" {
			summary.Documents++
		} else {
			summary.Members++
		}
	}
	out := make([]CatalogSummary, 0, len(counts))
	for _, summary := range counts {
		out = append(out, *summary)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Area != out[j].Area {
			return out[i].Area < out[j].Area
		}
		if out[i].Target != out[j].Target {
			return out[i].Target < out[j].Target
		}
		return out[i].Status < out[j].Status
	})
	return out
}

func knownStdlibEntries() map[string]StdlibEntry {
	known := map[string]StdlibEntry{}
	for _, entry := range StdlibMatrix() {
		known[entry.API] = entry
	}
	return known
}

func knownKey(typeName, memberName string) string {
	if typeName == "" {
		return memberName
	}
	if memberName == "" {
		return typeName
	}
	return typeName + "." + memberName
}

func catalogSymbol(namespace, typeName, memberName string) string {
	parts := make([]string, 0, 3)
	if namespace != "" && namespace != "(none)" && namespace != "System" {
		parts = append(parts, namespace)
	}
	if typeName != "" {
		parts = append(parts, typeName)
	}
	if memberName != "" {
		parts = append(parts, memberName)
	}
	return strings.Join(parts, ".")
}

func catalogID(symbol, kind, signature string) string {
	id := strings.ToLower(symbol)
	id = strings.ReplaceAll(id, ".", "/")
	id = strings.ReplaceAll(id, " ", "-")
	if signature != "" {
		id += "#" + strings.ToLower(signature)
	}
	if kind != "" {
		id += ":" + strings.ToLower(kind)
	}
	replacer := strings.NewReplacer("(", "-", ")", "", ",", "-", "<", "-", ">", "-", "[", "-", "]", "-", ":", "-", "__", "_")
	id = replacer.Replace(id)
	for strings.Contains(id, "--") {
		id = strings.ReplaceAll(id, "--", "-")
	}
	return strings.Trim(id, "-")
}

func canonicalNamespace(namespace string) string {
	namespace = emptyNone(namespace)
	if namespace == "(none)" {
		return ""
	}
	if strings.EqualFold(namespace, "system") {
		return "System"
	}
	return namespace
}

func canonicalName(name string) string {
	for _, suffix := range []string{" Class", " Methods", " Method"} {
		name = strings.TrimSuffix(name, suffix)
	}
	return strings.TrimSpace(name)
}

func emptyNone(value string) string {
	if value == "(none)" {
		return ""
	}
	return value
}

func stringIn(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.EqualFold(value, candidate) {
			return true
		}
	}
	return false
}
