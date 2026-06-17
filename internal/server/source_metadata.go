package server

import (
	"encoding/xml"
	"fmt"
	"hash/crc32"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

type SourceMetadata struct {
	Project     project.Project
	ToolingOrg  storage.OrgState
	Components  []metadataComponent
	ListViews   map[string][]listViewMetadata
	Layouts     map[string][]layoutMetadata
	Compact     map[string][]compactLayoutMetadata
	componentBy map[string]metadataComponent
}

type metadataComponent struct {
	Type     string
	FullName string
	FileName string
	ID       storage.ID
	Content  string
}

type listViewMetadata struct {
	ID            string
	ObjectName    string
	DeveloperName string
	Label         string
	Columns       []string
	FilterScope   string
	FileName      string
}

type layoutMetadata struct {
	ID         string
	ObjectName string
	Name       string
	FileName   string
	Sections   []layoutSectionMetadata
}

type layoutSectionMetadata struct {
	ID         string
	Label      string
	Style      string
	UseHeading bool
	Columns    []layoutColumnMetadata
}

type layoutColumnMetadata struct {
	Items []layoutItemMetadata
}

type layoutItemMetadata struct {
	Field    string
	Behavior string
}

type compactLayoutMetadata struct {
	ID            string
	ObjectName    string
	DeveloperName string
	Label         string
	Fields        []string
	FileName      string
}

type listViewXML struct {
	Label       string   `xml:"label"`
	Columns     []string `xml:"columns"`
	FilterScope string   `xml:"filterScope"`
}

type compactLayoutXML struct {
	Label  string   `xml:"label"`
	Fields []string `xml:"fields"`
}

type layoutXML struct {
	Sections []layoutSectionXML `xml:"layoutSections"`
}

type layoutSectionXML struct {
	Label   string            `xml:"label"`
	Style   string            `xml:"style"`
	Columns []layoutColumnXML `xml:"layoutColumns"`
}

type layoutColumnXML struct {
	Items []layoutItemXML `xml:"layoutItems"`
}

type layoutItemXML struct {
	Field    string `xml:"field"`
	Behavior string `xml:"behavior"`
}

type customObjectXML struct {
	Label string `xml:"label"`
}

type customFieldXML struct {
	FullName string `xml:"fullName"`
	Label    string `xml:"label"`
	Type     string `xml:"type"`
}

type recordTypeXML struct {
	Label  string `xml:"label"`
	Active string `xml:"active"`
}

type validationRuleXML struct {
	Active       string `xml:"active"`
	ErrorMessage string `xml:"errorMessage"`
	Description  string `xml:"description"`
}

func NewSourceMetadataFromProject(p project.Project) (SourceMetadata, error) {
	meta := SourceMetadata{
		Project:     p,
		ToolingOrg:  storage.NewOrgState(),
		ListViews:   make(map[string][]listViewMetadata),
		Layouts:     make(map[string][]layoutMetadata),
		Compact:     make(map[string][]compactLayoutMetadata),
		componentBy: make(map[string]metadataComponent),
	}
	meta.ToolingOrg.APIVersion = p.SourceAPIVersion
	meta.ToolingOrg.Namespace = p.Namespace
	if meta.ToolingOrg.APIVersion == "" {
		meta.ToolingOrg.APIVersion = storage.DefaultRESTAPIVersion
	}
	if err := meta.loadToolingObjects(); err != nil {
		return SourceMetadata{}, err
	}
	if err := meta.loadObjectMetadata(); err != nil {
		return SourceMetadata{}, err
	}
	meta.sortAndIndexComponents()
	return meta, nil
}

func (m *SourceMetadata) hasData() bool {
	return len(m.Components) > 0 || len(m.ListViews) > 0 || len(m.Layouts) > 0 || len(m.Compact) > 0 || len(m.ToolingOrg.Objects) > 0
}

func (s *Server) handleToolingGladeCodeIntel(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if len(parts) != 1 {
		writeSalesforceError(w, errUnknownTooling)
		return
	}

	graph := codeintel.Graph{}
	if s.Index != nil {
		graph = codeintel.Build(*s.Index, codeintel.Options{})
	}

	switch parts[0] {
	case "symbols":
		records := graph.SortedSymbols()
		writeJSON(w, http.StatusOK, codeIntelQueryPayload(len(records), records))
	case "definition":
		id, ok := codeIntelSymbolID(r.URL.Query().Get("symbol"), graph)
		if !ok {
			writeJSON(w, http.StatusOK, codeIntelQueryPayload(0, []codeintel.Symbol{}))
			return
		}
		symbol, ok := graph.Definition(id)
		if !ok {
			writeJSON(w, http.StatusOK, codeIntelQueryPayload(0, []codeintel.Symbol{}))
			return
		}
		writeJSON(w, http.StatusOK, codeIntelQueryPayload(1, []codeintel.Symbol{symbol}))
	case "references":
		id, ok := codeIntelSymbolID(r.URL.Query().Get("symbol"), graph)
		if !ok {
			writeJSON(w, http.StatusOK, codeIntelQueryPayload(0, []codeintel.Use{}))
			return
		}
		records := graph.References(id, true)
		writeJSON(w, http.StatusOK, codeIntelQueryPayload(len(records), records))
	default:
		writeSalesforceError(w, errUnknownTooling)
	}
}

func codeIntelQueryPayload(totalSize int, records any) map[string]any {
	return map[string]any{
		"totalSize": totalSize,
		"done":      true,
		"records":   records,
	}
}

func codeIntelSymbolID(raw string, graph codeintel.Graph) (codeintel.SymbolID, bool) {
	symbolName := strings.TrimSpace(raw)
	if symbolName == "" {
		return "", false
	}
	if _, ok := graph.Definition(codeintel.SymbolID(symbolName)); ok {
		return codeintel.SymbolID(symbolName), true
	}
	if objectName, fieldName, ok := strings.Cut(symbolName, "."); ok && objectName != "" && fieldName != "" {
		id := codeintel.SObjectFieldID(objectName, fieldName)
		if _, ok := graph.Definition(id); ok {
			return id, true
		}
	}
	if id := codeintel.SObjectID(symbolName); id != "" {
		if _, ok := graph.Definition(id); ok {
			return id, true
		}
	}
	for _, symbol := range graph.SortedSymbols() {
		if symbol.Name == symbolName {
			return symbol.ID, true
		}
	}
	return "", false
}

func (m *SourceMetadata) loadToolingObjects() error {
	defs := map[string]storage.ObjectDefinition{
		"ApexClass":      toolingObjectDefinition("ApexClass", "Apex Class", "01p", []string{"Name", "Body", "BodyCrc", "ApiVersion", "Status", "IsValid", "LengthWithoutComments", "NamespacePrefix"}),
		"ApexTrigger":    toolingObjectDefinition("ApexTrigger", "Apex Trigger", "01q", []string{"Name", "Body", "BodyCrc", "ApiVersion", "Status", "IsValid", "LengthWithoutComments", "TableEnumOrId"}),
		"ApexPage":       toolingObjectDefinition("ApexPage", "Apex Page", "066", []string{"Name", "Markup", "ApiVersion", "MasterLabel"}),
		"ApexComponent":  toolingObjectDefinition("ApexComponent", "Apex Component", "099", []string{"Name", "Markup", "ApiVersion", "MasterLabel"}),
		"StaticResource": toolingObjectDefinition("StaticResource", "Static Resource", "081", []string{"Name", "Body", "ContentType", "CacheControl", "Description"}),
		"CustomObject":   toolingObjectDefinition("CustomObject", "Custom Object", "01I", []string{"DeveloperName", "FullName", "MasterLabel", "NamespacePrefix", "ManageableState"}),
		"CustomField":    toolingObjectDefinition("CustomField", "Custom Field", "00N", []string{"DeveloperName", "FullName", "TableEnumOrId", "NamespacePrefix", "ManageableState", "MetadataType"}),
		"Layout":         toolingObjectDefinition("Layout", "Layout", "00h", []string{"Name", "FullName", "TableEnumOrId", "ManageableState"}),
		"CompactLayout":  toolingObjectDefinition("CompactLayout", "Compact Layout", "0CL", []string{"DeveloperName", "FullName", "MasterLabel", "SobjectType", "NamespacePrefix", "ManageableState"}),
		"RecordType":     toolingObjectDefinition("RecordType", "Record Type", "012", []string{"DeveloperName", "FullName", "Name", "SobjectType", "Active", "IsActive"}),
		"ValidationRule": toolingObjectDefinition("ValidationRule", "Validation Rule", "03d", []string{"ValidationName", "FullName", "EntityDefinitionId", "Active", "ErrorMessage", "Description"}),
	}
	for name, def := range defs {
		m.ToolingOrg.Objects[name] = storage.ObjectState{Definition: def, Records: make(map[storage.ID]storage.Record)}
	}
	if err := m.addSourceComponents("ApexClass", "01p", m.Project.ApexFiles, ".cls"); err != nil {
		return err
	}
	if err := m.addSourceComponents("ApexTrigger", "01q", m.Project.ApexFiles, ".trigger"); err != nil {
		return err
	}
	if err := m.addSourceComponents("ApexPage", "066", m.Project.VisualforcePageFiles, ".page"); err != nil {
		return err
	}
	if err := m.addSourceComponents("ApexComponent", "099", m.Project.VisualforceComponentFiles, ".component"); err != nil {
		return err
	}
	for i, path := range m.Project.StaticResourceFiles {
		name := trimKnownSuffix(filepath.Base(path), ".resource")
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		id := sequenceID("081", i+1)
		record := storage.Record{ID: id, Object: "StaticResource", Fields: map[string]storage.Value{
			"Name": storage.StringValue(name),
			"Body": storage.StringValue(string(content)),
		}}
		m.ToolingOrg.Objects["StaticResource"].Records[id] = record
		m.Components = append(m.Components, metadataComponent{Type: "StaticResource", FullName: name, FileName: path, ID: id, Content: string(content)})
	}
	return nil
}

func (m *SourceMetadata) addSourceComponents(objectName, prefix string, paths []string, suffix string) error {
	filtered := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.EqualFold(filepath.Ext(path), suffix) || strings.HasSuffix(strings.ToLower(path), strings.ToLower(suffix)) {
			filtered = append(filtered, path)
		}
	}
	sort.Strings(filtered)
	for i, path := range filtered {
		bodyBytes, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		body := string(bodyBytes)
		name := trimKnownSuffix(filepath.Base(path), suffix)
		id := sequenceID(prefix, i+1)
		fields := map[string]storage.Value{
			"Name":                  storage.StringValue(name),
			"Body":                  storage.StringValue(body),
			"BodyCrc":               storage.IntegerValue(int64(crc32.ChecksumIEEE(bodyBytes))),
			"ApiVersion":            storage.DecimalValue(sourceAPIVersion(m.Project.SourceAPIVersion)),
			"Status":                storage.StringValue("Active"),
			"IsValid":               storage.BooleanValue(true),
			"LengthWithoutComments": storage.IntegerValue(int64(len(body))),
		}
		if objectName == "ApexTrigger" {
			fields["TableEnumOrId"] = storage.StringValue(triggerObjectName(body))
		} else if objectName == "ApexClass" && m.Project.Namespace != "" {
			fields["NamespacePrefix"] = storage.StringValue(m.Project.Namespace)
		} else if objectName == "ApexPage" || objectName == "ApexComponent" {
			fields = map[string]storage.Value{
				"Name":        storage.StringValue(name),
				"Markup":      storage.StringValue(body),
				"ApiVersion":  storage.DecimalValue(sourceAPIVersion(m.Project.SourceAPIVersion)),
				"MasterLabel": storage.StringValue(name),
			}
		}
		record := storage.Record{ID: id, Object: objectName, Fields: fields}
		m.ToolingOrg.Objects[objectName].Records[id] = record
		m.Components = append(m.Components, metadataComponent{Type: objectName, FullName: name, FileName: path, ID: id, Content: body})
	}
	return nil
}

