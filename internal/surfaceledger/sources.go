package surfaceledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type SourceFamily struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Kind        string `json:"kind"`
	Deliverable string `json:"deliverable,omitempty"`
	Required    bool   `json:"required"`
}

type SourceFamilyStatus struct {
	SourceFamily
	LocalStatus string `json:"localStatus"`
	FailedPages int    `json:"failedPages"`
	EmptyPages  int    `json:"emptyPages"`
	TotalPages  int    `json:"totalPages"`
}

type SourceAuditReport struct {
	DocsRoot             string               `json:"docsRoot"`
	AtlasPinned          int                  `json:"atlasPinned"`
	AtlasMissing         int                  `json:"atlasMissing"`
	AtlasPartial         int                  `json:"atlasPartial"`
	MissingRequired      int                  `json:"missingRequired"`
	PartialRequired      int                  `json:"partialRequired"`
	ManifestPresent      bool                 `json:"manifestPresent"`
	SearchIndexPresent   bool                 `json:"searchIndexPresent"`
	MissingLocalMarkdown int                  `json:"missingLocalMarkdown"`
	UnmanifestedMarkdown int                  `json:"unmanifestedMarkdown"`
	LWCStatus            string               `json:"lwcStatus"`
	SiteReferencesStatus string               `json:"siteReferencesStatus"`
	Families             []SourceFamilyStatus `json:"families"`
}

type docsetVersion struct {
	Pages struct {
		Failed int `json:"failed"`
		Empty  int `json:"empty"`
		Total  int `json:"total"`
	} `json:"pages"`
}

type manifestEntry struct {
	Path string `json:"path"`
}

func SourceUniverse() []SourceFamily {
	return []SourceFamily{
		{Name: "apex", Title: "Apex Reference Guide", Kind: "atlas", Deliverable: "apexref", Required: true},
		{Name: "apex-guide", Title: "Apex Developer Guide", Kind: "atlas", Deliverable: "apexcode", Required: true},
		{Name: "visualforce", Title: "Visualforce Component Reference", Kind: "atlas", Deliverable: "pages", Required: true},
		{Name: "lightning", Title: "Lightning Aura Components Reference", Kind: "atlas", Deliverable: "lightning", Required: true},
		{Name: "rest-api", Title: "REST API Developer Guide", Kind: "atlas", Deliverable: "api_rest", Required: true},
		{Name: "tooling-api", Title: "Tooling API Developer Guide", Kind: "atlas", Deliverable: "api_tooling", Required: true},
		{Name: "object-reference", Title: "Object Reference for the Salesforce Platform", Kind: "atlas", Deliverable: "object_reference", Required: true},
		{Name: "field-reference", Title: "Salesforce Field Reference Guide", Kind: "atlas", Deliverable: "sfFieldRef", Required: true},
		{Name: "soql-sosl", Title: "SOQL and SOSL Reference", Kind: "atlas", Deliverable: "soql_sosl", Required: true},
		{Name: "metadata-api", Title: "Metadata API Developer Guide", Kind: "atlas", Deliverable: "api_meta", Required: true},
		{Name: "soap-api", Title: "SOAP API Developer Guide", Kind: "atlas", Deliverable: "api", Required: true},
		{Name: "bulk-api", Title: "Bulk API Developer Guide", Kind: "atlas", Deliverable: "api_asynch", Required: true},
		{Name: "ui-api", Title: "User Interface API Developer Guide", Kind: "atlas", Deliverable: "uiapi", Required: true},
		{Name: "platform-events", Title: "Platform Events Developer Guide", Kind: "atlas", Deliverable: "platform_events", Required: true},
		{Name: "streaming-api", Title: "Streaming API Developer Guide", Kind: "atlas", Deliverable: "api_streaming", Required: true},
		{Name: "connect-rest-api", Title: "Connect REST API Developer Guide", Kind: "atlas", Deliverable: "chatterapi", Required: true},
		{Name: "service-connector-api-reference", Title: "Service Cloud Connector API Reference", Kind: "atlas", Deliverable: "service_connector_api_developer_guide", Required: true},
		{Name: "limits-reference", Title: "Salesforce Developer Limits and Allocations Quick Reference", Kind: "atlas", Deliverable: "salesforce_app_limits_cheatsheet", Required: true},
		{Name: "cli-reference", Title: "Salesforce CLI Command Reference", Kind: "atlas", Deliverable: "sfdx_cli_reference", Required: true},
		{Name: "analytics-cli-reference", Title: "Salesforce Analytics Plugin CLI Command Reference", Kind: "atlas", Deliverable: "bi_dev_guide_cli_reference", Required: true},
		{Name: "commerce-cli-reference", Title: "Salesforce Commerce Plug-In CLI Command Reference", Kind: "atlas", Deliverable: "comm_cli_reference", Required: true},
		{Name: "lwc", Title: "Lightning Web Components Reference", Kind: "non-atlas", Required: true},
		{Name: "site-references", Title: "Developer Docs Site References", Kind: "site-references", Required: true},
		{Name: "pub-sub-api", Title: "Pub/Sub API", Kind: "covered-through-site-references", Required: true},
		{Name: "graphql-api", Title: "GraphQL API", Kind: "covered-through-site-references", Required: true},
		{Name: "agentforce", Title: "Agentforce", Kind: "covered-through-site-references", Required: true},
		{Name: "marketing-cloud-ampscript", Title: "Marketing Cloud AMPscript", Kind: "covered-through-site-references", Required: true},
		{Name: "sf-connect-amazon-rds", Title: "Salesforce Connect Amazon RDS", Kind: "covered-through-site-references", Required: true},
	}
}

