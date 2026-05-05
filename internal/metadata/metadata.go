package metadata

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/project"
)

type Index struct {
	CustomLabels          []CustomLabel          `json:"customLabels,omitempty"`
	StaticResources       []StaticResource       `json:"staticResources,omitempty"`
	NamedCredentials      []NamedCredential      `json:"namedCredentials,omitempty"`
	RemoteSites           []RemoteSite           `json:"remoteSites,omitempty"`
	CustomMetadataRecords []CustomMetadataRecord `json:"customMetadataRecords,omitempty"`

	labelsByName    map[string]int
	resourcesByName map[string]int
	endpointsByName map[string][]EndpointRef
	recordsByName   map[string]int
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

func (i *Index) sortAndBuildLookups() {
	sort.Slice(i.CustomLabels, func(a, b int) bool { return i.CustomLabels[a].Name < i.CustomLabels[b].Name })
	sort.Slice(i.StaticResources, func(a, b int) bool { return i.StaticResources[a].Name < i.StaticResources[b].Name })
	sort.Slice(i.NamedCredentials, func(a, b int) bool { return i.NamedCredentials[a].Name < i.NamedCredentials[b].Name })
	sort.Slice(i.RemoteSites, func(a, b int) bool { return i.RemoteSites[a].Name < i.RemoteSites[b].Name })
	sort.Slice(i.CustomMetadataRecords, func(a, b int) bool { return i.CustomMetadataRecords[a].FullName < i.CustomMetadataRecords[b].FullName })

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
		name := trimKnownSuffix(filepath.Base(path), ".resource")
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

func baseNoMetaExt(path string) string {
	base := filepath.Base(path)
	for _, suffix := range []string{".namedCredential-meta.xml", ".namedCredential", ".remoteSite-meta.xml", ".remoteSite"} {
		base = trimKnownSuffix(base, suffix)
	}
	return base
}

func trimKnownSuffix(name, suffix string) string {
	if strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix)) {
		return name[:len(name)-len(suffix)]
	}
	return name
}

func lookupKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
