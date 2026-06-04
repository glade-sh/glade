package resource

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
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

type tabXML struct {
	CustomObject bool   `xml:"customObject"`
	Description  string `xml:"description"`
	Label        string `xml:"label"`
	Motif        string `xml:"motif"`
}

type quickActionXML struct {
	Label        string `xml:"label"`
	Type         string `xml:"type"`
	TargetObject string `xml:"targetObject"`
}

type fieldSetXML struct {
	FullName        string              `xml:"fullName"`
	Label           string              `xml:"label"`
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

type folderXML struct {
	Name               string `xml:"name"`
	AccessType         string `xml:"accessType"`
	PublicFolderAccess string `xml:"publicFolderAccess"`
}

type visualforcePageXML struct {
	APIVersion                string `xml:"apiVersion"`
	AvailableInTouch          *bool  `xml:"availableInTouch"`
	ConfirmationTokenRequired *bool  `xml:"confirmationTokenRequired"`
	Description               string `xml:"description"`
	Label                     string `xml:"label"`
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
	resources, err := loadStaticResources(p.StaticResourceFiles, p.StaticResourceMetas, p.Namespace)
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
	registry.ManagedLabelNamespaces = managedLabelNamespaces(p)
	for _, path := range p.TabFiles {
		tab, err := loadTab(path)
		if err != nil {
			return storage.MetadataRegistry{}, err
		}
		registry.Tabs = append(registry.Tabs, tab)
	}
	for _, path := range p.QuickActionFiles {
		action, err := loadQuickAction(path)
		if err != nil {
			return storage.MetadataRegistry{}, err
		}
		registry.QuickActions = append(registry.QuickActions, action)
	}
	for _, path := range p.FieldSetFiles {
		fieldSet, err := loadFieldSet(path, p.Namespace)
		if err != nil {
			return storage.MetadataRegistry{}, err
		}
		registry.FieldSets = append(registry.FieldSets, fieldSet)
	}
	for _, path := range p.ObjectFiles {
		fieldSets, err := loadObjectFieldSets(path, p.Namespace)
		if err != nil {
			return storage.MetadataRegistry{}, err
		}
		registry.FieldSets = append(registry.FieldSets, fieldSets...)
	}
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
	registry, err := LoadProjectWithDependencies(p)
	if err != nil {
		return err
	}
	org.Metadata = registry
	ensureMetadataObjects(org)
	if err := ensureFolderObject(org, projectFolderFilesWithDependencies(p), p.Namespace); err != nil {
		return err
	}
	ensureApexPageObject(org, p.VisualforcePageFiles, p.Namespace)
	return nil
}

func projectFolderFilesWithDependencies(p project.Project) []string {
	files := append([]string(nil), p.FolderFiles...)
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status != "loaded" || dep.Project == nil {
			continue
		}
		files = append(files, projectFolderFilesWithDependencies(*dep.Project)...)
	}
	return files
}

func LoadProjectWithDependencies(p project.Project) (storage.MetadataRegistry, error) {
	registry, err := LoadProject(p)
	if err != nil {
		return storage.MetadataRegistry{}, err
	}
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status != "loaded" || dep.Project == nil {
			continue
		}
		depRegistry, err := LoadProjectWithDependencies(*dep.Project)
		if err != nil {
			return storage.MetadataRegistry{}, err
		}
		mergeRegistry(&registry, depRegistry)
	}
	sortRegistry(&registry)
	return registry, nil
}

