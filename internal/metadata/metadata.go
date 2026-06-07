package metadata

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

type Index struct {
	CustomLabels          []CustomLabel          `json:"customLabels,omitempty"`
	StaticResources       []StaticResource       `json:"staticResources,omitempty"`
	NamedCredentials      []NamedCredential      `json:"namedCredentials,omitempty"`
	RemoteSites           []RemoteSite           `json:"remoteSites,omitempty"`
	CustomMetadataRecords []CustomMetadataRecord `json:"customMetadataRecords,omitempty"`
	FieldSets             []FieldSet             `json:"fieldSets,omitempty"`
	VisualforcePages      []NamedAsset           `json:"visualforcePages,omitempty"`
	VisualforceComponents []NamedAsset           `json:"visualforceComponents,omitempty"`
	AuraComponents        []NamedAsset           `json:"auraComponents,omitempty"`
	LWCComponents         []NamedAsset           `json:"lwcComponents,omitempty"`
	Workflows             []NamedAsset           `json:"workflows,omitempty"`
	Flows                 []NamedAsset           `json:"flows,omitempty"`
	Layouts               []NamedAsset           `json:"layouts,omitempty"`
	CompactLayouts        []NamedAsset           `json:"compactLayouts,omitempty"`
	Tabs                  []NamedAsset           `json:"tabs,omitempty"`
	WebLinks              []NamedAsset           `json:"webLinks,omitempty"`
	QuickActions          []NamedAsset           `json:"quickActions,omitempty"`
	GlobalValueSets       []NamedAsset           `json:"globalValueSets,omitempty"`
	StandardValueSets     []NamedAsset           `json:"standardValueSets,omitempty"`
	FlexiPages            []NamedAsset           `json:"flexiPages,omitempty"`
	Applications          []NamedAsset           `json:"applications,omitempty"`
	Profiles              []PermissionContainer  `json:"profiles,omitempty"`
	PermissionSets        []PermissionContainer  `json:"permissionSets,omitempty"`
	PermissionAssignments []NamedAsset           `json:"permissionAssignments,omitempty"`

	labelsByName    map[string]int
	resourcesByName map[string]int
	endpointsByName map[string][]EndpointRef
	recordsByName   map[string]int
	fieldSetsByName map[string]int
}

type CustomLabel struct {
	Name             string `json:"name"`
	Value            string `json:"value,omitempty"`
	Language         string `json:"language,omitempty"`
	Protected        bool   `json:"protected,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
	Categories       string `json:"categories,omitempty"`
	File             string `json:"file,omitempty"`
}

type StaticResource struct {
	Name         string `json:"name"`
	ContentPath  string `json:"contentPath,omitempty"`
	MetadataPath string `json:"metadataPath,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
	CacheControl string `json:"cacheControl,omitempty"`
	Description  string `json:"description,omitempty"`
}