func (m *SourceMetadata) loadObjectMetadata() error {
	for i, path := range m.Project.ListViewFiles {
		view, err := loadListView(path, i+1)
		if err != nil {
			return err
		}
		m.ListViews[view.ObjectName] = append(m.ListViews[view.ObjectName], view)
		m.Components = append(m.Components, metadataComponent{Type: "ListView", FullName: view.ObjectName + "." + view.DeveloperName, FileName: path, ID: storage.ID(view.ID)})
	}
	for i, path := range m.Project.LayoutFiles {
		layout, err := loadLayout(path, i+1)
		if err != nil {
			return err
		}
		m.Layouts[layout.ObjectName] = append(m.Layouts[layout.ObjectName], layout)
		m.Components = append(m.Components, metadataComponent{Type: "Layout", FullName: layout.Name, FileName: path, ID: storage.ID(layout.ID)})
	}
	for i, path := range m.Project.CompactLayoutFiles {
		compact, err := loadCompactLayout(path, i+1)
		if err != nil {
			return err
		}
		m.Compact[compact.ObjectName] = append(m.Compact[compact.ObjectName], compact)
		m.Components = append(m.Components, metadataComponent{Type: "CompactLayout", FullName: compact.ObjectName + "." + compact.DeveloperName, FileName: path, ID: storage.ID(compact.ID)})
	}
	for _, component := range schemaBackedComponents(m.Project) {
		m.Components = append(m.Components, component)
	}
	if err := m.addToolingSchemaMetadataRecords(); err != nil {
		return err
	}
	return nil
}

