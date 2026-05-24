package capability

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

const SalesforceCoverageSchemaVersion = 1

type SalesforceCoverageReport struct {
	SchemaVersion   int                          `json:"schemaVersion"`
	SourceDocuments int                          `json:"sourceDocuments"`
	SourceMembers   int                          `json:"sourceMembers"`
	Entries         int                          `json:"entries"`
	Areas           []SalesforceCoverageArea     `json:"areas"`
	Totals          SalesforceCoverageTotals     `json:"totals"`
	Tooling         *SalesforceToolingAlignment  `json:"tooling,omitempty"`
	NextGates       []SalesforceCoverageNextGate `json:"nextGates,omitempty"`
}

type SalesforceCoverageTotals struct {
	Supported   int `json:"supported"`
	Partial     int `json:"partial"`
	Stub        int `json:"stub"`
	Unsupported int `json:"unsupported"`
	Unknown     int `json:"unknown"`
}

type SalesforceCoverageArea struct {
	Area        string                    `json:"area"`
	Target      SupportTarget             `json:"target"`
	Entries     int                       `json:"entries"`
	Documents   int                       `json:"documents,omitempty"`
	Members     int                       `json:"members,omitempty"`
	Supported   int                       `json:"supported"`
	Partial     int                       `json:"partial"`
	Stub        int                       `json:"stub"`
	Unsupported int                       `json:"unsupported"`
	Unknown     int                       `json:"unknown"`
	TopUnknown  []SalesforceCoverageEntry `json:"topUnknown,omitempty"`
}

type SalesforceCoverageEntry struct {
	Symbol     string `json:"symbol"`
	Kind       string `json:"kind"`
	Owner      string `json:"owner,omitempty"`
	DocsSource string `json:"docsSource,omitempty"`
}

type SalesforceCoverageNextGate struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

type SalesforceToolingAlignment struct {
	Source                        string                    `json:"source,omitempty"`
	Namespaces                    int                       `json:"namespaces"`
	Classes                       int                       `json:"classes"`
	Constructors                  int                       `json:"constructors"`
	Methods                       int                       `json:"methods"`
	Properties                    int                       `json:"properties"`
	Members                       int                       `json:"members"`
	SystemDefaultNamespaceClasses int                       `json:"systemDefaultNamespaceClasses"`
	SystemDefaultNamespaceMembers int                       `json:"systemDefaultNamespaceMembers"`
	ConcreteRuntimeAPIs           int                       `json:"concreteRuntimeApis"`
	ConcreteRuntimeAPIsInTooling  int                       `json:"concreteRuntimeApisInTooling"`
	ConcreteRuntimeAPIsMissing    int                       `json:"concreteRuntimeApisMissing"`
	CatalogSystemEntries          int                       `json:"catalogSystemEntries"`
	CatalogSystemEntriesInTooling int                       `json:"catalogSystemEntriesInTooling"`
	CatalogSystemEntriesMissing   int                       `json:"catalogSystemEntriesMissing"`
	SymbolTableClasses            int                       `json:"symbolTableClasses,omitempty"`
	SymbolTableConstructors       int                       `json:"symbolTableConstructors,omitempty"`
	SymbolTableMethods            int                       `json:"symbolTableMethods,omitempty"`
	SymbolTableProperties         int                       `json:"symbolTableProperties,omitempty"`
	SymbolTableMembers            int                       `json:"symbolTableMembers,omitempty"`
	MissingRuntimeAPIs            []SalesforceCoverageEntry `json:"missingRuntimeApis,omitempty"`
	MissingCatalogSystemEntries   []SalesforceCoverageEntry `json:"missingCatalogSystemEntries,omitempty"`
}

func BuildSalesforceCoverageReport(catalog Catalog) SalesforceCoverageReport {
	return BuildSalesforceCoverageReportWithTooling(catalog, nil, nil, "")
}

