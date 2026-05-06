package resource

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/storage"
)

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

type translationsXML struct {
	CustomLabels []translatedLabelXML `xml:"customLabels"`
}

type translatedLabelXML struct {
	Name  string `xml:"name"`
	Label string `xml:"label"`
}

type staticResourceXML struct {
	CacheControl string `xml:"cacheControl"`
	ContentType  string `xml:"contentType"`
	Description  string `xml:"description"`
}

type namedCredentialXML struct {
	FullName      string `xml:"fullName"`
	Endpoint      string `xml:"endpoint"`
	Protocol      string `xml:"protocol"`
	PrincipalType string `xml:"principalType"`
}

type remoteSiteXML struct {
	FullName    string `xml:"fullName"`
	URL         string `xml:"url"`
	Active      *bool  `xml:"isActive"`
	Description string `xml:"description"`
}

func LoadProject(p project.Project) (storage.MetadataRegistry, error) {
	var registry storage.MetadataRegistry
	for _, path := range p.LabelFiles {
		labels, err := loadLabels(path, p.Namespace)
		if err != nil {
			return storage.MetadataRegistry{}, err
		}
		registry.Labels = append(registry.Labels, labels...)
	}
	for _, path := range p.TranslationFiles {
		labels, err := loadTranslations(path, p.Namespace)
		if err != nil {
			return storage.MetadataRegistry{}, err
		}
		registry.Labels = append(registry.Labels, labels...)
	}
	resources, err := loadStaticResources(p.StaticResourceFiles, p.StaticResourceMetas)
	if err != nil {
		return storage.MetadataRegistry{}, err
	}
	registry.StaticResources = resources
	assets, err := loadContentAssets(p.ContentAssetFiles, p.ContentAssetMetas)
	if err != nil {
		return storage.MetadataRegistry{}, err
	}
	registry.ContentAssets = assets
	for _, path := range p.NamedCredentialFiles {
		endpoint, err := loadNamedCredential(path)
		if err != nil {
			return storage.MetadataRegistry{}, err
		}
		registry.Endpoints = append(registry.Endpoints, endpoint)
	}
	for _, path := range p.RemoteSiteFiles {
		endpoint, err := loadRemoteSite(path)
		if err != nil {
			return storage.MetadataRegistry{}, err
		}
		registry.Endpoints = append(registry.Endpoints, endpoint)
	}
	sortRegistry(&registry)
	return registry, nil
}

func ApplyProject(org *storage.OrgState, p project.Project) error {
	registry, err := LoadProject(p)
	if err != nil {
		return err
	}
	org.Metadata = registry
	ensureMetadataObjects(org)
	return nil
}

func StaticResourceURL(name string) string {
	return "/resource/" + strings.Trim(strings.TrimSpace(name), "/")
}

func ContentAssetURL(name string) string {
	return "/sfc/servlet.shepherd/version/download/" + strings.Trim(strings.TrimSpace(name), "/")
}

func URLForStaticResource(registry storage.MetadataRegistry, name, path string) (string, bool) {
	name = strings.TrimSpace(name)
	for _, resource := range registry.StaticResources {
		if strings.EqualFold(resource.Name, name) {
			return joinURLPath(resourceURL(resource), path), true
		}
	}
	for _, asset := range registry.ContentAssets {
		if strings.EqualFold(asset.Name, name) {
			return joinURLPath(assetURL(asset), path), true
		}
	}
	return "", false
}

func LookupLabel(registry storage.MetadataRegistry, namespace, name string) (string, bool) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" {
		for _, label := range registry.Labels {
			if strings.EqualFold(label.Name, name) {
				return label.Value, true
			}
		}
		return "", false
	}
	for i := len(registry.Labels) - 1; i >= 0; i-- {
		label := registry.Labels[i]
		if !strings.EqualFold(label.Name, name) {
			continue
		}
		if namespace != "" && label.Namespace != "" && !strings.EqualFold(label.Namespace, namespace) {
			continue
		}
		return label.Value, true
	}
	return "", false
}

func ResolveEndpoint(registry storage.MetadataRegistry, endpoint string) (string, bool) {
	if !strings.HasPrefix(strings.ToLower(endpoint), "callout:") {
		return endpoint, true
	}
	rest := strings.TrimPrefix(endpoint, "callout:")
	name, suffix, _ := strings.Cut(rest, "/")
	for _, candidate := range registry.Endpoints {
		if !strings.EqualFold(candidate.Name, name) {
			continue
		}
		base := strings.TrimRight(candidate.URL, "/")
		if suffix == "" {
			return base, true
		}
		return base + "/" + strings.TrimLeft(suffix, "/"), true
	}
	return endpoint, false
}