func mergeRegistry(dst *storage.MetadataRegistry, src storage.MetadataRegistry) {
	if dst == nil {
		return
	}
	dst.Labels = append(dst.Labels, src.Labels...)
	dst.ManagedLabelNamespaces = append(dst.ManagedLabelNamespaces, src.ManagedLabelNamespaces...)
	dst.DataCategoryGroups = append(dst.DataCategoryGroups, src.DataCategoryGroups...)
	dst.StaticResources = append(dst.StaticResources, src.StaticResources...)
	dst.ContentAssets = append(dst.ContentAssets, src.ContentAssets...)
	dst.EmailTemplates = append(dst.EmailTemplates, src.EmailTemplates...)
	dst.Tabs = append(dst.Tabs, src.Tabs...)
	dst.QuickActions = append(dst.QuickActions, src.QuickActions...)
	dst.FieldSets = append(dst.FieldSets, src.FieldSets...)
	dst.Endpoints = append(dst.Endpoints, src.Endpoints...)
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

func loadTab(path string) (storage.TabMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return storage.TabMetadata{}, err
	}
	var raw tabXML
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := xml.Unmarshal(data, &raw); err != nil {
			return storage.TabMetadata{}, err
		}
	}
	name := metadataNameFromPath(path, ".tab-meta.xml", ".tab")
	label := strings.TrimSpace(raw.Label)
	if label == "" {
		label = name
	}
	tab := storage.TabMetadata{
		Name:        name,
		Label:       label,
		Custom:      raw.CustomObject,
		Motif:       strings.TrimSpace(raw.Motif),
		Description: strings.TrimSpace(raw.Description),
		File:        path,
	}
	if raw.CustomObject || hasSuffixFold(name, "__c") {
		tab.SObjectName = name
	}
	return tab, nil
}

func loadQuickAction(path string) (storage.QuickActionMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return storage.QuickActionMetadata{}, err
	}
	var raw quickActionXML
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := xml.Unmarshal(data, &raw); err != nil {
			return storage.QuickActionMetadata{}, err
		}
	}
	name := metadataNameFromPath(path, ".quickAction-meta.xml", ".quickaction-meta.xml", ".quickAction", ".quickaction")
	targetObject := strings.TrimSpace(raw.TargetObject)
	if targetObject == "" {
		if dot := strings.IndexByte(name, '.'); dot > 0 {
			targetObject = name[:dot]
		}
	}
	label := strings.TrimSpace(raw.Label)
	if label == "" {
		label = name
	}
	return storage.QuickActionMetadata{
		Name:         name,
		Label:        label,
		Type:         strings.TrimSpace(raw.Type),
		TargetObject: targetObject,
		File:         path,
	}, nil
}

func loadFieldSet(path, namespace string) (storage.FieldSetMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return storage.FieldSetMetadata{}, err
	}
	var raw fieldSetXML
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := xml.Unmarshal(data, &raw); err != nil {
			return storage.FieldSetMetadata{}, err
		}
	}
	return fieldSetFromXML(raw, objectNameFromFieldSetPath(path), metadataNameFromPath(path, ".fieldSet-meta.xml", ".fieldSet"), namespace, path), nil
}

func loadObjectFieldSets(path, namespace string) ([]storage.FieldSetMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw objectFieldSetsXML
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := xml.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
	}
	out := make([]storage.FieldSetMetadata, 0, len(raw.FieldSets))
	objectName := objectNameFromObjectPath(path)
	for _, rawFieldSet := range raw.FieldSets {
		fieldSet := fieldSetFromXML(rawFieldSet, objectName, "", namespace, path)
		if fieldSet.Name == "" {
			continue
		}
		out = append(out, fieldSet)
	}
	return out, nil
}

func fieldSetFromXML(raw fieldSetXML, objectName, fallbackName, namespace, path string) storage.FieldSetMetadata {
	name := strings.TrimSpace(raw.FullName)
	if name == "" {
		name = fallbackName
	}
	members := make([]storage.FieldSetMemberMetadata, 0, len(raw.DisplayedFields)+len(raw.AvailableFields))
	for _, member := range append(raw.DisplayedFields, raw.AvailableFields...) {
		field := strings.TrimSpace(member.Field)
		if field == "" {
			continue
		}
		members = append(members, storage.FieldSetMemberMetadata{Field: field, Required: member.Required})
	}
	return storage.FieldSetMetadata{
		ObjectName: objectName,
		Namespace:  strings.TrimSpace(namespace),
		Name:       name,
		Label:      strings.TrimSpace(raw.Label),
		Fields:     members,
		File:       path,
	}
}

func objectNameFromFieldSetPath(path string) string {
	dir := filepath.Dir(path)
	if filepath.Base(dir) != "fieldSets" {
		return ""
	}
	objectDir := filepath.Dir(dir)
	if filepath.Base(filepath.Dir(objectDir)) != "objects" {
		return ""
	}
	return filepath.Base(objectDir)
}

func objectNameFromObjectPath(path string) string {
	base := filepath.Base(path)
	for _, suffix := range []string{".object-meta.xml", ".object"} {
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix)
		}
	}
	return ""
}

