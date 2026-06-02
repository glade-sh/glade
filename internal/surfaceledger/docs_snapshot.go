package surfaceledger

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/apexdocs"
)

var apiVersionPattern = regexp.MustCompile(`(?i)Available in API version\s+([0-9]+(?:\.[0-9]+)?)`)

func BuildDocsSnapshot(source string) ([]SurfaceLedgerRow, error) {
	inv, err := apexdocs.BuildInventory(source)
	if err != nil {
		return nil, err
	}
	rows := RowsFromDocsInventory(inv)
	for i := range rows {
		if rows[i].DocsSource == "" {
			continue
		}
		rows[i].APIVersion = readAPIVersion(filepath.Join(source, filepath.FromSlash(rows[i].DocsSource)))
	}
	return rows, nil
}

func RowsFromDocsInventory(inv apexdocs.Inventory) []SurfaceLedgerRow {
	var rows []SurfaceLedgerRow
	for _, doc := range inv.Documents {
		product := ProductFromSourcePath(doc.SourcePath)
		row := RowFromDocs(SurfaceLedgerRow{
			SurfaceID:  docsSurfaceID(product, doc, apexdocs.Member{}),
			Product:    product,
			Area:       areaForProduct(product),
			Namespace:  docsNamespace(product, doc),
			TypeName:   doc.Name,
			Kind:       docsKind(product, doc.Kind),
			DocsSource: doc.SourcePath,
			DocsTitle:  doc.Title,
			Sources:    []string{"docs"},
		})
		rows = append(rows, row)
		for _, member := range doc.Members {
			rows = append(rows, RowFromDocs(SurfaceLedgerRow{
				SurfaceID:  docsSurfaceID(product, doc, member),
				Product:    product,
				Area:       areaForProduct(product),
				Namespace:  docsNamespace(product, doc),
				TypeName:   doc.Name,
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

func docsNamespace(product string, doc apexdocs.Document) string {
	if doc.Namespace != "" {
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

func docsKind(product, kind string) string {
	switch product {
	case ProductREST:
		return KindResource
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

func areaForProduct(product string) string {
	switch product {
	case ProductREST, ProductTooling:
		return AreaServer
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
	parts := strings.Split(inside, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) == 0 {
			continue
		}
		out = append(out, fields[0])
	}
	return out
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