func loadListView(path string, ordinal int) (listViewMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return listViewMetadata{}, err
	}
	var raw listViewXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return listViewMetadata{}, err
	}
	objectName := objectNameFromNestedMetadata(path, "listViews")
	name := trimKnownSuffix(filepath.Base(path), ".listView-meta.xml")
	label := strings.TrimSpace(raw.Label)
	if label == "" {
		label = name
	}
	return listViewMetadata{
		ID:            string(sequenceID("00B", ordinal)),
		ObjectName:    objectName,
		DeveloperName: name,
		Label:         label,
		Columns:       trimStringList(raw.Columns),
		FilterScope:   strings.TrimSpace(raw.FilterScope),
		FileName:      path,
	}, nil
}

func loadCompactLayout(path string, ordinal int) (compactLayoutMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return compactLayoutMetadata{}, err
	}
	var raw compactLayoutXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return compactLayoutMetadata{}, err
	}
	objectName := objectNameFromNestedMetadata(path, "compactLayouts")
	name := trimKnownSuffix(filepath.Base(path), ".compactLayout-meta.xml")
	label := strings.TrimSpace(raw.Label)
	if label == "" {
		label = name
	}
	return compactLayoutMetadata{
		ID:            string(sequenceID("0CL", ordinal)),
		ObjectName:    objectName,
		DeveloperName: name,
		Label:         label,
		Fields:        trimStringList(raw.Fields),
		FileName:      path,
	}, nil
}