func metadataNameFromPath(path string, suffixes ...string) string {
	base := filepath.Base(path)
	lower := strings.ToLower(base)
	for _, suffix := range suffixes {
		if strings.HasSuffix(lower, strings.ToLower(suffix)) {
			return base[:len(base)-len(suffix)]
		}
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func LookupLabel(registry storage.MetadataRegistry, namespace, name string) (string, bool) {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" {
		for _, label := range registry.Labels {
			if labelNameMatches(label.Name, name) {
				return label.Value, true
			}
		}
		return "", false
	}
	for i := len(registry.Labels) - 1; i >= 0; i-- {
		label := registry.Labels[i]
		if !labelNameMatches(label.Name, name) {
			continue
		}
		if namespace != "" && label.Namespace != "" && !strings.EqualFold(label.Namespace, namespace) {
			continue
		}
		return label.Value, true
	}
	return "", false
}

type LabelLookupStatus string

const (
	LabelLookupMissing                  LabelLookupStatus = "missing"
	LabelLookupPlatformFallback         LabelLookupStatus = "platform-label-fallback"
	LabelLookupManagedNamespaceFallback LabelLookupStatus = "managed-namespace-fallback"
	LabelLookupResolved                 LabelLookupStatus = "resolved"
)

func ResolveLabel(registry storage.MetadataRegistry, orgNamespace, namespace, name string) (string, LabelLookupStatus) {
	if value, ok := LookupLabel(registry, namespace, name); ok {
		return value, LabelLookupResolved
	}
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" || name == "" {
		return "", LabelLookupMissing
	}
	if isPlatformLabelNamespace(namespace) {
		return name, LabelLookupPlatformFallback
	}
	if orgNamespace != "" && strings.EqualFold(namespace, orgNamespace) {
		return "", LabelLookupMissing
	}
	for _, managed := range registry.ManagedLabelNamespaces {
		if strings.EqualFold(strings.TrimSpace(managed), namespace) {
			return name, LabelLookupManagedNamespaceFallback
		}
	}
	return "", LabelLookupMissing
}

func isPlatformLabelNamespace(namespace string) bool {
	switch strings.ToLower(strings.TrimSpace(namespace)) {
	case "site":
		return true
	default:
		return false
	}
}

func labelNameMatches(candidate, requested string) bool {
	candidate = strings.TrimSpace(candidate)
	requested = strings.TrimSpace(requested)
	if strings.EqualFold(candidate, requested) {
		return true
	}
	strippedCandidate := stripAnyNamespaceToken(candidate)
	if strippedCandidate != candidate && strings.EqualFold(strippedCandidate, requested) {
		return true
	}
	strippedRequested := stripAnyNamespaceToken(requested)
	return strippedRequested != requested && strings.EqualFold(candidate, strippedRequested)
}

func managedLabelNamespaces(p project.Project) []string {
	aliases := make(map[string]bool)
	add := func(value string) {
		token := namespaceToken(value)
		if token == "" {
			return
		}
		if p.Namespace != "" && strings.EqualFold(token, p.Namespace) {
			return
		}
		aliases[strings.ToLower(token)] = true
	}
	for _, paths := range [][]string{
		p.ObjectFiles,
		p.FieldFiles,
		p.FieldSetFiles,
		p.RecordTypeFiles,
		p.ValidationRuleFiles,
		p.CustomMetadataFiles,
		p.LayoutFiles,
		p.CompactLayoutFiles,
		p.TabFiles,
		p.WebLinkFiles,
		p.QuickActionFiles,
		p.ApexFiles,
		p.VisualforcePageFiles,
		p.VisualforceComponentFiles,
		p.AuraFiles,
		p.LWCFiles,
	} {
		for _, path := range paths {
			add(filepath.Base(path))
			for _, part := range strings.Split(filepath.ToSlash(path), "/") {
				add(part)
			}
		}
	}
	if len(aliases) == 0 {
		return nil
	}
	out := make([]string, 0, len(aliases))
	for alias := range aliases {
		out = append(out, alias)
	}
	sort.Strings(out)
	return out
}