func BuildSalesforceCoverageReportWithTooling(catalog Catalog, tooling *ToolingCompletions, apexClassSymbols *ToolingApexClassSymbols, toolingSource string) SalesforceCoverageReport {
	type key struct {
		area   string
		target SupportTarget
	}
	areas := map[key]*SalesforceCoverageArea{}
	for _, entry := range catalog.Entries {
		k := key{area: entry.Area, target: entry.Target}
		area := areas[k]
		if area == nil {
			area = &SalesforceCoverageArea{Area: entry.Area, Target: entry.Target}
			areas[k] = area
		}
		area.Entries++
		if entry.MemberName == "" {
			area.Documents++
		} else {
			area.Members++
		}
		incrementSalesforceCoverageStatus(&area.Supported, &area.Partial, &area.Stub, &area.Unsupported, &area.Unknown, entry.Status)
		if entry.Status == StatusUnknown && len(area.TopUnknown) < 10 {
			area.TopUnknown = append(area.TopUnknown, SalesforceCoverageEntry{
				Symbol:     entry.Symbol,
				Kind:       entry.Kind,
				Owner:      entry.Owner,
				DocsSource: entry.DocsSource,
			})
		}
	}

	report := SalesforceCoverageReport{
		SchemaVersion:   SalesforceCoverageSchemaVersion,
		SourceDocuments: catalog.SourceDocuments,
		SourceMembers:   catalog.SourceMembers,
		Entries:         len(catalog.Entries),
		NextGates: []SalesforceCoverageNextGate{{
			Name:    "docs inventory",
			Command: "go run ./cmd/glade compat docs-inventory --source <salesforce-docs> --json",
		}, {
			Name:    "capability catalog",
			Command: "go run ./cmd/glade compat catalog --inventory <inventory.json> --json",
		}, {
			Name:    "coverage manifest",
			Command: "go run ./cmd/glade compat salesforce-coverage --source <salesforce-docs> --tooling-completions <completions.json> --tooling-symbols <apex-class-symbols.json> --json",
		}},
	}
	for _, area := range areas {
		report.Areas = append(report.Areas, *area)
		report.Totals.Supported += area.Supported
		report.Totals.Partial += area.Partial
		report.Totals.Stub += area.Stub
		report.Totals.Unsupported += area.Unsupported
		report.Totals.Unknown += area.Unknown
	}
	sort.Slice(report.Areas, func(i, j int) bool {
		if report.Areas[i].Area != report.Areas[j].Area {
			return report.Areas[i].Area < report.Areas[j].Area
		}
		return report.Areas[i].Target < report.Areas[j].Target
	})
	if tooling != nil || apexClassSymbols != nil {
		alignment := BuildSalesforceToolingAlignment(catalog, tooling, apexClassSymbols)
		alignment.Source = toolingSource
		report.Tooling = &alignment
	}
	return report
}

func BuildSalesforceToolingAlignment(catalog Catalog, tooling *ToolingCompletions, apexClassSymbols *ToolingApexClassSymbols) SalesforceToolingAlignment {
	symbols, alignment := flattenToolingSymbols(tooling, apexClassSymbols)

	runtimeAPIs := make([]SalesforceCoverageEntry, 0)
	for _, entry := range StdlibMatrix() {
		if !isConcreteRuntimeAPI(entry.API) {
			continue
		}
		runtimeAPIs = append(runtimeAPIs, SalesforceCoverageEntry{
			Symbol: entry.API,
			Kind:   "runtime-api",
			Owner:  entry.Area,
		})
	}
	alignment.ConcreteRuntimeAPIs = len(runtimeAPIs)
	for _, entry := range runtimeAPIs {
		if _, ok := symbols[strings.ToLower(entry.Symbol)]; ok {
			alignment.ConcreteRuntimeAPIsInTooling++
			continue
		}
		alignment.ConcreteRuntimeAPIsMissing++
		if len(alignment.MissingRuntimeAPIs) < 25 {
			alignment.MissingRuntimeAPIs = append(alignment.MissingRuntimeAPIs, entry)
		}
	}

	for _, entry := range catalog.Entries {
		if entry.Target != TargetExecutableParity && entry.Target != TargetLocalModel {
			continue
		}
		if entry.Symbol == "" {
			continue
		}
		alignment.CatalogSystemEntries++
		if _, ok := symbols[strings.ToLower(entry.Symbol)]; ok {
			alignment.CatalogSystemEntriesInTooling++
			continue
		}
		alignment.CatalogSystemEntriesMissing++
		if len(alignment.MissingCatalogSystemEntries) < 25 {
			alignment.MissingCatalogSystemEntries = append(alignment.MissingCatalogSystemEntries, SalesforceCoverageEntry{
				Symbol:     entry.Symbol,
				Kind:       entry.Kind,
				Owner:      entry.Owner,
				DocsSource: entry.DocsSource,
			})
		}
	}

	return alignment
}