func loadLayout(path string, ordinal int) (layoutMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return layoutMetadata{}, err
	}
	var raw layoutXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return layoutMetadata{}, err
	}
	name := trimKnownSuffix(filepath.Base(path), ".layout-meta.xml")
	objectName := name
	if idx := strings.Index(name, "-"); idx > 0 {
		objectName = name[:idx]
	}
	sections := make([]layoutSectionMetadata, 0, len(raw.Sections))
	for i, section := range raw.Sections {
		label := strings.TrimSpace(section.Label)
		if label == "" {
			label = fmt.Sprintf("Section %d", i+1)
		}
		columns := make([]layoutColumnMetadata, 0, len(section.Columns))
		for _, column := range section.Columns {
			items := make([]layoutItemMetadata, 0, len(column.Items))
			for _, item := range column.Items {
				field := strings.TrimSpace(item.Field)
				if field == "" {
					continue
				}
				items = append(items, layoutItemMetadata{
					Field:    field,
					Behavior: strings.TrimSpace(item.Behavior),
				})
			}
			columns = append(columns, layoutColumnMetadata{Items: items})
		}
		sections = append(sections, layoutSectionMetadata{
			ID:         fmt.Sprintf("section-%d", i+1),
			Label:      label,
			Style:      strings.TrimSpace(section.Style),
			UseHeading: label != "",
			Columns:    columns,
		})
	}
	return layoutMetadata{ID: string(sequenceID("00h", ordinal)), ObjectName: objectName, Name: name, FileName: path, Sections: sections}, nil
}