func AuditSourceUniverse(docsRoot string) (SourceAuditReport, error) {
	report := SourceAuditReport{DocsRoot: docsRoot, LWCStatus: "missing", SiteReferencesStatus: "missing"}
	if docsRoot == "" {
		return report, fmt.Errorf("--docs is required")
	}
	report.ManifestPresent = fileExists(filepath.Join(docsRoot, "manifest.json"))
	report.SearchIndexPresent = fileExists(filepath.Join(docsRoot, "search-index.json"))
	report.MissingLocalMarkdown, report.UnmanifestedMarkdown = markdownManifestDiff(docsRoot)
	for _, family := range SourceUniverse() {
		status := auditSourceFamily(docsRoot, family)
		report.Families = append(report.Families, status)
		if family.Kind == "atlas" {
			report.AtlasPinned++
			if status.LocalStatus == "missing" {
				report.AtlasMissing++
			}
			if status.LocalStatus == "partial" {
				report.AtlasPartial++
			}
		}
		if family.Name == "lwc" {
			report.LWCStatus = status.LocalStatus
		}
		if family.Name == "site-references" {
			report.SiteReferencesStatus = status.LocalStatus
		}
		if family.Required && status.LocalStatus == "missing" {
			report.MissingRequired++
		}
		if family.Required && status.LocalStatus == "partial" {
			report.PartialRequired++
		}
	}
	return report, nil
}

func auditSourceFamily(root string, family SourceFamily) SourceFamilyStatus {
	status := SourceFamilyStatus{SourceFamily: family, LocalStatus: "missing"}
	switch family.Kind {
	case "covered-through-site-references":
		if hasSiteReferenceProject(root, family.Name) {
			status.LocalStatus = "present"
		}
		return status
	}
	dir := filepath.Join(root, family.Name)
	if !dirExists(dir) {
		return status
	}
	status.LocalStatus = "present"
	version, ok := readDocsetVersion(filepath.Join(dir, "_version.json"))
	if ok {
		status.FailedPages = version.Pages.Failed
		status.EmptyPages = version.Pages.Empty
		status.TotalPages = version.Pages.Total
		if version.Pages.Failed > 0 || version.Pages.Empty > 0 {
			status.LocalStatus = "partial"
		}
	}
	if family.Name == "site-references" && !fileExists(filepath.Join(dir, "_catalog.md")) {
		status.LocalStatus = "partial"
	}
	return status
}

