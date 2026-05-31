package server

import (
	"encoding/xml"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strings"

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

func (m *SourceMetadata) loadToolingObjects() error {
	defs := map[string]storage.ObjectDefinition{
		"ApexClass":      toolingObjectDefinition("ApexClass", "Apex Class", "01p", []string{"Name", "Body", "BodyCrc", "ApiVersion", "Status", "IsValid", "LengthWithoutComments", "NamespacePrefix"}),
		"ApexTrigger":    toolingObjectDefinition("ApexTrigger", "Apex Trigger", "01q", []string{"Name", "Body", "BodyCrc", "ApiVersion", "Status", "IsValid", "LengthWithoutComments", "TableEnumOrId"}),
		"ApexPage":       toolingObjectDefinition("ApexPage", "Apex Page", "066", []string{"Name", "Markup", "ApiVersion", "MasterLabel"}),
		"ApexComponent":  toolingObjectDefinition("ApexComponent", "Apex Component", "099", []string{"Name", "Markup", "ApiVersion", "MasterLabel"}),
		"StaticResource": toolingObjectDefinition("StaticResource", "Static Resource", "081", []string{"Name", "Body", "ContentType", "CacheControl", "Description"}),
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
		layout := loadLayout(path, i+1)
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

func loadLayout(path string, ordinal int) layoutMetadata {
	name := trimKnownSuffix(filepath.Base(path), ".layout-meta.xml")
	objectName := name
	if idx := strings.Index(name, "-"); idx > 0 {
		objectName = name[:idx]
	}
	return layoutMetadata{ID: string(sequenceID("00h", ordinal)), ObjectName: objectName, Name: name, FileName: path}
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
		case "IsValid":
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