func (m *SourceMetadata) addToolingSchemaMetadataRecords() error {
	for i, path := range m.Project.ObjectFiles {
		name := trimKnownSuffix(filepath.Base(path), ".object-meta.xml")
		label, err := customObjectLabel(path, name)
		if err != nil {
			return err
		}
		m.addToolingRecord("CustomObject", sequenceID("01I", i+1), map[string]storage.Value{
			"DeveloperName":   storage.StringValue(name),
			"FullName":        storage.StringValue(name),
			"MasterLabel":     storage.StringValue(label),
			"NamespacePrefix": storage.StringValue(m.Project.Namespace),
			"ManageableState": storage.StringValue("unmanaged"),
		})
	}
	for i, path := range m.Project.FieldFiles {
		objectName := objectNameFromNestedMetadata(path, "fields")
		name := trimKnownSuffix(filepath.Base(path), ".field-meta.xml")
		field, err := loadCustomField(path)
		if err != nil {
			return err
		}
		if field.FullName != "" {
			name = field.FullName
		}
		m.addToolingRecord("CustomField", sequenceID("00N", i+1), map[string]storage.Value{
			"DeveloperName":   storage.StringValue(name),
			"FullName":        storage.StringValue(objectName + "." + name),
			"TableEnumOrId":   storage.StringValue(objectName),
			"NamespacePrefix": storage.StringValue(m.Project.Namespace),
			"ManageableState": storage.StringValue("unmanaged"),
			"MetadataType":    storage.StringValue(field.Type),
		})
	}
	for _, layouts := range m.Layouts {
		for _, layout := range layouts {
			m.addToolingRecord("Layout", storage.ID(layout.ID), map[string]storage.Value{
				"Name":            storage.StringValue(layout.Name),
				"FullName":        storage.StringValue(layout.Name),
				"TableEnumOrId":   storage.StringValue(layout.ObjectName),
				"ManageableState": storage.StringValue("unmanaged"),
			})
		}
	}
	for _, compactLayouts := range m.Compact {
		for _, compact := range compactLayouts {
			m.addToolingRecord("CompactLayout", storage.ID(compact.ID), map[string]storage.Value{
				"DeveloperName":   storage.StringValue(compact.DeveloperName),
				"FullName":        storage.StringValue(compact.ObjectName + "." + compact.DeveloperName),
				"MasterLabel":     storage.StringValue(compact.Label),
				"SobjectType":     storage.StringValue(compact.ObjectName),
				"NamespacePrefix": storage.StringValue(m.Project.Namespace),
				"ManageableState": storage.StringValue("unmanaged"),
			})
		}
	}
	for i, path := range m.Project.RecordTypeFiles {
		objectName := objectNameFromNestedMetadata(path, "recordTypes")
		name := trimKnownSuffix(filepath.Base(path), ".recordType-meta.xml")
		recordType, err := loadRecordType(path)
		if err != nil {
			return err
		}
		label := strings.TrimSpace(recordType.Label)
		if label == "" {
			label = name
		}
		active := metadataBoolean(recordType.Active, true)
		m.addToolingRecord("RecordType", sequenceID("012", i+1), map[string]storage.Value{
			"DeveloperName": storage.StringValue(name),
			"FullName":      storage.StringValue(objectName + "." + name),
			"Name":          storage.StringValue(label),
			"SobjectType":   storage.StringValue(objectName),
			"Active":        storage.BooleanValue(active),
			"IsActive":      storage.BooleanValue(active),
		})
	}
	for i, path := range m.Project.ValidationRuleFiles {
		objectName := objectNameFromNestedMetadata(path, "validationRules")
		name := trimKnownSuffix(filepath.Base(path), ".validationRule-meta.xml")
		rule, err := loadValidationRule(path)
		if err != nil {
			return err
		}
		m.addToolingRecord("ValidationRule", sequenceID("03d", i+1), map[string]storage.Value{
			"ValidationName":     storage.StringValue(name),
			"FullName":           storage.StringValue(objectName + "." + name),
			"EntityDefinitionId": storage.StringValue(objectName),
			"Active":             storage.BooleanValue(metadataBoolean(rule.Active, false)),
			"ErrorMessage":       storage.StringValue(strings.TrimSpace(rule.ErrorMessage)),
			"Description":        storage.StringValue(strings.TrimSpace(rule.Description)),
		})
	}
	return nil
}