func namespaceToken(name string) string {
	name = strings.TrimSpace(name)
	first := strings.Index(name, "__")
	last := strings.LastIndex(name, "__")
	if first <= 0 || first >= last {
		return ""
	}
	token := name[:first]
	if strings.Contains(token, ".") {
		token = token[strings.LastIndex(token, ".")+1:]
	}
	return token
}

func ResolveEndpoint(registry storage.MetadataRegistry, endpoint string) (string, bool) {
	if !hasPrefixFold(endpoint, "callout:") {
		return endpoint, true
	}
	rest := endpoint[len("callout:"):]
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

func loadStaticResources(contentFiles, metaFiles []string, namespace string) ([]storage.StaticResourceMetadata, error) {
	byName := make(map[string]*storage.StaticResourceMetadata)
	namespace = strings.TrimSpace(namespace)
	for _, path := range contentFiles {
		name := resourceNameFromContentPath(path)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		byName[lookupKey(name)] = &storage.StaticResourceMetadata{Name: name, NamespacePrefix: namespace, ContentPath: path, Content: string(content), URL: StaticResourceURL(name)}
	}
	for _, path := range metaFiles {
		meta, err := loadResourceMeta(path)
		if err != nil {
			return nil, err
		}
		name := resourceNameFromMetaPath(path)
		resource := byName[lookupKey(name)]
		if resource == nil {
			resource = &storage.StaticResourceMetadata{Name: name, NamespacePrefix: namespace, URL: StaticResourceURL(name)}
			byName[lookupKey(name)] = resource
		}
		resource.NamespacePrefix = namespace
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

func ensureApexPageObject(org *storage.OrgState, pageFiles []string, namespace string) {
	if org == nil || len(pageFiles) == 0 {
		return
	}
	object := storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "ApexPage",
			Label:     "Apex Page",
			KeyPrefix: "066",
			Fields: map[string]storage.Field{
				"Id":                          {APIName: "Id", Label: "Record ID", Type: storage.FieldID},
				"Name":                        {APIName: "Name", Label: "Name", Type: storage.FieldString},
				"NamespacePrefix":             {APIName: "NamespacePrefix", Label: "Namespace Prefix", Type: storage.FieldString},
				"ApiVersion":                  {APIName: "ApiVersion", Label: "API Version", Type: storage.FieldDecimal},
				"ControllerKey":               {APIName: "ControllerKey", Label: "Controller Key", Type: storage.FieldString},
				"ControllerType":              {APIName: "ControllerType", Label: "Controller Type", Type: storage.FieldString},
				"Description":                 {APIName: "Description", Label: "Description", Type: storage.FieldString},
				"IsAvailableInTouch":          {APIName: "IsAvailableInTouch", Label: "Available in Touch", Type: storage.FieldBoolean},
				"IsConfirmationTokenRequired": {APIName: "IsConfirmationTokenRequired", Label: "Confirmation Token Required", Type: storage.FieldBoolean},
				"Markup":                      {APIName: "Markup", Label: "Markup", Type: storage.FieldString},
				"MasterLabel":                 {APIName: "MasterLabel", Label: "Master Label", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	paths := append([]string(nil), pageFiles...)
	sort.Strings(paths)
	for i, path := range paths {
		name := visualforcePageName(path)
		if name == "" {
			continue
		}
		meta := loadVisualforcePageMeta(path + "-meta.xml")
		markup := ""
		if data, err := os.ReadFile(path); err == nil {
			markup = string(data)
		}
		id := storage.ID("066" + leftPad(i+1, 12))
		object.Records[id] = storage.Record{ID: id, Object: "ApexPage", Fields: map[string]storage.Value{
			"Id":                          storage.IDValue(id),
			"Name":                        storage.StringValue(name),
			"NamespacePrefix":             storage.StringValue(namespace),
			"ApiVersion":                  visualforcePageAPIVersion(meta.APIVersion),
			"ControllerKey":               storage.NullValue(),
			"ControllerType":              storage.NullValue(),
			"Description":                 storage.StringValue(meta.Description),
			"IsAvailableInTouch":          storage.BooleanValue(meta.AvailableInTouch != nil && *meta.AvailableInTouch),
			"IsConfirmationTokenRequired": storage.BooleanValue(meta.ConfirmationTokenRequired != nil && *meta.ConfirmationTokenRequired),
			"Markup":                      storage.StringValue(markup),
			"MasterLabel":                 storage.StringValue(firstNonEmpty(meta.Label, name)),
		}}
	}
	org.Objects["ApexPage"] = object
}

func ensureStaticResourceObject(org *storage.OrgState) {
	object := storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "StaticResource",
			Label:     "Static Resource",
			KeyPrefix: "081",
			Fields: map[string]storage.Field{
				"Name":            {APIName: "Name", Label: "Name", Type: storage.FieldString},
				"Body":            {APIName: "Body", Label: "Body", Type: storage.FieldBlob},
				"ContentType":     {APIName: "ContentType", Label: "Content Type", Type: storage.FieldString},
				"CacheControl":    {APIName: "CacheControl", Label: "Cache Control", Type: storage.FieldString},
				"NamespacePrefix": {APIName: "NamespacePrefix", Label: "Namespace Prefix", Type: storage.FieldString},
				"SystemModStamp":  {APIName: "SystemModStamp", Label: "System Modstamp", Type: storage.FieldDateTime},
				"URL":             {APIName: "URL", Label: "URL", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	for i, resource := range org.Metadata.StaticResources {
		id := storage.ID("081" + leftPad(i+1, 12))
		object.Records[id] = storage.Record{ID: id, Object: "StaticResource", Fields: map[string]storage.Value{
			"Name":            storage.StringValue(resource.Name),
			"Body":            storage.BlobValue(resource.Content),
			"ContentType":     storage.StringValue(resource.ContentType),
			"CacheControl":    storage.StringValue(resource.CacheControl),
			"NamespacePrefix": staticResourceNamespaceValue(resource.NamespacePrefix),
			"SystemModStamp":  storage.DateTimeValue("2026-01-01T00:00:00Z"),
			"URL":             storage.StringValue(resourceURL(resource)),
		}}
	}
	org.Objects["StaticResource"] = object
}

func staticResourceNamespaceValue(namespace string) storage.Value {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return storage.NullValue()
	}
	return storage.StringValue(namespace)
}

func ensureEmailTemplateObject(org *storage.OrgState) {
	storage.EnsureStandardObject(org, "EmailTemplate")
	object := org.Objects["EmailTemplate"]
	for i, template := range org.Metadata.EmailTemplates {
		id := storage.ID("00X" + leftPad(i+100001, 12))
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

func ensureFolderObject(org *storage.OrgState, folderFiles []string, namespace string) error {
	if org == nil || len(folderFiles) == 0 {
		return nil
	}
	if org.Objects == nil {
		org.Objects = make(map[string]storage.ObjectState)
	}
	object := org.Objects["Folder"]
	if object.Definition.APIName == "" {
		object.Definition.APIName = "Folder"
	}
	if object.Definition.Label == "" {
		object.Definition.Label = "Folder"
	}
	if object.Definition.PluralLabel == "" {
		object.Definition.PluralLabel = "Folders"
	}
	if object.Definition.KeyPrefix == "" {
		object.Definition.KeyPrefix = "00l"
	}
	if object.Records == nil {
		object.Records = make(map[storage.ID]storage.Record)
	}
	ensureFolderFields(&object.Definition)

	paths := append([]string(nil), folderFiles...)
	sort.Strings(paths)
	for i, path := range paths {
		meta, err := loadFolderMeta(path)
		if err != nil {
			return err
		}
		developerName := folderDeveloperName(path)
		name := firstNonEmpty(meta.Name, developerName)
		if name == "" {
			continue
		}
		namespaceValue := storage.NullValue()
		if strings.TrimSpace(namespace) != "" {
			namespaceValue = storage.StringValue(strings.TrimSpace(namespace))
		}
		id := storage.ID("00l" + leftPad(i+1, 12))
		object.Records[id] = storage.Record{ID: id, Object: "Folder", Fields: map[string]storage.Value{
			"Id":                 storage.IDValue(id),
			"Name":               storage.StringValue(name),
			"DeveloperName":      storage.StringValue(developerName),
			"Type":               storage.StringValue(folderTypeFromPath(path)),
			"NamespacePrefix":    namespaceValue,
			"AccessType":         storage.StringValue(strings.TrimSpace(meta.AccessType)),
			"PublicFolderAccess": storage.StringValue(strings.TrimSpace(meta.PublicFolderAccess)),
		}}
	}
	org.Objects["Folder"] = object
	return nil
}

func ensureFolderFields(definition *storage.ObjectDefinition) {
	if definition.Fields == nil {
		definition.Fields = make(map[string]storage.Field)
	}
	for _, field := range []storage.Field{
		{APIName: "Id", Label: "Record ID", Type: storage.FieldID},
		{APIName: "Name", Label: "Name", Type: storage.FieldString},
		{APIName: "DeveloperName", Label: "Developer Name", Type: storage.FieldString},
		{APIName: "Type", Label: "Type", Type: storage.FieldString},
		{APIName: "NamespacePrefix", Label: "Namespace Prefix", Type: storage.FieldString},
		{APIName: "AccessType", Label: "Access Type", Type: storage.FieldString},
		{APIName: "PublicFolderAccess", Label: "Public Folder Access", Type: storage.FieldString},
	} {
		if _, ok := definition.Fields[field.APIName]; !ok {
			definition.Fields[field.APIName] = field
		}
	}
}

func loadFolderMeta(path string) (folderXML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return folderXML{}, err
	}
	var raw folderXML
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := xml.Unmarshal(data, &raw); err != nil {
			return folderXML{}, err
		}
	}
	return raw, nil
}

func folderDeveloperName(path string) string {
	return metadataNameFromPath(path,
		".documentFolder-meta.xml",
		".emailFolder-meta.xml",
		".reportFolder-meta.xml",
		".dashboardFolder-meta.xml",
	)
}

func folderTypeFromPath(path string) string {
	lower := strings.ToLower(filepath.Base(path))
	switch {
	case strings.HasSuffix(lower, ".documentfolder-meta.xml"):
		return "Document"
	case strings.HasSuffix(lower, ".emailfolder-meta.xml"):
		return "Email"
	case strings.HasSuffix(lower, ".reportfolder-meta.xml"):
		return "Report"
	case strings.HasSuffix(lower, ".dashboardfolder-meta.xml"):
		return "Dashboard"
	default:
		return ""
	}
}

func visualforcePageName(path string) string {
	base := filepath.Base(path)
	return trimKnownSuffix(base, ".page")
}

func loadVisualforcePageMeta(path string) visualforcePageXML {
	var page visualforcePageXML
	data, err := os.ReadFile(path)
	if err != nil {
		return page
	}
	_ = xml.Unmarshal(data, &page)
	return page
}

func visualforcePageAPIVersion(raw string) storage.Value {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return storage.NullValue()
	}
	return storage.DecimalValue(raw)
}

func sortRegistry(registry *storage.MetadataRegistry) {
	sort.Slice(registry.Labels, func(i, j int) bool {
		if registry.Labels[i].Name != registry.Labels[j].Name {
			return registry.Labels[i].Name < registry.Labels[j].Name
		}
		return registry.Labels[i].Language < registry.Labels[j].Language
	})
	sort.Slice(registry.Tabs, func(i, j int) bool { return registry.Tabs[i].Name < registry.Tabs[j].Name })
	sort.Slice(registry.QuickActions, func(i, j int) bool { return registry.QuickActions[i].Name < registry.QuickActions[j].Name })
	sort.Slice(registry.FieldSets, func(i, j int) bool {
		if registry.FieldSets[i].ObjectName != registry.FieldSets[j].ObjectName {
			return registry.FieldSets[i].ObjectName < registry.FieldSets[j].ObjectName
		}
		return registry.FieldSets[i].Name < registry.FieldSets[j].Name
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
		if hasSuffixFold(base, suffix) {
			return base[:len(base)-len(suffix)]
		}
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
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
	for _, suffix := range []string{".namedCredential-meta.xml", ".namedCredential", ".remoteSite-meta.xml", ".remoteSite", ".email-meta.xml", ".email"} {
		if hasSuffixFold(base, suffix) {
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
	if hasSuffixFold(value, suffix) {
		return value[:len(value)-len(suffix)]
	}
	return value
}

func hasPrefixFold(value, prefix string) bool {
	return len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix)
}

func hasSuffixFold(value, suffix string) bool {
	return len(value) >= len(suffix) && strings.EqualFold(value[len(value)-len(suffix):], suffix)
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
