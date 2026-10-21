package vm

import (
	"encoding/xml"

	"github.com/glade-sh/glade/internal/storage"
)

type layoutMetadataXML struct {
	XMLName  xml.Name                   `xml:"Layout"`
	FullName string                     `xml:"fullName"`
	Sections []layoutSectionMetadataXML `xml:"layoutSections"`
}

type layoutSectionMetadataXML struct {
	CustomLabel   *bool                     `xml:"customLabel"`
	DetailHeading *bool                     `xml:"detailHeading"`
	EditHeading   *bool                     `xml:"editHeading"`
	Label         string                    `xml:"label"`
	Style         string                    `xml:"style"`
	Columns       []layoutColumnMetadataXML `xml:"layoutColumns"`
}

type layoutColumnMetadataXML struct {
	Items []layoutItemMetadataXML `xml:"layoutItems"`
}

type layoutItemMetadataXML struct {
	Field    string `xml:"field"`
	Behavior string `xml:"behavior"`
}

func declaredMetadataLayout(entry storage.LayoutMetadata) Value {
	var metadata layoutMetadataXML
	if err := xml.Unmarshal([]byte(entry.XML), &metadata); err != nil {
		return Null
	}
	layout := Object("Metadata.Layout")
	layout.Fields["fullName"] = String(entry.FullName)
	sections := typedList("List<Metadata.LayoutSection>")
	for _, declaration := range metadata.Sections {
		section := Object("Metadata.LayoutSection")
		section.Fields["label"] = Null
		if declaration.Label != "" {
			section.Fields["label"] = String(declaration.Label)
		}
		section.Fields["style"] = layoutMetadataEnum("Metadata.LayoutSectionStyle", declaration.Style)
		for _, flag := range []struct {
			name  string
			value *bool
		}{{"customLabel", declaration.CustomLabel}, {"detailHeading", declaration.DetailHeading}, {"editHeading", declaration.EditHeading}} {
			section.Fields[flag.name] = Null
			if flag.value != nil {
				section.Fields[flag.name] = Bool(*flag.value)
			}
		}
		columns := typedList("List<Metadata.LayoutColumn>")
		for _, declaration := range declaration.Columns {
			column := Object("Metadata.LayoutColumn")
			items := typedList("List<Metadata.LayoutItem>")
			for _, declaration := range declaration.Items {
				item := Object("Metadata.LayoutItem")
				item.Fields["field"] = Null
				if declaration.Field != "" {
					item.Fields["field"] = String(declaration.Field)
				}
				item.Fields["behavior"] = layoutMetadataEnum("Metadata.UiBehavior", declaration.Behavior)
				items.List = append(items.List, item)
			}
			column.Fields["layoutItems"] = items
			columns.List = append(columns.List, column)
		}
		section.Fields["layoutColumns"] = columns
		sections.List = append(sections.List, section)
	}
	layout.Fields["layoutSections"] = sections
	return layout
}

func layoutMetadataEnum(typeName, name string) Value {
	canonical, names, ok := metadataEnumSpec(typeName)
	if !ok || name == "" {
		return Null
	}
	value, _ := namedEnumStaticValue(canonical, names, canonical+"."+name)
	return value
}