func flattenToolingSymbols(tooling *ToolingCompletions, apexClassSymbols *ToolingApexClassSymbols) (map[string]SalesforceCoverageEntry, SalesforceToolingAlignment) {
	symbols := map[string]SalesforceCoverageEntry{}
	alignment := SalesforceToolingAlignment{}
	if tooling != nil {
		alignment.Namespaces = len(tooling.PublicDeclarations)
		for namespace, classes := range tooling.PublicDeclarations {
			for className, decl := range classes {
				alignment.Classes++
				if strings.EqualFold(namespace, "System") {
					alignment.SystemDefaultNamespaceClasses++
				}
				addToolingSymbol(symbols, namespace, className, "", "class")
				alignment.Constructors += len(decl.Constructors)
				alignment.Methods += len(decl.Methods)
				alignment.Properties += len(decl.Properties)
				alignment.Members += len(decl.Constructors) + len(decl.Methods) + len(decl.Properties)
				if strings.EqualFold(namespace, "System") {
					alignment.SystemDefaultNamespaceMembers += len(decl.Constructors) + len(decl.Methods) + len(decl.Properties)
				}
				for _, ctor := range decl.Constructors {
					name := ctor.Name
					if name == "" {
						name = className
					}
					addToolingSymbol(symbols, namespace, className, name, "constructor")
				}
				for _, method := range decl.Methods {
					addToolingSymbol(symbols, namespace, className, cleanToolingMemberName(method.Name), "method")
					addToolingSignatureSymbol(symbols, namespace, className, method)
				}
				for _, property := range decl.Properties {
					addToolingSymbol(symbols, namespace, className, cleanToolingMemberName(property.Name), "property")
				}
			}
		}
	}
	if apexClassSymbols != nil {
		for _, record := range apexClassSymbols.Records {
			flattenToolingSymbolTable(symbols, &alignment, record.NamespacePrefix, record.Name, record.SymbolTable)
		}
	}
	return symbols, alignment
}

func flattenToolingSymbolTable(symbols map[string]SalesforceCoverageEntry, alignment *SalesforceToolingAlignment, namespace, className string, symbolTable *ToolingSymbolTable) {
	if symbolTable == nil {
		addToolingSymbol(symbols, namespace, className, "", "class")
		alignment.SymbolTableClasses++
		return
	}
	name := className
	if name == "" {
		name = symbolTable.Name
	}
	if name == "" {
		name = symbolTable.TableDeclaration.Name
	}
	ns := namespace
	if ns == "" && symbolTable.Namespace != nil {
		ns = *symbolTable.Namespace
	}
	addToolingSymbol(symbols, ns, name, "", "class")
	alignment.SymbolTableClasses++
	alignment.SymbolTableConstructors += len(symbolTable.Constructors)
	alignment.SymbolTableMethods += len(symbolTable.Methods)
	alignment.SymbolTableProperties += len(symbolTable.Properties)
	alignment.SymbolTableMembers += len(symbolTable.Constructors) + len(symbolTable.Methods) + len(symbolTable.Properties)
	for _, ctor := range symbolTable.Constructors {
		memberName := ctor.Name
		if memberName == "" {
			memberName = name
		}
		addToolingSymbol(symbols, ns, name, cleanToolingMemberName(memberName), "constructor")
	}
	for _, method := range symbolTable.Methods {
		addToolingSymbol(symbols, ns, name, cleanToolingMemberName(method.Name), "method")
	}
	for _, property := range symbolTable.Properties {
		addToolingSymbol(symbols, ns, name, cleanToolingMemberName(property.Name), "property")
	}
	for _, inner := range symbolTable.InnerClasses {
		flattenToolingSymbolTable(symbols, alignment, ns, name+"."+innerToolingClassName(inner), &inner)
	}
}

func innerToolingClassName(symbolTable ToolingSymbolTable) string {
	if symbolTable.Name != "" {
		return symbolTable.Name
	}
	if symbolTable.TableDeclaration.Name != "" {
		return symbolTable.TableDeclaration.Name
	}
	return "Inner"
}