func SourceAuditComplete(report SourceAuditReport) bool {
	return report.MissingRequired == 0 &&
		report.PartialRequired == 0 &&
		report.ManifestPresent &&
		report.SearchIndexPresent &&
		report.MissingLocalMarkdown == 0 &&
		report.UnmanifestedMarkdown == 0
}

func SourceAuditMarkdown(report SourceAuditReport) string {
	var b strings.Builder
	fmt.Fprintln(&b, "# Salesforce Surface Sources")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Docs root: `%s`\n", report.DocsRoot)
	fmt.Fprintf(&b, "- Atlas docsets pinned: %d\n", report.AtlasPinned)
	fmt.Fprintf(&b, "- Atlas docsets missing: %d\n", report.AtlasMissing)
	fmt.Fprintf(&b, "- Atlas docsets partial: %d\n", report.AtlasPartial)
	fmt.Fprintf(&b, "- Missing required source families: %d\n", report.MissingRequired)
	fmt.Fprintf(&b, "- Partial required source families: %d\n", report.PartialRequired)
	fmt.Fprintf(&b, "- Manifest present: %t\n", report.ManifestPresent)
	fmt.Fprintf(&b, "- Search index present: %t\n", report.SearchIndexPresent)
	fmt.Fprintf(&b, "- Missing local markdown: %d\n", report.MissingLocalMarkdown)
	fmt.Fprintf(&b, "- Unmanifested markdown: %d\n", report.UnmanifestedMarkdown)
	fmt.Fprintf(&b, "- LWC: %s\n", report.LWCStatus)
	fmt.Fprintf(&b, "- Site references: %s\n", report.SiteReferencesStatus)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Source family | Kind | Status | Failed | Empty | Total |")
	fmt.Fprintln(&b, "| --- | --- | --- | ---: | ---: | ---: |")
	for _, family := range report.Families {
		fmt.Fprintf(&b, "| `%s` | %s | %s | %d | %d | %d |\n", family.Name, family.Kind, family.LocalStatus, family.FailedPages, family.EmptyPages, family.TotalPages)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Missing Source Family Rows")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Row | Source family | Action |")
	fmt.Fprintln(&b, "| --- | --- | --- |")
	for _, family := range report.Families {
		if family.LocalStatus != "missing" && family.LocalStatus != "partial" {
			continue
		}
		fmt.Fprintf(&b, "| `missing-source-family:%s` | `%s` | add scraper support, copy improved docs into the workspace docs source, mark the family out-of-current-runtime-scope, or add a small public-doc-backed fixture for narrow behavior |\n", family.Name, family.Name)
	}
	return b.String()
}

func hasSiteReferenceProject(root, project string) bool {
	if project == "site-references" {
		return dirExists(filepath.Join(root, "site-references"))
	}
	if project == "graphql-api" {
		project = "graphql"
	}
	matches, _ := filepath.Glob(filepath.Join(root, "site-references", "*", project))
	return len(matches) > 0
}

func readDocsetVersion(path string) (docsetVersion, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return docsetVersion{}, false
	}
	var version docsetVersion
	if err := json.Unmarshal(data, &version); err != nil {
		return docsetVersion{}, false
	}
	return version, true
}

func markdownManifestDiff(root string) (int, int) {
	manifestPath := filepath.Join(root, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return 1, 0
	}
	var entries []manifestEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return 1, 0
	}
	manifest := map[string]bool{}
	for _, entry := range entries {
		if entry.Path != "" {
			manifest[filepath.ToSlash(entry.Path)] = true
		}
	}
	files := map[string]bool{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if !strings.Contains(rel, string(filepath.Separator)) {
			return nil
		}
		files[filepath.ToSlash(rel)] = true
		return nil
	})
	missing := 0
	for path := range manifest {
		if !files[path] {
			missing++
		}
	}
	unmanifested := 0
	for path := range files {
		if !manifest[path] {
			unmanifested++
		}
	}
	return missing, unmanifested
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func SortedSourceFamilyNames() []string {
	families := SourceUniverse()
	names := make([]string, 0, len(families))
	for _, family := range families {
		names = append(names, family.Name)
	}
	sort.Strings(names)
	return names
}