func (m *SourceMetadata) addToolingRecord(objectName string, id storage.ID, fields map[string]storage.Value) {
	state := m.ToolingOrg.Objects[objectName]
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	state.Records[id] = storage.Record{ID: id, Object: objectName, Fields: fields}
	m.ToolingOrg.Objects[objectName] = state
}

func customObjectLabel(path, fallback string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var raw customObjectXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return "", err
	}
	label := strings.TrimSpace(raw.Label)
	if label == "" {
		label = fallback
	}
	return label, nil
}

func loadCustomField(path string) (customFieldXML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return customFieldXML{}, err
	}
	var raw customFieldXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return customFieldXML{}, err
	}
	raw.FullName = strings.TrimSpace(raw.FullName)
	raw.Label = strings.TrimSpace(raw.Label)
	raw.Type = strings.TrimSpace(raw.Type)
	return raw, nil
}

func loadRecordType(path string) (recordTypeXML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return recordTypeXML{}, err
	}
	var raw recordTypeXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return recordTypeXML{}, err
	}
	return raw, nil
}

func loadValidationRule(path string) (validationRuleXML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return validationRuleXML{}, err
	}
	var raw validationRuleXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return validationRuleXML{}, err
	}
	return raw, nil
}

func metadataBoolean(value string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true":
		return true
	case "false":
		return false
	default:
		return fallback
	}
}

func schemaBackedComponents(p project.Project) []metadataComponent {
	var out []metadataComponent
	add := func(typ, fullName, path, prefix string, n int) {
		if fullName == "" {
			return
		}
		out = append(out, metadataComponent{Type: typ, FullName: fullName, FileName: path, ID: sequenceID(prefix, n)})
	}
	for i, path := range p.ObjectFiles {
		add("CustomObject", trimKnownSuffix(filepath.Base(path), ".object-meta.xml"), path, "01I", i+1)
	}
	for i, path := range p.FieldFiles {
		objectName := objectNameFromNestedMetadata(path, "fields")
		add("CustomField", objectName+"."+trimKnownSuffix(filepath.Base(path), ".field-meta.xml"), path, "00N", i+1)
	}
	for i, path := range p.RecordTypeFiles {
		objectName := objectNameFromNestedMetadata(path, "recordTypes")
		add("RecordType", objectName+"."+trimKnownSuffix(filepath.Base(path), ".recordType-meta.xml"), path, "012", i+1)
	}
	for i, path := range p.ValidationRuleFiles {
		objectName := objectNameFromNestedMetadata(path, "validationRules")
		add("ValidationRule", objectName+"."+trimKnownSuffix(filepath.Base(path), ".validationRule-meta.xml"), path, "03d", i+1)
	}
	for i, path := range p.WorkflowFiles {
		add("Workflow", trimKnownSuffix(filepath.Base(path), ".workflow-meta.xml"), path, "04W", i+1)
	}
	return out
}

