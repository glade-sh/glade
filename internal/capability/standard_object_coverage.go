package capability

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/glade-sh/glade/internal/storage"
)

const StandardObjectCoverageSchemaVersion = 2

const (
	StandardObjectCoverageShape    = "shape"
	StandardObjectCoverageBehavior = "behavior"
)

var standardObjectBehaviorCoverage = map[string]bool{
	"Account":                     true,
	"Attachment":                  true,
	"CampaignMember":              true,
	"CampaignMemberStatus":        true,
	"Contact":                     true,
	"ContentDistribution":         true,
	"ContentDocument":             true,
	"ContentDocumentLink":         true,
	"ContentVersion":              true,
	"Document":                    true,
	"EmailMessage":                true,
	"EmailMessageRelation":        true,
	"FieldPermissions":            true,
	"Lead":                        true,
	"ObjectPermissions":           true,
	"Opportunity":                 true,
	"OpportunityLineItem":         true,
	"PermissionSet":               true,
	"PermissionSetAssignment":     true,
	"PermissionSetGroup":          true,
	"PermissionSetGroupComponent": true,
	"Pricebook2":                  true,
	"PricebookEntry":              true,
	"Product2":                    true,
	"Profile":                     true,
	"RecordType":                  true,
	"SetupEntityAccess":           true,
	"User":                        true,
}

type StandardObjectCoverageReport struct {
	SchemaVersion int                           `json:"schemaVersion"`
	Objects       []StandardObjectCoverageEntry `json:"objects"`
	Totals        StandardObjectCoverageTotals  `json:"totals"`
}

type StandardObjectCoverageTotals struct {
	Objects         int `json:"objects"`
	ShapeObjects    int `json:"shapeObjects"`
	BehaviorObjects int `json:"behaviorObjects"`
	KeyPrefixes     int `json:"keyPrefixes"`
	Fields          int `json:"fields"`
	Relationships   int `json:"relationships"`
	RecordTypes     int `json:"recordTypes"`
	Picklists       int `json:"picklists"`
	References      int `json:"references"`
}

type StandardObjectCoverageEntry struct {
	Object        string `json:"object"`
	Label         string `json:"label,omitempty"`
	PluralLabel   string `json:"pluralLabel,omitempty"`
	KeyPrefix     string `json:"keyPrefix,omitempty"`
	Coverage      string `json:"coverage"`
	Fields        int    `json:"fields"`
	Relationships int    `json:"relationships"`
	RecordTypes   int    `json:"recordTypes"`
	Picklists     int    `json:"picklists"`
	References    int    `json:"references"`
}

func BuildStandardObjectCoverageReport() StandardObjectCoverageReport {
	names := storage.KnownStandardObjectNames()
	report := StandardObjectCoverageReport{
		SchemaVersion: StandardObjectCoverageSchemaVersion,
		Objects:       make([]StandardObjectCoverageEntry, 0, len(names)),
	}
	for _, name := range names {
		definition, ok := storage.StandardObjectDefinition(name)
		if !ok {
			continue
		}
		entry := StandardObjectCoverageEntry{
			Object:        definition.APIName,
			Label:         definition.Label,
			PluralLabel:   definition.PluralLabel,
			KeyPrefix:     definition.KeyPrefix,
			Coverage:      standardObjectCoverageLevel(definition.APIName),
			Fields:        len(definition.Fields),
			Relationships: len(definition.Relations),
			RecordTypes:   len(definition.RecordTypes),
		}
		for _, field := range definition.Fields {
			if len(field.PicklistValues) > 0 {
				entry.Picklists++
			}
			if len(field.ReferenceTo) > 0 {
				entry.References++
			}
		}
		report.Objects = append(report.Objects, entry)
		report.Totals.Objects++
		report.Totals.ShapeObjects++
		if entry.Coverage == StandardObjectCoverageBehavior {
			report.Totals.BehaviorObjects++
		}
		if entry.KeyPrefix != "" {
			report.Totals.KeyPrefixes++
		}
		report.Totals.Fields += entry.Fields
		report.Totals.Relationships += entry.Relationships
		report.Totals.RecordTypes += entry.RecordTypes
		report.Totals.Picklists += entry.Picklists
		report.Totals.References += entry.References
	}
	sort.Slice(report.Objects, func(i, j int) bool {
		return report.Objects[i].Object < report.Objects[j].Object
	})
	return report
}

func standardObjectCoverageLevel(objectName string) string {
	if standardObjectBehaviorCoverage[objectName] {
		return StandardObjectCoverageBehavior
	}
	return StandardObjectCoverageShape
}

func WriteStandardObjectCoverageJSON(w io.Writer, report StandardObjectCoverageReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteStandardObjectCoverageMarkdown(w io.Writer, report StandardObjectCoverageReport) error {
	if _, err := fmt.Fprintln(w, "# Standard Object Coverage"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\nGenerated from `internal/storage` standard object metadata."); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\n- Objects: %d\n", report.Totals.Objects); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Shape objects: %d\n", report.Totals.ShapeObjects); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Behavior objects: %d\n", report.Totals.BehaviorObjects); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Key prefixes: %d\n", report.Totals.KeyPrefixes); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Fields: %d\n", report.Totals.Fields); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Relationships: %d\n", report.Totals.Relationships); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Record types: %d\n", report.Totals.RecordTypes); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Picklist fields: %d\n", report.Totals.Picklists); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Reference fields: %d\n", report.Totals.References); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\n| Object | Coverage | Key Prefix | Fields | Relationships | Record Types | Picklists | References |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | --- | ---: | ---: | ---: | ---: | ---: |"); err != nil {
		return err
	}
	for _, entry := range report.Objects {
		if _, err := fmt.Fprintf(w, "| `%s` | `%s` | `%s` | %d | %d | %d | %d | %d |\n",
			entry.Object, entry.Coverage, entry.KeyPrefix, entry.Fields, entry.Relationships, entry.RecordTypes, entry.Picklists, entry.References); err != nil {
			return err
		}
	}
	return nil
}
