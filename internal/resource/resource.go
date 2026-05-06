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

type emailTemplateXML struct {
	FullName      string `xml:"fullName"`
	Name          string `xml:"name"`
	Subject       string `xml:"subject"`
	Body          string `xml:"body"`
	HTMLValue     string `xml:"htmlValue"`
	Markup        string `xml:"markup"`
	TemplateType  string `xml:"templateType"`
	TemplateStyle string `xml:"templateStyle"`
	Encoding      string `xml:"encodingKey"`
	Description   string `xml:"description"`
	FolderName    string `xml:"folderName"`
	Active        *bool  `xml:"available"`
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
	templates, err := loadEmailTemplates(p.EmailTemplateFiles, p.Namespace)
	if err != nil {
		return storage.MetadataRegistry{}, err
	}
	registry.EmailTemplates = templates
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
		if metadataNameMatches(resource.Name, name) {
			return joinURLPath(resourceURL(resource), path), true
		}
	}
	for _, asset := range registry.ContentAssets {
		if metadataNameMatches(asset.Name, name) {
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
		if !metadataNameMatches(candidate.Name, name) {
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

func metadataNameMatches(candidate, requested string) bool {
	candidate = strings.TrimSpace(candidate)
	requested = strings.TrimSpace(requested)
	if strings.EqualFold(candidate, requested) {
		return true
	}
	strippedRequested := stripAnyNamespaceToken(requested)
	if strippedRequested != requested && strings.EqualFold(candidate, strippedRequested) {
		return true
	}
	strippedCandidate := stripAnyNamespaceToken(candidate)
	return strippedCandidate != candidate && strings.EqualFold(strippedCandidate, requested)
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

func loadEmailTemplates(paths []string, namespace string) ([]storage.EmailTemplateMetadata, error) {
	byName := make(map[string]*storage.EmailTemplateMetadata)
	for _, path := range paths {
		name := emailTemplateNameFromPath(path)
		key := lookupKey(name)
		template := byName[key]
		if template == nil {
			template = &storage.EmailTemplateMetadata{Name: name, DeveloperName: name, Namespace: namespace}
			byName[key] = template
		}
		lower := strings.ToLower(path)
		if strings.HasSuffix(lower, ".email-meta.xml") {
			meta, err := loadEmailTemplateMeta(path)
			if err != nil {
				return nil, err
			}
			template.MetadataPath = path
			template.File = firstNonEmpty(template.File, path)
			if meta.FullName != "" {
				template.Name = meta.FullName
				template.DeveloperName = developerNameFromFullName(meta.FullName)
			}
			if meta.Name != "" {
				template.Name = meta.Name
			}
			template.Subject = strings.TrimSpace(meta.Subject)
			if strings.TrimSpace(meta.Body) != "" {
				template.Body = strings.TrimSpace(meta.Body)
			}
			template.HTMLValue = strings.TrimSpace(meta.HTMLValue)
			template.Markup = strings.TrimSpace(meta.Markup)
			template.TemplateType = strings.TrimSpace(meta.TemplateType)
			template.TemplateStyle = strings.TrimSpace(meta.TemplateStyle)
			template.Encoding = strings.TrimSpace(meta.Encoding)
			template.Description = strings.TrimSpace(meta.Description)
			template.FolderName = strings.TrimSpace(meta.FolderName)
			template.Active = meta.Active == nil || *meta.Active
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		template.File = path
		template.Body = strings.TrimSpace(string(content))
		if template.Active == false {
			template.Active = true
		}
	}
	out := make([]storage.EmailTemplateMetadata, 0, len(byName))
	for _, template := range byName {
		if template.DeveloperName == "" {
			template.DeveloperName = developerNameFromFullName(template.Name)
		}
		out = append(out, *template)
	}
	return out, nil
}

func loadEmailTemplateMeta(path string) (emailTemplateXML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return emailTemplateXML{}, err
	}
	var raw emailTemplateXML
	if err := xml.Unmarshal(data, &raw); err != nil {
		return emailTemplateXML{}, err
	}
	return raw, nil
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
	if len(org.Metadata.EmailTemplates) > 0 {
		ensureEmailTemplateObject(org)
	}
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

func ensureEmailTemplateObject(org *storage.OrgState) {
	storage.EnsureStandardObject(org, "EmailTemplate")
	object := org.Objects["EmailTemplate"]
	for i, template := range org.Metadata.EmailTemplates {
		id := storage.ID("00X" + leftPad(i+1, 12))
		object.Records[id] = storage.Record{ID: id, Object: "EmailTemplate", Fields: map[string]storage.Value{
			"Id":              storage.IDValue(id),
			"Name":            storage.StringValue(firstNonEmpty(template.Name, template.DeveloperName)),
			"DeveloperName":   storage.StringValue(firstNonEmpty(template.DeveloperName, developerNameFromFullName(template.Name))),
			"NamespacePrefix": storage.StringValue(template.Namespace),
			"Subject":         storage.StringValue(template.Subject),
			"Body":            storage.StringValue(template.Body),
			"HtmlValue":       storage.StringValue(template.HTMLValue),
			"Markup":          storage.StringValue(template.Markup),
			"Description":     storage.StringValue(template.Description),
			"Encoding":        storage.StringValue(template.Encoding),
			"TemplateType":    storage.StringValue(template.TemplateType),
			"TemplateStyle":   storage.StringValue(template.TemplateStyle),
			"FolderId":        storage.StringValue(template.FolderName),
			"IsActive":        storage.BooleanValue(template.Active),
		}}
	}
	org.Objects["EmailTemplate"] = object
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
	sort.Slice(registry.EmailTemplates, func(i, j int) bool {
		return registry.EmailTemplates[i].DeveloperName < registry.EmailTemplates[j].DeveloperName
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
	for _, suffix := range []string{".namedCredential-meta.xml", ".namedCredential", ".remoteSite-meta.xml", ".remoteSite", ".email-meta.xml", ".email"} {
		if strings.HasSuffix(strings.ToLower(base), strings.ToLower(suffix)) {
			return base[:len(base)-len(suffix)]
		}
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func emailTemplateNameFromPath(path string) string {
	return baseNoMetaExt(path)
}

func developerNameFromFullName(fullName string) string {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return ""
	}
	parts := strings.Split(fullName, "/")
	return parts[len(parts)-1]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
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

func stripAnyNamespaceToken(name string) string {
	first := strings.Index(name, "__")
	if first <= 0 || first+2 >= len(name) {
		return name
	}
	return name[first+2:]
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