func (m *SourceMetadata) sortAndIndexComponents() {
	for objectName := range m.ListViews {
		sort.Slice(m.ListViews[objectName], func(i, j int) bool {
			return m.ListViews[objectName][i].DeveloperName < m.ListViews[objectName][j].DeveloperName
		})
	}
	for objectName := range m.Layouts {
		sort.Slice(m.Layouts[objectName], func(i, j int) bool { return m.Layouts[objectName][i].Name < m.Layouts[objectName][j].Name })
	}
	for objectName := range m.Compact {
		sort.Slice(m.Compact[objectName], func(i, j int) bool {
			return m.Compact[objectName][i].DeveloperName < m.Compact[objectName][j].DeveloperName
		})
	}
	sort.Slice(m.Components, func(i, j int) bool {
		if m.Components[i].Type == m.Components[j].Type {
			return m.Components[i].FullName < m.Components[j].FullName
		}
		return m.Components[i].Type < m.Components[j].Type
	})
	m.componentBy = make(map[string]metadataComponent, len(m.Components))
	for _, component := range m.Components {
		m.componentBy[metadataComponentKey(component.Type, component.FullName)] = component
	}
}

func toolingObjectDefinition(apiName, label, prefix string, fields []string) storage.ObjectDefinition {
	def := storage.ObjectDefinition{APIName: apiName, Label: label, PluralLabel: label + "s", KeyPrefix: prefix, Fields: map[string]storage.Field{
		"Id": {APIName: "Id", Type: storage.FieldID},
	}}
	for _, field := range fields {
		fieldType := storage.FieldString
		switch field {
		case "BodyCrc", "LengthWithoutComments":
			fieldType = storage.FieldInteger
		case "ApiVersion":
			fieldType = storage.FieldDecimal
		case "IsValid", "Active", "IsActive":
			fieldType = storage.FieldBoolean
		}
		def.Fields[field] = storage.Field{APIName: field, Type: fieldType}
	}
	return def
}

func sourceAPIVersion(version string) string {
	if strings.TrimSpace(version) == "" {
		return storage.DefaultRESTAPIVersion
	}
	return strings.TrimSpace(version)
}

func triggerObjectName(body string) string {
	fields := strings.Fields(body)
	for i := 0; i+1 < len(fields); i++ {
		if strings.EqualFold(fields[i], "on") {
			return strings.Trim(fields[i+1], "({")
		}
	}
	return ""
}

func objectNameFromNestedMetadata(path, folder string) string {
	dir := filepath.Dir(path)
	if filepath.Base(dir) != folder {
		return ""
	}
	return filepath.Base(filepath.Dir(dir))
}

func trimStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func sequenceID(prefix string, ordinal int) storage.ID {
	return storage.ID(fmt.Sprintf("%s%012d", prefix, ordinal))
}

func trimKnownSuffix(name, suffix string) string {
	if len(name) >= len(suffix) && strings.EqualFold(name[len(name)-len(suffix):], suffix) {
		return name[:len(name)-len(suffix)]
	}
	return strings.TrimSuffix(name, suffix)
}

func metadataComponentKey(typ, fullName string) string {
	return strings.ToLower(strings.TrimSpace(typ)) + ":" + strings.ToLower(strings.TrimSpace(fullName))
}