func addToolingSymbol(symbols map[string]SalesforceCoverageEntry, namespace, className, memberName, kind string) {
	if className == "" {
		return
	}
	symbol := toolingSymbol(namespace, className, memberName)
	symbols[strings.ToLower(symbol)] = SalesforceCoverageEntry{Symbol: symbol, Kind: kind, Owner: toolingOwner(namespace)}
	if strings.EqualFold(namespace, "System") {
		qualified := "System." + symbol
		symbols[strings.ToLower(qualified)] = SalesforceCoverageEntry{Symbol: qualified, Kind: kind, Owner: toolingOwner(namespace)}
	}
}

func addToolingSignatureSymbol(symbols map[string]SalesforceCoverageEntry, namespace, className string, method ToolingMethod) {
	methodName := cleanToolingMemberName(method.Name)
	if methodName == "" {
		return
	}
	args := method.ArgTypes
	if len(args) == 0 && len(method.Parameters) > 0 {
		args = make([]string, 0, len(method.Parameters))
		for _, param := range method.Parameters {
			args = append(args, param.Type)
		}
	}
	symbol := toolingSymbol(namespace, className, methodName) + "(" + strings.Join(args, ",") + ")"
	symbols[strings.ToLower(symbol)] = SalesforceCoverageEntry{Symbol: symbol, Kind: "method", Owner: toolingOwner(namespace)}
	if strings.EqualFold(namespace, "System") {
		qualified := "System." + symbol
		symbols[strings.ToLower(qualified)] = SalesforceCoverageEntry{Symbol: qualified, Kind: "method", Owner: toolingOwner(namespace)}
	}
}

func cleanToolingMemberName(name string) string {
	if strings.Contains(name, "||") {
		name = strings.TrimSpace(strings.Split(name, "||")[0])
	}
	return strings.TrimSpace(name)
}

func toolingSymbol(namespace, className, memberName string) string {
	return catalogSymbol(namespace, className, memberName)
}

func toolingOwner(namespace string) string {
	if strings.EqualFold(namespace, "System") {
		return "Tooling API System namespace"
	}
	return "Tooling API " + namespace + " namespace"
}

func isConcreteRuntimeAPI(api string) bool {
	if api == "" || strings.ContainsAny(api, "* /") {
		return false
	}
	return strings.Count(api, ".") >= 1
}

func incrementSalesforceCoverageStatus(supported, partial, stub, unsupported, unknown *int, status Status) {
	switch status {
	case StatusSupported:
		*supported = *supported + 1
	case StatusPartial:
		*partial = *partial + 1
	case StatusStub:
		*stub = *stub + 1
	case StatusUnsupported:
		*unsupported = *unsupported + 1
	default:
		*unknown = *unknown + 1
	}
}