func loadLabels(path, namespace string) ([]storage.LabelMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw customLabelsXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	labels := make([]storage.LabelMetadata, 0, len(raw.Labels))
	for _, label := range raw.Labels {
		name := strings.TrimSpace(label.FullName)
		if name == "" {
			continue
		}
		labels = append(labels, storage.LabelMetadata{
			Name:             name,
			Namespace:        namespace,
			Language:         strings.TrimSpace(label.Language),
			Value:            strings.TrimSpace(label.Value),
			Protected:        label.Protected,
			ShortDescription: strings.TrimSpace(label.ShortDescription),
			Categories:       strings.TrimSpace(label.Categories),
			File:             path,
		})
	}
	return labels, nil
}

func loadTranslations(path, namespace string) ([]storage.LabelMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw translationsXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	language := strings.TrimSuffix(filepath.Base(path), ".translation-meta.xml")
	language = strings.TrimSuffix(language, ".translation")
	labels := make([]storage.LabelMetadata, 0, len(raw.CustomLabels))
	for _, label := range raw.CustomLabels {
		name := strings.TrimSpace(label.Name)
		if name == "" {
			continue
		}
		labels = append(labels, storage.LabelMetadata{Name: name, Namespace: namespace, Language: language, Value: strings.TrimSpace(label.Label), File: path})
	}
	return labels, nil
}

func loadStaticResources(contentFiles, metaFiles []string) ([]storage.StaticResourceMetadata, error) {
	byName := make(map[string]*storage.StaticResourceMetadata)
	for _, path := range contentFiles {
		name := trimKnownSuffix(filepath.Base(path), ".resource")
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		byName[lookupKey(name)] = &storage.StaticResourceMetadata{Name: name, ContentPath: path, Content: string(content), URL: StaticResourceURL(name)}
	}
	for _, path := range metaFiles {
		meta, err := loadResourceMeta(path)
		if err != nil {
			return nil, err
		}
		name := resourceNameFromMetaPath(path)
		resource := byName[lookupKey(name)]
		if resource == nil {
			resource = &storage.StaticResourceMetadata{Name: name, URL: StaticResourceURL(name)}
			byName[lookupKey(name)] = resource
		}
		resource.MetadataPath = path
		resource.ContentType = strings.TrimSpace(meta.ContentType)
		resource.CacheControl = strings.TrimSpace(meta.CacheControl)
		resource.Description = strings.TrimSpace(meta.Description)
	}
	out := make([]storage.StaticResourceMetadata, 0, len(byName))
	for _, resource := range byName {
		out = append(out, *resource)
	}
	return out, nil
}

func loadContentAssets(contentFiles, metaFiles []string) ([]storage.ContentAssetMetadata, error) {
	byName := make(map[string]*storage.ContentAssetMetadata)
	for _, path := range contentFiles {
		name := trimKnownSuffix(filepath.Base(path), ".asset")
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		byName[lookupKey(name)] = &storage.ContentAssetMetadata{Name: name, ContentPath: path, Content: string(content), URL: ContentAssetURL(name)}
	}
	for _, path := range metaFiles {
		meta, err := loadResourceMeta(path)
		if err != nil {
			return nil, err
		}
		name := trimKnownSuffix(filepath.Base(path), ".asset-meta.xml")
		asset := byName[lookupKey(name)]
		if asset == nil {
			asset = &storage.ContentAssetMetadata{Name: name, URL: ContentAssetURL(name)}
			byName[lookupKey(name)] = asset
		}
		asset.MetadataPath = path
		asset.ContentType = strings.TrimSpace(meta.ContentType)
		asset.Description = strings.TrimSpace(meta.Description)
	}
	out := make([]storage.ContentAssetMetadata, 0, len(byName))
	for _, asset := range byName {
		out = append(out, *asset)
	}
	return out, nil
}

func loadResourceMeta(path string) (staticResourceXML, error) {
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

func loadNamedCredential(path string) (storage.EndpointMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return storage.EndpointMetadata{}, err
	}
	var raw namedCredentialXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return storage.EndpointMetadata{}, err
	}
	name := strings.TrimSpace(raw.FullName)
	if name == "" {
		name = baseNoMetaExt(path)
	}
	return storage.EndpointMetadata{Kind: "NamedCredential", Name: name, URL: strings.TrimSpace(raw.Endpoint), Protocol: strings.TrimSpace(raw.Protocol), PrincipalType: strings.TrimSpace(raw.PrincipalType), Active: true, File: path}, nil
}