type NamedCredential struct {
	Name          string `json:"name"`
	Endpoint      string `json:"endpoint,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	PrincipalType string `json:"principalType,omitempty"`
	Label         string `json:"label,omitempty"`
	File          string `json:"file,omitempty"`
}

type RemoteSite struct {
	Name        string `json:"name"`
	URL         string `json:"url,omitempty"`
	Active      bool   `json:"active,omitempty"`
	Description string `json:"description,omitempty"`
	File        string `json:"file,omitempty"`
}

type CustomMetadataRecord struct {
	FullName      string                `json:"fullName"`
	ObjectName    string                `json:"objectName,omitempty"`
	DeveloperName string                `json:"developerName,omitempty"`
	Label         string                `json:"label,omitempty"`
	Protected     bool                  `json:"protected,omitempty"`
	Values        []CustomMetadataValue `json:"values,omitempty"`
	File          string                `json:"file,omitempty"`
}

type CustomMetadataValue struct {
	Field string `json:"field"`
	Value string `json:"value,omitempty"`
}

type FieldSet struct {
	ObjectName  string           `json:"objectName,omitempty"`
	Name        string           `json:"name"`
	Label       string           `json:"label,omitempty"`
	Description string           `json:"description,omitempty"`
	Fields      []FieldSetMember `json:"fields,omitempty"`
	File        string           `json:"file,omitempty"`
}

type FieldSetMember struct {
	Field    string `json:"field"`
	Required bool   `json:"required,omitempty"`
}

type NamedAsset struct {
	Name string `json:"name"`
	File string `json:"file,omitempty"`
}

type PermissionContainer struct {
	Name              string                 `json:"name"`
	Label             string                 `json:"label,omitempty"`
	ObjectPermissions []ObjectPermissionStub `json:"objectPermissions,omitempty"`
	FieldPermissions  []FieldPermissionStub  `json:"fieldPermissions,omitempty"`
	File              string                 `json:"file,omitempty"`
}

type ObjectPermissionStub struct {
	Object           string `json:"object"`
	Read             bool   `json:"read,omitempty"`
	Create           bool   `json:"create,omitempty"`
	Edit             bool   `json:"edit,omitempty"`
	Delete           bool   `json:"delete,omitempty"`
	ViewAllRecords   bool   `json:"viewAllRecords,omitempty"`
	ModifyAllRecords bool   `json:"modifyAllRecords,omitempty"`
}

type FieldPermissionStub struct {
	Field    string `json:"field"`
	Readable bool   `json:"readable,omitempty"`
	Editable bool   `json:"editable,omitempty"`
}

type EndpointRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

func LoadProject(p project.Project) (Index, error) {
	idx := Index{}

	for _, path := range p.LabelFiles {
		labels, err := loadLabels(path)
		if err != nil {
			return Index{}, err
		}
		idx.CustomLabels = append(idx.CustomLabels, labels...)
	}

	resources, err := loadStaticResources(p.StaticResourceFiles, p.StaticResourceMetas)
	if err != nil {
		return Index{}, err
	}
	idx.StaticResources = resources

	for _, path := range p.NamedCredentialFiles {
		credential, err := loadNamedCredential(path)
		if err != nil {
			return Index{}, err
		}
		idx.NamedCredentials = append(idx.NamedCredentials, credential)
	}
	for _, path := range p.RemoteSiteFiles {
		site, err := loadRemoteSite(path)
		if err != nil {
			return Index{}, err
		}
		idx.RemoteSites = append(idx.RemoteSites, site)
	}
	for _, path := range p.CustomMetadataFiles {
		record, err := loadCustomMetadataRecord(path)
		if err != nil {
			return Index{}, err
		}
		idx.CustomMetadataRecords = append(idx.CustomMetadataRecords, record)
	}
	for _, path := range p.FieldSetFiles {
		fieldSet, err := loadFieldSet(path)
		if err != nil {
			return Index{}, err
		}
		idx.FieldSets = append(idx.FieldSets, fieldSet)
	}
	for _, path := range p.ObjectFiles {
		fieldSets, err := loadObjectFieldSets(path)
		if err != nil {
			return Index{}, err
		}
		idx.FieldSets = append(idx.FieldSets, fieldSets...)
	}
	idx.VisualforcePages = namedAssets(p.VisualforcePageFiles, ".page")
	idx.VisualforceComponents = namedAssets(p.VisualforceComponentFiles, ".component")
	idx.AuraComponents = componentAssets(p.AuraFiles)
	idx.LWCComponents = componentAssets(p.LWCFiles)
	idx.Workflows = namedAssets(p.WorkflowFiles, ".workflow-meta.xml")
	idx.Flows = namedAssets(p.FlowFiles, ".flow-meta.xml", ".flow")
	idx.Layouts = namedAssets(p.LayoutFiles, ".layout-meta.xml", ".layout")
	idx.CompactLayouts = namedAssets(p.CompactLayoutFiles, ".compactLayout-meta.xml")
	idx.Tabs = namedAssets(p.TabFiles, ".tab-meta.xml", ".tab")
	idx.WebLinks = namedAssets(p.WebLinkFiles, ".webLink-meta.xml", ".weblink-meta.xml", ".webLink", ".weblink")
	idx.QuickActions = namedAssets(p.QuickActionFiles, ".quickAction-meta.xml", ".quickaction-meta.xml", ".quickAction", ".quickaction")
	idx.GlobalValueSets = namedAssets(p.GlobalValueSetFiles, ".globalValueSet-meta.xml", ".globalvalueset-meta.xml", ".globalValueSet", ".globalvalueset")
	idx.StandardValueSets = namedAssets(p.StandardValueSetFiles, ".standardValueSet-meta.xml", ".standardvalueset-meta.xml", ".standardValueSet", ".standardvalueset")
	idx.FlexiPages = namedAssets(p.FlexiPageFiles, ".flexipage-meta.xml", ".flexipage")
	idx.Applications = namedAssets(p.ApplicationFiles, ".app-meta.xml", ".app")
	for _, path := range p.ProfileFiles {
		container, err := loadPermissionContainer(path, ".profile-meta.xml", ".profile")
		if err != nil {
			return Index{}, err
		}
		idx.Profiles = append(idx.Profiles, container)
	}
	for _, path := range p.PermissionSetFiles {
		container, err := loadPermissionContainer(path, ".permissionset-meta.xml", ".permissionset")
		if err != nil {
			return Index{}, err
		}
		idx.PermissionSets = append(idx.PermissionSets, container)
	}
	idx.PermissionAssignments = namedAssets(p.PermissionAssignmentFiles, ".permissionsetassignment-meta.xml", ".permissionsetassignment")

	idx.sortAndBuildLookups()
	return idx, nil
}

func (i Index) CustomLabel(name string) (CustomLabel, bool) {
	idx, ok := i.labelsByName[lookupKey(name)]
	if !ok {
		return CustomLabel{}, false
	}
	return i.CustomLabels[idx], true
}

func (i Index) StaticResource(name string) (StaticResource, bool) {
	idx, ok := i.resourcesByName[lookupKey(name)]
	if !ok {
		return StaticResource{}, false
	}
	return i.StaticResources[idx], true
}

func (i Index) Endpoint(name string) (EndpointRef, bool) {
	endpoints := i.EndpointRefs(name)
	if len(endpoints) == 0 {
		return EndpointRef{}, false
	}
	return endpoints[0], true
}

func (i Index) EndpointRefs(name string) []EndpointRef {
	endpoints := i.endpointsByName[lookupKey(name)]
	if len(endpoints) == 0 {
		return nil
	}
	return append([]EndpointRef(nil), endpoints...)
}

func (i Index) NamedCredential(name string) (NamedCredential, bool) {
	key := lookupKey(name)
	for _, credential := range i.NamedCredentials {
		if lookupKey(credential.Name) == key {
			return credential, true
		}
	}
	return NamedCredential{}, false
}

func (i Index) RemoteSite(name string) (RemoteSite, bool) {
	key := lookupKey(name)
	for _, site := range i.RemoteSites {
		if lookupKey(site.Name) == key {
			return site, true
		}
	}
	return RemoteSite{}, false
}

func (i Index) CustomMetadataRecord(fullName string) (CustomMetadataRecord, bool) {
	idx, ok := i.recordsByName[lookupKey(fullName)]
	if !ok {
		return CustomMetadataRecord{}, false
	}
	return i.CustomMetadataRecords[idx], true
}

func (i Index) FieldSet(objectName, name string) (FieldSet, bool) {
	idx, ok := i.fieldSetsByName[lookupKey(objectName+"."+name)]
	if !ok {
		return FieldSet{}, false
	}
	return i.FieldSets[idx], true
}

func (i *Index) sortAndBuildLookups() {
	sort.Slice(i.CustomLabels, func(a, b int) bool { return i.CustomLabels[a].Name < i.CustomLabels[b].Name })
	sort.Slice(i.StaticResources, func(a, b int) bool { return i.StaticResources[a].Name < i.StaticResources[b].Name })
	sort.Slice(i.NamedCredentials, func(a, b int) bool { return i.NamedCredentials[a].Name < i.NamedCredentials[b].Name })
	sort.Slice(i.RemoteSites, func(a, b int) bool { return i.RemoteSites[a].Name < i.RemoteSites[b].Name })
	sort.Slice(i.CustomMetadataRecords, func(a, b int) bool { return i.CustomMetadataRecords[a].FullName < i.CustomMetadataRecords[b].FullName })
	sort.Slice(i.FieldSets, func(a, b int) bool {
		if i.FieldSets[a].ObjectName != i.FieldSets[b].ObjectName {
			return i.FieldSets[a].ObjectName < i.FieldSets[b].ObjectName
		}
		return i.FieldSets[a].Name < i.FieldSets[b].Name
	})
	sortNamedAssets(i.VisualforcePages)
	sortNamedAssets(i.VisualforceComponents)
	sortNamedAssets(i.AuraComponents)
	sortNamedAssets(i.LWCComponents)
	sortNamedAssets(i.Workflows)
	sortNamedAssets(i.Flows)
	sortNamedAssets(i.Layouts)
	sortNamedAssets(i.CompactLayouts)
	sortNamedAssets(i.Tabs)
	sortNamedAssets(i.WebLinks)
	sortNamedAssets(i.QuickActions)
	sortNamedAssets(i.GlobalValueSets)
	sortNamedAssets(i.StandardValueSets)
	sortNamedAssets(i.FlexiPages)
	sortNamedAssets(i.Applications)
	sort.Slice(i.Profiles, func(a, b int) bool { return i.Profiles[a].Name < i.Profiles[b].Name })
	sort.Slice(i.PermissionSets, func(a, b int) bool { return i.PermissionSets[a].Name < i.PermissionSets[b].Name })
	sortNamedAssets(i.PermissionAssignments)

	i.labelsByName = make(map[string]int, len(i.CustomLabels))
	for n, label := range i.CustomLabels {
		i.labelsByName[lookupKey(label.Name)] = n
	}
	i.resourcesByName = make(map[string]int, len(i.StaticResources))
	for n, resource := range i.StaticResources {
		i.resourcesByName[lookupKey(resource.Name)] = n
	}
	i.endpointsByName = make(map[string][]EndpointRef, len(i.NamedCredentials)+len(i.RemoteSites))
	for _, credential := range i.NamedCredentials {
		key := lookupKey(credential.Name)
		i.endpointsByName[key] = append(i.endpointsByName[key], EndpointRef{Kind: "NamedCredential", Name: credential.Name, URL: credential.Endpoint})
	}
	for _, site := range i.RemoteSites {
		key := lookupKey(site.Name)
		i.endpointsByName[key] = append(i.endpointsByName[key], EndpointRef{Kind: "RemoteSiteSetting", Name: site.Name, URL: site.URL})
	}
	i.recordsByName = make(map[string]int, len(i.CustomMetadataRecords))
	for n, record := range i.CustomMetadataRecords {
		i.recordsByName[lookupKey(record.FullName)] = n
	}
	i.fieldSetsByName = make(map[string]int, len(i.FieldSets))
	for n, fieldSet := range i.FieldSets {
		i.fieldSetsByName[lookupKey(fieldSet.ObjectName+"."+fieldSet.Name)] = n
	}
}

type customLabelsXML struct {
	Labels []customLabelXML `xml:"labels"`
}

type customLabelXML struct {
	FullName         string `xml:"fullName"`
	Value            string `xml:"value"`
	Language         string `xml:"language"`
	Protected        bool   `xml:"protected"`
	ShortDescription string `xml:"shortDescription"`
	Categories       string `xml:"categories"`
}

func loadLabels(path string) ([]CustomLabel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw customLabelsXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	labels := make([]CustomLabel, 0, len(raw.Labels))
	for _, label := range raw.Labels {
		name := strings.TrimSpace(label.FullName)
		if name == "" {
			continue
		}
		labels = append(labels, CustomLabel{
			Name:             name,
			Value:            strings.TrimSpace(label.Value),
			Language:         strings.TrimSpace(label.Language),
			Protected:        label.Protected,
			ShortDescription: strings.TrimSpace(label.ShortDescription),
			Categories:       strings.TrimSpace(label.Categories),
			File:             path,
		})
	}
	return labels, nil
}

type staticResourceXML struct {
	CacheControl string `xml:"cacheControl"`
	ContentType  string `xml:"contentType"`
	Description  string `xml:"description"`
}

func loadStaticResources(contentFiles, metaFiles []string) ([]StaticResource, error) {
	byName := make(map[string]*StaticResource)
	for _, path := range contentFiles {
		name := resourceNameFromContentPath(path)
		byName[lookupKey(name)] = &StaticResource{Name: name, ContentPath: path}
	}
	for _, path := range metaFiles {
		meta, err := loadStaticResourceMeta(path)
		if err != nil {
			return nil, err
		}
		name := resourceNameFromMetaPath(path)
		key := lookupKey(name)
		resource := byName[key]
		if resource == nil {
			resource = &StaticResource{Name: name}
			byName[key] = resource
		}
		resource.MetadataPath = path
		resource.CacheControl = strings.TrimSpace(meta.CacheControl)
		resource.ContentType = strings.TrimSpace(meta.ContentType)
		resource.Description = strings.TrimSpace(meta.Description)
	}
	out := make([]StaticResource, 0, len(byName))
	for _, resource := range byName {
		out = append(out, *resource)
	}
	return out, nil
}

func loadStaticResourceMeta(path string) (staticResourceXML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return staticResourceXML{}, err
	}
	var raw staticResourceXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return staticResourceXML{}, err
	}
	return raw, nil
}

type namedCredentialXML struct {
	FullName      string `xml:"fullName"`
	Endpoint      string `xml:"endpoint"`
	Protocol      string `xml:"protocol"`
	PrincipalType string `xml:"principalType"`
	Label         string `xml:"label"`
}

func loadNamedCredential(path string) (NamedCredential, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return NamedCredential{}, err
	}
	var raw namedCredentialXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return NamedCredential{}, err
	}
	name := strings.TrimSpace(raw.FullName)
	if name == "" {
		name = baseNoMetaExt(path)
	}
	return NamedCredential{
		Name:          name,
		Endpoint:      strings.TrimSpace(raw.Endpoint),
		Protocol:      strings.TrimSpace(raw.Protocol),
		PrincipalType: strings.TrimSpace(raw.PrincipalType),
		Label:         strings.TrimSpace(raw.Label),
		File:          path,
	}, nil
}

type remoteSiteXML struct {
	FullName    string `xml:"fullName"`
	URL         string `xml:"url"`
	Active      bool   `xml:"isActive"`
	Description string `xml:"description"`
}

func loadRemoteSite(path string) (RemoteSite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RemoteSite{}, err
	}
	var raw remoteSiteXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return RemoteSite{}, err
	}
	name := strings.TrimSpace(raw.FullName)
	if name == "" {
		name = baseNoMetaExt(path)
	}
	return RemoteSite{
		Name:        name,
		URL:         strings.TrimSpace(raw.URL),
		Active:      raw.Active,
		Description: strings.TrimSpace(raw.Description),
		File:        path,
	}, nil
}

type customMetadataXML struct {
	Label     string                   `xml:"label"`
	Protected bool                     `xml:"protected"`
	Values    []customMetadataValueXML `xml:"values"`
}

type customMetadataValueXML struct {
	Field string `xml:"field"`
	Value string `xml:"value"`
}

func loadCustomMetadataRecord(path string) (CustomMetadataRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CustomMetadataRecord{}, err
	}
	var raw customMetadataXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return CustomMetadataRecord{}, err
	}
	fullName := trimKnownSuffix(filepath.Base(path), ".md")
	objectName, developerName := customMetadataNames(fullName)
	values := make([]CustomMetadataValue, 0, len(raw.Values))
	for _, value := range raw.Values {
		field := strings.TrimSpace(value.Field)
		if field == "" {
			continue
		}
		values = append(values, CustomMetadataValue{Field: field, Value: strings.TrimSpace(value.Value)})
	}
	return CustomMetadataRecord{
		FullName:      fullName,
		ObjectName:    objectName,
		DeveloperName: developerName,
		Label:         strings.TrimSpace(raw.Label),
		Protected:     raw.Protected,
		Values:        values,
		File:          path,
	}, nil
}

type fieldSetXML struct {
	FullName        string              `xml:"fullName"`
	Label           string              `xml:"label"`
	Description     string              `xml:"description"`
	DisplayedFields []fieldSetMemberXML `xml:"displayedFields"`
	AvailableFields []fieldSetMemberXML `xml:"availableFields"`
}

type objectFieldSetsXML struct {
	FieldSets []fieldSetXML `xml:"fieldSets"`
}

type fieldSetMemberXML struct {
	Field    string `xml:"field"`
	Required bool   `xml:"isRequired"`
}

func loadFieldSet(path string) (FieldSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FieldSet{}, err
	}
	var raw fieldSetXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return FieldSet{}, err
	}
	name := strings.TrimSpace(raw.FullName)
	if name == "" {
		name = trimKnownSuffix(filepath.Base(path), ".fieldSet-meta.xml")
	}
	return fieldSetFromXML(raw, objectNameFromFieldSetPath(path), name, path), nil
}

func loadObjectFieldSets(path string) ([]FieldSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw objectFieldSetsXML
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	out := make([]FieldSet, 0, len(raw.FieldSets))
	objectName := objectNameFromObjectPath(path)
	for _, rawFieldSet := range raw.FieldSets {
		name := strings.TrimSpace(rawFieldSet.FullName)
		if name == "" {
			continue
		}
		out = append(out, fieldSetFromXML(rawFieldSet, objectName, name, path))
	}
	return out, nil
}

func fieldSetFromXML(raw fieldSetXML, objectName, name, path string) FieldSet {
	members := make([]FieldSetMember, 0, len(raw.DisplayedFields)+len(raw.AvailableFields))
	addMembers := func(rawMembers []fieldSetMemberXML) {
		for _, member := range rawMembers {
			field := strings.TrimSpace(member.Field)
			if field == "" {
				continue
			}
			members = append(members, FieldSetMember{Field: field, Required: member.Required})
		}
	}
	addMembers(raw.DisplayedFields)
	addMembers(raw.AvailableFields)
	return FieldSet{
		ObjectName:  objectName,
		Name:        name,
		Label:       strings.TrimSpace(raw.Label),
		Description: strings.TrimSpace(raw.Description),
		Fields:      members,
		File:        path,
	}
}

type permissionContainerXML struct {
	FullName          string          `xml:"fullName"`
	Label             string          `xml:"label"`
	ObjectPermissions []objectPermXML `xml:"objectPermissions"`
	FieldPermissions  []fieldPermXML  `xml:"fieldPermissions"`
}

type objectPermXML struct {
	Object           string `xml:"object"`
	AllowRead        bool   `xml:"allowRead"`
	AllowCreate      bool   `xml:"allowCreate"`
	AllowEdit        bool   `xml:"allowEdit"`
	AllowDelete      bool   `xml:"allowDelete"`
	ViewAllRecords   bool   `xml:"viewAllRecords"`
	ModifyAllRecords bool   `xml:"modifyAllRecords"`
}

type fieldPermXML struct {
	Field    string `xml:"field"`
	Readable bool   `xml:"readable"`
	Editable bool   `xml:"editable"`
}

func loadPermissionContainer(path string, suffixes ...string) (PermissionContainer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PermissionContainer{}, err
	}
	var raw permissionContainerXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return PermissionContainer{}, err
	}
	name := strings.TrimSpace(raw.FullName)
	if name == "" {
		name = trimAnySuffix(filepath.Base(path), suffixes...)
	}
	container := PermissionContainer{Name: name, Label: strings.TrimSpace(raw.Label), File: path}
	for _, perm := range raw.ObjectPermissions {
		objectName := strings.TrimSpace(perm.Object)
		if objectName == "" {
			continue
		}
		container.ObjectPermissions = append(container.ObjectPermissions, ObjectPermissionStub{
			Object:           objectName,
			Read:             perm.AllowRead,
			Create:           perm.AllowCreate,
			Edit:             perm.AllowEdit,
			Delete:           perm.AllowDelete,
			ViewAllRecords:   perm.ViewAllRecords,
			ModifyAllRecords: perm.ModifyAllRecords,
		})
	}
	for _, perm := range raw.FieldPermissions {
		fieldName := strings.TrimSpace(perm.Field)
		if fieldName == "" {
			continue
		}
		container.FieldPermissions = append(container.FieldPermissions, FieldPermissionStub{Field: fieldName, Readable: perm.Readable, Editable: perm.Editable})
	}
	return container, nil
}

func namedAssets(paths []string, suffixes ...string) []NamedAsset {
	assets := make([]NamedAsset, 0, len(paths))
	for _, path := range paths {
		assets = append(assets, NamedAsset{Name: trimAnySuffix(filepath.Base(path), suffixes...), File: path})
	}
	sortNamedAssets(assets)
	return assets
}

func componentAssets(paths []string) []NamedAsset {
	seen := make(map[string]string)
	for _, path := range paths {
		dir := filepath.Dir(path)
		name := filepath.Base(dir)
		if _, ok := seen[name]; !ok {
			seen[name] = dir
		}
	}
	assets := make([]NamedAsset, 0, len(seen))
	for name, path := range seen {
		assets = append(assets, NamedAsset{Name: name, File: path})
	}
	sortNamedAssets(assets)
	return assets
}

func sortNamedAssets(assets []NamedAsset) {
	sort.Slice(assets, func(a, b int) bool { return assets[a].Name < assets[b].Name })
}

func customMetadataNames(fullName string) (string, string) {
	parts := strings.SplitN(fullName, ".", 2)
	if len(parts) != 2 {
		return "", fullName
	}
	objectName := parts[0]
	if !strings.HasSuffix(objectName, "__mdt") {
		objectName += "__mdt"
	}
	return objectName, parts[1]
}

func resourceNameFromMetaPath(path string) string {
	base := filepath.Base(path)
	base = trimKnownSuffix(base, ".staticresource-meta.xml")
	return trimKnownSuffix(base, ".resource-meta.xml")
}

func resourceNameFromContentPath(path string) string {
	base := filepath.Base(path)
	if name := trimKnownSuffix(base, ".resource"); name != base {
		return name
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func baseNoMetaExt(path string) string {
	base := filepath.Base(path)
	for _, suffix := range []string{".namedCredential-meta.xml", ".namedCredential", ".remoteSite-meta.xml", ".remoteSite"} {
		base = trimKnownSuffix(base, suffix)
	}
	return base
}

func objectNameFromFieldSetPath(path string) string {
	dir := filepath.Dir(filepath.Dir(path))
	if filepath.Base(filepath.Dir(path)) != "fieldSets" {
		return ""
	}
	return filepath.Base(dir)
}

func objectNameFromObjectPath(path string) string {
	base := filepath.Base(path)
	for _, suffix := range []string{".object-meta.xml", ".object"} {
		if hasSuffixFold(base, suffix) {
			return base[:len(base)-len(suffix)]
		}
	}
	return ""
}

func trimAnySuffix(name string, suffixes ...string) string {
	for _, suffix := range suffixes {
		name = trimKnownSuffix(name, suffix)
	}
	return name
}

func trimKnownSuffix(name, suffix string) string {
	if hasSuffixFold(name, suffix) {
		return name[:len(name)-len(suffix)]
	}
	return name
}

func hasSuffixFold(name, suffix string) bool {
	return len(name) >= len(suffix) && strings.EqualFold(name[len(name)-len(suffix):], suffix)
}

func lookupKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