func WriteSalesforceCoverageJSON(w io.Writer, report SalesforceCoverageReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteSalesforceCoverageMarkdown(w io.Writer, report SalesforceCoverageReport) error {
	if _, err := fmt.Fprintln(w, "# Salesforce Coverage Manifest"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Source documents: %d\n", report.SourceDocuments); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Source members: %d\n", report.SourceMembers); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Coverage entries: %d\n", report.Entries); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Known supported entries: %d\n", report.Totals.Supported); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Unknown entries: %d\n", report.Totals.Unknown); err != nil {
		return err
	}
	if report.Tooling != nil {
		if _, err := fmt.Fprintf(w, "- Tooling API classes: %d\n", report.Tooling.Classes); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Tooling API members: %d\n", report.Tooling.Members); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Runtime APIs found in Tooling API: %d/%d\n", report.Tooling.ConcreteRuntimeAPIsInTooling, report.Tooling.ConcreteRuntimeAPIs); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Area | Target | Entries | Supported | Partial | Stub | Unsupported | Unknown |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |"); err != nil {
		return err
	}
	for _, area := range report.Areas {
		if _, err := fmt.Fprintf(w, "| %s | `%s` | %d | %d | %d | %d | %d | %d |\n",
			area.Area, area.Target, area.Entries, area.Supported, area.Partial, area.Stub, area.Unsupported, area.Unknown); err != nil {
			return err
		}
	}
	if report.Tooling != nil {
		if _, err := fmt.Fprintln(w, "\n## Tooling API System Alignment"); err != nil {
			return err
		}
		if report.Tooling.Source != "" {
			if _, err := fmt.Fprintf(w, "\nSource: `%s`\n", report.Tooling.Source); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "\n- Namespaces: %d\n", report.Tooling.Namespaces); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Classes: %d\n", report.Tooling.Classes); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Constructors: %d\n", report.Tooling.Constructors); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Methods: %d\n", report.Tooling.Methods); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Properties: %d\n", report.Tooling.Properties); err != nil {
			return err
		}
		if report.Tooling.SymbolTableClasses > 0 {
			if _, err := fmt.Fprintf(w, "- ApexClass SymbolTable classes: %d\n", report.Tooling.SymbolTableClasses); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(w, "- ApexClass SymbolTable members: %d\n", report.Tooling.SymbolTableMembers); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "- System-default namespace classes: %d\n", report.Tooling.SystemDefaultNamespaceClasses); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- System-default namespace members: %d\n", report.Tooling.SystemDefaultNamespaceMembers); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Concrete runtime APIs in Tooling API: %d/%d\n", report.Tooling.ConcreteRuntimeAPIsInTooling, report.Tooling.ConcreteRuntimeAPIs); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Catalog system entries in Tooling API: %d/%d\n", report.Tooling.CatalogSystemEntriesInTooling, report.Tooling.CatalogSystemEntries); err != nil {
			return err
		}
		if len(report.Tooling.MissingRuntimeAPIs) > 0 {
			if _, err := fmt.Fprintln(w, "\n### Runtime APIs Not Found In Tooling API"); err != nil {
				return err
			}
			for _, entry := range report.Tooling.MissingRuntimeAPIs {
				if _, err := fmt.Fprintf(w, "- `%s`\n", entry.Symbol); err != nil {
					return err
				}
			}
		}
		if len(report.Tooling.MissingCatalogSystemEntries) > 0 {
			if _, err := fmt.Fprintln(w, "\n### Catalog System Entries Not Found In Tooling API"); err != nil {
				return err
			}
			for _, entry := range report.Tooling.MissingCatalogSystemEntries {
				if _, err := fmt.Fprintf(w, "- `%s`\n", entry.Symbol); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func WriteSalesforceCoverageText(w io.Writer, report SalesforceCoverageReport) error {
	fmt.Fprintf(w, "schemaVersion: %d\n", report.SchemaVersion)
	fmt.Fprintf(w, "sourceDocuments: %d\n", report.SourceDocuments)
	fmt.Fprintf(w, "sourceMembers: %d\n", report.SourceMembers)
	fmt.Fprintf(w, "entries: %d\n", report.Entries)
	fmt.Fprintf(w, "supported: %d\n", report.Totals.Supported)
	fmt.Fprintf(w, "partial: %d\n", report.Totals.Partial)
	fmt.Fprintf(w, "stub: %d\n", report.Totals.Stub)
	fmt.Fprintf(w, "unsupported: %d\n", report.Totals.Unsupported)
	fmt.Fprintf(w, "unknown: %d\n", report.Totals.Unknown)
	if report.Tooling != nil {
		fmt.Fprintf(w, "toolingClasses: %d\n", report.Tooling.Classes)
		fmt.Fprintf(w, "toolingMembers: %d\n", report.Tooling.Members)
		if report.Tooling.SymbolTableClasses > 0 {
			fmt.Fprintf(w, "toolingSymbolTableClasses: %d\n", report.Tooling.SymbolTableClasses)
			fmt.Fprintf(w, "toolingSymbolTableMembers: %d\n", report.Tooling.SymbolTableMembers)
		}
		fmt.Fprintf(w, "toolingRuntimeMatched: %d/%d\n", report.Tooling.ConcreteRuntimeAPIsInTooling, report.Tooling.ConcreteRuntimeAPIs)
		fmt.Fprintf(w, "toolingCatalogSystemMatched: %d/%d\n", report.Tooling.CatalogSystemEntriesInTooling, report.Tooling.CatalogSystemEntries)
	}
	if len(report.Areas) == 0 {
		return nil
	}
	fmt.Fprintln(w, "area summary:")
	for _, area := range report.Areas {
		fmt.Fprintf(w, "  %s [%s]: entries=%d supported=%d partial=%d stub=%d unsupported=%d unknown=%d\n",
			area.Area, area.Target, area.Entries, area.Supported, area.Partial, area.Stub, area.Unsupported, area.Unknown)
	}
	return nil
}