func loadRemoteSite(path string) (storage.EndpointMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return storage.EndpointMetadata{}, err
	}
	var raw remoteSiteXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return storage.EndpointMetadata{}, err
	}
	name := strings.TrimSpace(raw.FullName)
	if name == "" {
		name = baseNoMetaExt(path)
	}
	active := true
	if raw.Active != nil {
		active = *raw.Active
	}
	return storage.EndpointMetadata{Kind: "RemoteSiteSetting", Name: name, URL: strings.TrimSpace(raw.URL), Active: active, File: path}, nil
}

func ensureMetadataObjects(org *storage.OrgState) {
	if org == nil {
		return
	}
	if org.Objects == nil {
		org.Objects = make(map[string]storage.ObjectState)
	}
	ensureStaticResourceObject(org)
}

func ensureStaticResourceObject(org *storage.OrgState) {
	object := storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "StaticResource",
			Label:     "Static Resource",
			KeyPrefix: "081",
			Fields: map[string]storage.Field{
				"Name":         {APIName: "Name", Label: "Name", Type: storage.FieldString},
				"Body":         {APIName: "Body", Label: "Body", Type: storage.FieldBlob},
				"ContentType":  {APIName: "ContentType", Label: "Content Type", Type: storage.FieldString},
				"CacheControl": {APIName: "CacheControl", Label: "Cache Control", Type: storage.FieldString},
				"URL":          {APIName: "URL", Label: "URL", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	for i, resource := range org.Metadata.StaticResources {
		id := storage.ID("081" + leftPad(i+1, 12))
		object.Records[id] = storage.Record{ID: id, Object: "StaticResource", Fields: map[string]storage.Value{
			"Name":         storage.StringValue(resource.Name),
			"Body":         storage.BlobValue(resource.Content),
			"ContentType":  storage.StringValue(resource.ContentType),
			"CacheControl": storage.StringValue(resource.CacheControl),
			"URL":          storage.StringValue(resourceURL(resource)),
		}}
	}
	org.Objects["StaticResource"] = object
}

func sortRegistry(registry *storage.MetadataRegistry) {
	sort.Slice(registry.Labels, func(i, j int) bool {
		if registry.Labels[i].Name != registry.Labels[j].Name {
			return registry.Labels[i].Name < registry.Labels[j].Name
		}
		return registry.Labels[i].Language < registry.Labels[j].Language
	})
	sort.Slice(registry.StaticResources, func(i, j int) bool { return registry.StaticResources[i].Name < registry.StaticResources[j].Name })
	sort.Slice(registry.ContentAssets, func(i, j int) bool { return registry.ContentAssets[i].Name < registry.ContentAssets[j].Name })
	sort.Slice(registry.Endpoints, func(i, j int) bool {
		if registry.Endpoints[i].Name != registry.Endpoints[j].Name {
			return registry.Endpoints[i].Name < registry.Endpoints[j].Name
		}
		return registry.Endpoints[i].Kind < registry.Endpoints[j].Kind
	})
}

func resourceURL(resource storage.StaticResourceMetadata) string {
	if strings.TrimSpace(resource.URL) != "" {
		return resource.URL
	}
	return StaticResourceURL(resource.Name)
}

func assetURL(asset storage.ContentAssetMetadata) string {
	if strings.TrimSpace(asset.URL) != "" {
		return asset.URL
	}
	return ContentAssetURL(asset.Name)
}

func joinURLPath(base, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return base
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func resourceNameFromMetaPath(path string) string {
	base := filepath.Base(path)
	for _, suffix := range []string{".resource-meta.xml", ".staticresource-meta.xml"} {
		if strings.HasSuffix(strings.ToLower(base), strings.ToLower(suffix)) {
			return base[:len(base)-len(suffix)]
		}
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func baseNoMetaExt(path string) string {
	base := filepath.Base(path)
	for _, suffix := range []string{".namedCredential-meta.xml", ".namedCredential", ".remoteSite-meta.xml", ".remoteSite"} {
		if strings.HasSuffix(strings.ToLower(base), strings.ToLower(suffix)) {
			return base[:len(base)-len(suffix)]
		}
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func trimKnownSuffix(value, suffix string) string {
	if strings.HasSuffix(strings.ToLower(value), strings.ToLower(suffix)) {
		return value[:len(value)-len(suffix)]
	}
	return value
}

func lookupKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func leftPad(value, width int) string {
	text := ""
	for value > 0 {
		text = string(rune('0'+value%10)) + text
		value /= 10
	}
	for len(text) < width {
		text = "0" + text
	}
	return text
}
