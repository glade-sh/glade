package packageartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
)

const CurrentSchemaVersion = 2

type CaptureProvenance struct {
	Source      string    `json:"source,omitempty"`
	OrgID       string    `json:"orgId,omitempty"`
	Username    string    `json:"username,omitempty"`
	TargetOrg   string    `json:"targetOrg,omitempty"`
	APIVersion  string    `json:"apiVersion,omitempty"`
	CapturedAt  time.Time `json:"capturedAt,omitempty"`
	PackageID   string    `json:"packageId,omitempty"`
	InstalledID string    `json:"installedId,omitempty"`
}

type LightningBundle struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Exposed   bool   `json:"exposed,omitempty"`
}

func (b LightningBundle) QualifiedName() string {
	if strings.TrimSpace(b.Namespace) == "" {
		return strings.TrimSpace(b.Name)
	}
	return strings.TrimSpace(b.Namespace) + "/" + strings.TrimSpace(b.Name)
}

type BuildCapturedOptions struct {
	Namespace             string
	PackageName           string
	Version               string
	SourceAPIVersion      string
	Capture               CaptureProvenance
	ApexTypes             []ApexType
	Objects               []schema.Object
	CustomMetadataRecords []schema.CustomMetadataRecord
	LabelNames            []string
	StaticResourceNames   []string
	LightningBundles      []LightningBundle
}

type Artifact struct {
	SchemaVersion           int                           `json:"schemaVersion,omitempty"`
	Namespace               string                        `json:"namespace"`
	PackageName             string                        `json:"packageName,omitempty"`
	Version                 string                        `json:"version,omitempty"`
	SourceRoot              string                        `json:"sourceRoot,omitempty"`
	SourceHash              string                        `json:"sourceHash,omitempty"`
	SourceAPIVersion        string                        `json:"sourceApiVersion,omitempty"`
	BuiltAt                 time.Time                     `json:"builtAt"`
	Capture                 CaptureProvenance             `json:"capture,omitempty"`
	ApexTypes               []ApexType                    `json:"apexTypes,omitempty"`
	Objects                 []schema.Object               `json:"objects,omitempty"`
	CustomMetadataRecords   []schema.CustomMetadataRecord `json:"customMetadataRecords,omitempty"`
	LabelNames              []string                      `json:"labelNames,omitempty"`
	StaticResourceNames     []string                      `json:"staticResourceNames,omitempty"`
	LightningBundles        []LightningBundle             `json:"lightningBundles,omitempty"`
	Labels                  int                           `json:"labels"`
	StaticResources         int                           `json:"staticResources"`
	CodeIntelSymbolsVersion int                           `json:"codeIntelSymbolsVersion,omitempty"`
	CodeIntelSymbols        []CodeIntelSymbol             `json:"codeIntelSymbols,omitempty"`
	CodeIntelUsesVersion    int                           `json:"codeIntelUsesVersion,omitempty"`
	CodeIntelUses           []CodeIntelUse                `json:"codeIntelUses,omitempty"`
}

type ApexType struct {
	Kind       apexast.DeclarationKind `json:"kind"`
	Name       string                  `json:"name"`
	File       string                  `json:"file"`
	Namespace  string                  `json:"namespace,omitempty"`
	SourceRoot string                  `json:"sourceRoot,omitempty"`
	Version    string                  `json:"version,omitempty"`
	Dependency bool                    `json:"dependency,omitempty"`
	Modifiers  []string                `json:"modifiers,omitempty"`
	IsTest     bool                    `json:"isTest,omitempty"`
	SuperClass string                  `json:"superClass,omitempty"`
	Interfaces []string                `json:"interfaces,omitempty"`
	Range      diagnostic.Range        `json:"range"`
	Members    []ApexMember            `json:"members,omitempty"`
}

type ApexMember struct {
	Kind       apexast.DeclarationKind `json:"kind"`
	Name       string                  `json:"name"`
	Type       string                  `json:"type,omitempty"`
	Modifiers  []string                `json:"modifiers,omitempty"`
	Parameters []apexast.Parameter     `json:"parameters,omitempty"`
	Accessors  []apexast.Accessor      `json:"accessors,omitempty"`
	IsTest     bool                    `json:"isTest,omitempty"`
	Range      diagnostic.Range        `json:"range"`
}

type CodeIntelSymbol struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Container  string            `json:"container,omitempty"`
	Namespace  string            `json:"namespace,omitempty"`
	Type       string            `json:"type,omitempty"`
	Signature  string            `json:"signature,omitempty"`
	File       string            `json:"file,omitempty"`
	Range      diagnostic.Range  `json:"range"`
	Dependency bool              `json:"dependency,omitempty"`
	Artifact   bool              `json:"artifact,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type CodeIntelUse struct {
	SymbolID string            `json:"symbolId,omitempty"`
	Kind     string            `json:"kind"`
	Name     string            `json:"name"`
	File     string            `json:"file"`
	Range    diagnostic.Range  `json:"range"`
	Context  string            `json:"context,omitempty"`
	Resolved bool              `json:"resolved"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Summary struct {
	Namespace       string `json:"namespace"`
	SourceRoot      string `json:"sourceRoot"`
	Version         string `json:"version,omitempty"`
	Status          string `json:"status"`
	ApexTypes       int    `json:"apexTypes"`
	Objects         int    `json:"objects"`
	Labels          int    `json:"labels"`
	StaticResources int    `json:"staticResources"`
}

type Info struct {
	SchemaVersion         int               `json:"schemaVersion,omitempty"`
	Namespace             string            `json:"namespace"`
	PackageName           string            `json:"packageName,omitempty"`
	Version               string            `json:"version,omitempty"`
	SourceRoot            string            `json:"sourceRoot,omitempty"`
	SourceHash            string            `json:"sourceHash,omitempty"`
	SourceAPIVersion      string            `json:"sourceApiVersion,omitempty"`
	BuiltAt               time.Time         `json:"builtAt"`
	Capture               CaptureProvenance `json:"capture,omitempty"`
	ApexTypes             int               `json:"apexTypes"`
	Objects               int               `json:"objects"`
	CustomMetadataRecords int               `json:"customMetadataRecords"`
	Labels                int               `json:"labels"`
	StaticResources       int               `json:"staticResources"`
}

type Diff struct {
	Changed               bool     `json:"changed"`
	FromNamespace         string   `json:"fromNamespace,omitempty"`
	ToNamespace           string   `json:"toNamespace,omitempty"`
	FromVersion           string   `json:"fromVersion,omitempty"`
	ToVersion             string   `json:"toVersion,omitempty"`
	AddedTypes            int      `json:"addedTypes"`
	RemovedTypes          int      `json:"removedTypes"`
	ChangedTypes          int      `json:"changedTypes"`
	AddedObjects          int      `json:"addedObjects"`
	RemovedObjects        int      `json:"removedObjects"`
	ChangedObjects        int      `json:"changedObjects"`
	AddedTypeNames        []string `json:"addedTypeNames,omitempty"`
	RemovedTypeNames      []string `json:"removedTypeNames,omitempty"`
	ChangedTypeNames      []string `json:"changedTypeNames,omitempty"`
	AddedObjectNames      []string `json:"addedObjectNames,omitempty"`
	RemovedObjectNames    []string `json:"removedObjectNames,omitempty"`
	ChangedObjectNames    []string `json:"changedObjectNames,omitempty"`
	SourceHashChanged     bool     `json:"sourceHashChanged"`
	APIVersionChanged     bool     `json:"apiVersionChanged"`
	SchemaResourceChanged bool     `json:"schemaResourceChanged"`
}

func Build(namespace, version string, p project.Project, s schema.Schema, apexTypes []ApexType) (Artifact, error) {
	sourceHash, err := SourceHash(p)
	if err != nil {
		return Artifact{}, err
	}
	labels := packageLabelNames(namespace, p.LabelFiles)
	resources := packageStaticResourceNames(namespace, p.StaticResourceFiles, p.StaticResourceMetas)
	artifact := Artifact{
		SchemaVersion:         CurrentSchemaVersion,
		Namespace:             namespace,
		Version:               version,
		SourceRoot:            p.Root,
		SourceHash:            sourceHash,
		SourceAPIVersion:      p.SourceAPIVersion,
		BuiltAt:               time.Now().UTC(),
		ApexTypes:             globalContractTypes(apexTypes),
		Objects:               namespaceObjects(namespace, s.Objects),
		CustomMetadataRecords: namespaceCustomMetadataRecords(namespace, s.CustomMetadataRecords),
		LabelNames:            labels,
		StaticResourceNames:   resources,
		Labels:                len(labels),
		StaticResources:       len(resources),
	}
	artifact.CodeIntelSymbols = codeIntelSymbols(artifact, p)
	artifact.CodeIntelUses = codeIntelDeclarationUses(artifact.CodeIntelSymbols)
	if len(artifact.CodeIntelSymbols) > 0 {
		artifact.CodeIntelSymbolsVersion = 1
	}
	if len(artifact.CodeIntelUses) > 0 {
		artifact.CodeIntelUsesVersion = 1
	}
	return artifact, nil
}

func BuildCaptured(opts BuildCapturedOptions) (Artifact, error) {
	namespace := strings.TrimSpace(opts.Namespace)
	if namespace == "" {
		return Artifact{}, errors.New("namespace is required")
	}
	builtAt := time.Now().UTC()
	if !opts.Capture.CapturedAt.IsZero() {
		builtAt = opts.Capture.CapturedAt.UTC()
	}
	artifact := Artifact{
		SchemaVersion:         CurrentSchemaVersion,
		Namespace:             namespace,
		PackageName:           strings.TrimSpace(opts.PackageName),
		Version:               strings.TrimSpace(opts.Version),
		SourceAPIVersion:      strings.TrimSpace(opts.SourceAPIVersion),
		BuiltAt:               builtAt,
		Capture:               opts.Capture,
		ApexTypes:             sortedApexTypes(globalContractTypes(cloneApexTypes(opts.ApexTypes))),
		Objects:               sortedSchemaObjects(cloneSchemaObjects(opts.Objects)),
		CustomMetadataRecords: sortedCustomMetadataRecords(cloneCustomMetadataRecords(opts.CustomMetadataRecords)),
		LabelNames:            sortedUniqueStrings(opts.LabelNames),
		StaticResourceNames:   sortedUniqueStrings(opts.StaticResourceNames),
		LightningBundles:      sortedLightningBundles(opts.LightningBundles),
	}
	artifact.Labels = len(artifact.LabelNames)
	artifact.StaticResources = len(artifact.StaticResourceNames)
	artifact.SourceHash = capturedSourceHash(artifact)
	artifact.CodeIntelSymbols = codeIntelSymbols(artifact, project.Project{})
	artifact.CodeIntelUses = codeIntelDeclarationUses(artifact.CodeIntelSymbols)
	if len(artifact.CodeIntelSymbols) > 0 {
		artifact.CodeIntelSymbolsVersion = 1
	}
	if len(artifact.CodeIntelUses) > 0 {
		artifact.CodeIntelUsesVersion = 1
	}
	return artifact, nil
}

func Inspect(artifact Artifact) Info {
	return Info{
		SchemaVersion:         artifact.SchemaVersion,
		Namespace:             artifact.Namespace,
		PackageName:           artifact.PackageName,
		Version:               artifact.Version,
		SourceRoot:            artifact.SourceRoot,
		SourceHash:            artifact.SourceHash,
		SourceAPIVersion:      artifact.SourceAPIVersion,
		BuiltAt:               artifact.BuiltAt,
		Capture:               artifact.Capture,
		ApexTypes:             len(artifact.ApexTypes),
		Objects:               len(artifact.Objects),
		CustomMetadataRecords: len(artifact.CustomMetadataRecords),
		Labels:                artifact.Labels,
		StaticResources:       artifact.StaticResources,
	}
}

func Validate(artifact Artifact) []string {
	issues := make([]string, 0)
	schemaVersion := artifact.SchemaVersion
	if schemaVersion == 0 {
		schemaVersion = 1
	}
	if schemaVersion > CurrentSchemaVersion {
		issues = append(issues, fmt.Sprintf("unsupported artifact schemaVersion %d", artifact.SchemaVersion))
	}
	if strings.TrimSpace(artifact.Namespace) == "" {
		issues = append(issues, "namespace is required")
	}
	if strings.TrimSpace(artifact.SourceHash) == "" {
		issues = append(issues, "sourceHash is required")
	}
	for _, typ := range artifact.ApexTypes {
		if strings.TrimSpace(typ.Name) == "" {
			issues = append(issues, "apex type name is required")
		}
		if strings.TrimSpace(typ.Namespace) == "" {
			issues = append(issues, "apex type "+typ.Name+" is missing namespace")
		}
	}
	for _, object := range artifact.Objects {
		if strings.TrimSpace(object.Name) == "" {
			issues = append(issues, "object name is required")
		}
	}
	return issues
}

func Compare(from, to Artifact) Diff {
	diff := Diff{
		FromNamespace:         from.Namespace,
		ToNamespace:           to.Namespace,
		FromVersion:           from.Version,
		ToVersion:             to.Version,
		SourceHashChanged:     from.SourceHash != to.SourceHash,
		APIVersionChanged:     from.SourceAPIVersion != to.SourceAPIVersion,
		SchemaResourceChanged: len(from.CustomMetadataRecords) != len(to.CustomMetadataRecords) || from.Labels != to.Labels || from.StaticResources != to.StaticResources,
	}
	fromTypes := apexTypeFingerprints(from.ApexTypes)
	toTypes := apexTypeFingerprints(to.ApexTypes)
	diff.AddedTypeNames, diff.RemovedTypeNames, diff.ChangedTypeNames = compareFingerprints(fromTypes, toTypes)
	fromObjects := objectFingerprints(from.Objects)
	toObjects := objectFingerprints(to.Objects)
	diff.AddedObjectNames, diff.RemovedObjectNames, diff.ChangedObjectNames = compareFingerprints(fromObjects, toObjects)
	diff.AddedTypes = len(diff.AddedTypeNames)
	diff.RemovedTypes = len(diff.RemovedTypeNames)
	diff.ChangedTypes = len(diff.ChangedTypeNames)
	diff.AddedObjects = len(diff.AddedObjectNames)
	diff.RemovedObjects = len(diff.RemovedObjectNames)
	diff.ChangedObjects = len(diff.ChangedObjectNames)
	diff.Changed = diff.SourceHashChanged || diff.APIVersionChanged || diff.SchemaResourceChanged || diff.AddedTypes > 0 || diff.RemovedTypes > 0 || diff.ChangedTypes > 0 || diff.AddedObjects > 0 || diff.RemovedObjects > 0 || diff.ChangedObjects > 0
	return diff
}

func Summarize(dep project.ManagedPackageDependency, artifact Artifact) Summary {
	return Summary{
		Namespace:       dep.Namespace,
		SourceRoot:      dep.SourceRoot,
		Version:         dep.Version,
		Status:          dep.Status,
		ApexTypes:       len(artifact.ApexTypes),
		Objects:         len(artifact.Objects),
		Labels:          artifact.Labels,
		StaticResources: artifact.StaticResources,
	}
}

func WriteJSON(path string, artifact Artifact) error {
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func ReadJSON(path string) (Artifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Artifact{}, err
	}
	var artifact Artifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return Artifact{}, err
	}
	return artifact, nil
}

func apexTypeFingerprints(types []ApexType) map[string]string {
	out := make(map[string]string, len(types))
	for _, typ := range types {
		key := typ.Namespace + "." + typ.Name
		typ.File = ""
		typ.SourceRoot = ""
		data, _ := json.Marshal(typ)
		out[key] = string(data)
	}
	return out
}

func objectFingerprints(objects []schema.Object) map[string]string {
	out := make(map[string]string, len(objects))
	for _, object := range objects {
		data, _ := json.Marshal(object)
		out[object.Name] = string(data)
	}
	return out
}

func compareFingerprints(from, to map[string]string) (added []string, removed []string, changed []string) {
	for name, toValue := range to {
		fromValue, ok := from[name]
		if !ok {
			added = append(added, name)
			continue
		}
		if fromValue != toValue {
			changed = append(changed, name)
		}
	}
	for name := range from {
		if _, ok := to[name]; !ok {
			removed = append(removed, name)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	sort.Strings(changed)
	return added, removed, changed
}

func namespaceObjects(namespace string, objects []schema.Object) []schema.Object {
	out := make([]schema.Object, len(objects))
	for i, object := range objects {
		out[i] = object
		out[i].Name = namespaceCustomName(namespace, object.Name)
		out[i].Fields = make([]schema.Field, len(object.Fields))
		for j, field := range object.Fields {
			out[i].Fields[j] = field
			out[i].Fields[j].Name = namespaceCustomName(namespace, field.Name)
			out[i].Fields[j].ReferenceTo = namespaceCustomNames(namespace, field.ReferenceTo)
		}
	}
	return out
}

func namespaceCustomMetadataRecords(namespace string, records []schema.CustomMetadataRecord) []schema.CustomMetadataRecord {
	out := make([]schema.CustomMetadataRecord, len(records))
	for i, record := range records {
		out[i] = record
		out[i].ObjectName = namespaceCustomName(namespace, record.ObjectName)
		if record.FullName != "" {
			parts := strings.SplitN(record.FullName, ".", 2)
			parts[0] = namespaceCustomName(namespace, parts[0])
			out[i].FullName = strings.Join(parts, ".")
		}
		out[i].Values = make([]schema.CustomMetadataValue, len(record.Values))
		for j, value := range record.Values {
			out[i].Values[j] = value
			out[i].Values[j].Field = namespaceCustomName(namespace, value.Field)
		}
	}
	return out
}

func namespaceCustomNames(namespace string, names []string) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = namespaceCustomName(namespace, name)
	}
	return out
}

func namespaceCustomName(namespace, name string) string {
	if namespace == "" || name == "" || !isCustomSchemaName(name) || strings.Contains(name, "__") && strings.HasPrefix(name, namespace+"__") {
		return name
	}
	if strings.Contains(strings.TrimSuffix(strings.TrimSuffix(name, "__c"), "__mdt"), "__") {
		return name
	}
	return namespace + "__" + name
}

func isCustomSchemaName(name string) bool {
	return strings.HasSuffix(name, "__c") || strings.HasSuffix(name, "__mdt")
}

func SourceHash(p project.Project) (string, error) {
	files := artifactSourceFiles(p)
	sort.Strings(files)
	sum := sha256.New()
	for _, file := range files {
		rel, err := filepath.Rel(p.Root, file)
		if err != nil {
			rel = file
		}
		_, _ = sum.Write([]byte(filepath.ToSlash(rel)))
		_, _ = sum.Write([]byte{0})
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		_, _ = sum.Write(data)
		_, _ = sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func artifactSourceFiles(p project.Project) []string {
	files := make([]string, 0)
	files = append(files, p.ApexFiles...)
	files = append(files, p.ObjectFiles...)
	files = append(files, p.FieldFiles...)
	files = append(files, p.FieldSetFiles...)
	files = append(files, p.RecordTypeFiles...)
	files = append(files, p.ValidationRuleFiles...)
	files = append(files, p.LabelFiles...)
	files = append(files, p.TranslationFiles...)
	files = append(files, p.StaticResourceFiles...)
	files = append(files, p.StaticResourceMetas...)
	files = append(files, p.DataWeaveFiles...)
	files = append(files, p.DataWeaveMetas...)
	files = append(files, p.ContentAssetFiles...)
	files = append(files, p.ContentAssetMetas...)
	files = append(files, p.EmailTemplateFiles...)
	files = append(files, p.CustomMetadataFiles...)
	files = append(files, p.WorkflowFiles...)
	files = append(files, p.FlowFiles...)
	files = append(files, p.NamedCredentialFiles...)
	files = append(files, p.RemoteSiteFiles...)
	files = append(files, p.ProfileFiles...)
	files = append(files, p.PermissionSetFiles...)
	files = append(files, p.PermissionSetGroupFiles...)
	files = append(files, p.PermissionAssignmentFiles...)
	files = append(files, p.ListViewFiles...)
	files = append(files, p.LayoutFiles...)
	files = append(files, p.CompactLayoutFiles...)
	files = append(files, p.TabFiles...)
	files = append(files, p.WebLinkFiles...)
	files = append(files, p.QuickActionFiles...)
	files = append(files, p.GlobalValueSetFiles...)
	files = append(files, p.StandardValueSetFiles...)
	files = append(files, p.FlexiPageFiles...)
	files = append(files, p.ApplicationFiles...)
	files = append(files, p.VisualforcePageFiles...)
	files = append(files, p.VisualforceComponentFiles...)
	files = append(files, p.AuraFiles...)
	files = append(files, p.LWCFiles...)
	return files
}

func globalContractTypes(types []ApexType) []ApexType {
	out := make([]ApexType, 0, len(types))
	for _, typ := range types {
		if typ.IsTest || !hasModifier(typ.Modifiers, "global") {
			continue
		}
		typ.Members = globalContractMembers(typ.Members)
		out = append(out, typ)
	}
	return out
}

func globalContractMembers(members []ApexMember) []ApexMember {
	out := make([]ApexMember, 0, len(members))
	for _, member := range members {
		if !hasModifier(member.Modifiers, "global") {
			continue
		}
		out = append(out, member)
	}
	return out
}

func hasModifier(modifiers []string, want string) bool {
	for _, modifier := range modifiers {
		if modifier == want {
			return true
		}
	}
	return false
}

func codeIntelSymbols(artifact Artifact, p project.Project) []CodeIntelSymbol {
	symbols := make([]CodeIntelSymbol, 0)
	for _, typ := range artifact.ApexTypes {
		symbols = append(symbols, codeIntelSymbolForType(artifact.Namespace, artifact.Version, typ))
		for _, member := range typ.Members {
			symbols = append(symbols, codeIntelSymbolForMember(artifact.Namespace, artifact.Version, typ, member))
		}
	}
	for _, object := range artifact.Objects {
		symbols = append(symbols, codeIntelSymbolForObject(object))
		for _, field := range object.Fields {
			symbols = append(symbols, codeIntelSymbolForField(object, field))
		}
	}
	for _, record := range artifact.CustomMetadataRecords {
		symbols = append(symbols, codeIntelSymbolForCustomMetadata(record))
	}
	for _, label := range artifactMetadataNames(artifact.LabelNames, packageLabelNames(artifact.Namespace, p.LabelFiles)) {
		symbols = append(symbols, CodeIntelSymbol{
			ID:         labelID(label),
			Kind:       "label",
			Name:       label,
			Namespace:  artifact.Namespace,
			Dependency: true,
			Artifact:   true,
		})
	}
	for _, resource := range artifactMetadataNames(artifact.StaticResourceNames, packageStaticResourceNames(artifact.Namespace, p.StaticResourceFiles, p.StaticResourceMetas)) {
		symbols = append(symbols, CodeIntelSymbol{
			ID:         staticResourceID(resource),
			Kind:       "static_resource",
			Name:       resource,
			Namespace:  artifact.Namespace,
			Dependency: true,
			Artifact:   true,
		})
	}
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].ID < symbols[j].ID
	})
	return symbols
}

func codeIntelSymbolForType(namespace, version string, typ ApexType) CodeIntelSymbol {
	if typ.Namespace == "" {
		typ.Namespace = namespace
	}
	if typ.Version == "" {
		typ.Version = version
	}
	metadata := map[string]string{
		"declarationKind": string(typ.Kind),
		"sourceRoot":      typ.SourceRoot,
		"version":         typ.Version,
	}
	return CodeIntelSymbol{
		ID:         apexTypeID(typ.Namespace, typ.Name),
		Kind:       "apex_type",
		Name:       typ.Name,
		Namespace:  typ.Namespace,
		File:       typ.File,
		Range:      typ.Range,
		Dependency: true,
		Artifact:   true,
		Metadata:   metadata,
	}
}

func codeIntelSymbolForMember(namespace, version string, typ ApexType, member ApexMember) CodeIntelSymbol {
	if typ.Namespace == "" {
		typ.Namespace = namespace
	}
	if typ.Version == "" {
		typ.Version = version
	}
	signature := codeIntelMemberSignature(member)
	return CodeIntelSymbol{
		ID:         apexMemberID(typ.Namespace, typ.Name, string(member.Kind), member.Name, signature),
		Kind:       "apex_member",
		Name:       member.Name,
		Container:  apexTypeID(typ.Namespace, typ.Name),
		Namespace:  typ.Namespace,
		Type:       member.Type,
		Signature:  signature,
		File:       typ.File,
		Range:      member.Range,
		Dependency: true,
		Artifact:   true,
		Metadata: map[string]string{
			"declarationKind": string(member.Kind),
			"owner":           typ.Name,
			"version":         typ.Version,
		},
	}
}

func codeIntelSymbolForObject(object schema.Object) CodeIntelSymbol {
	return CodeIntelSymbol{
		ID:         sObjectID(object.Name),
		Kind:       "sobject",
		Name:       object.Name,
		Dependency: true,
		Artifact:   true,
		Metadata: map[string]string{
			"label":              object.Label,
			"pluralLabel":        object.PluralLabel,
			"sharingModel":       object.SharingModel,
			"customSettingsType": object.CustomSettingsType,
		},
	}
}

func codeIntelSymbolForField(object schema.Object, field schema.Field) CodeIntelSymbol {
	return CodeIntelSymbol{
		ID:         sObjectFieldID(object.Name, field.Name),
		Kind:       "sobject_field",
		Name:       field.Name,
		Container:  sObjectID(object.Name),
		Type:       field.Type,
		Dependency: true,
		Artifact:   true,
		Metadata: map[string]string{
			"object":                object.Name,
			"label":                 field.Label,
			"relationshipName":      field.RelationshipName,
			"childRelationshipName": field.ChildRelationshipName,
			"referenceTo":           strings.Join(field.ReferenceTo, ","),
		},
	}
}

func codeIntelSymbolForCustomMetadata(record schema.CustomMetadataRecord) CodeIntelSymbol {
	return CodeIntelSymbol{
		ID:         customMetadataID(record.ObjectName, record.DeveloperName),
		Kind:       "custom_metadata",
		Name:       record.FullName,
		Container:  sObjectID(record.ObjectName),
		File:       record.File,
		Dependency: true,
		Artifact:   true,
		Metadata: map[string]string{
			"object":        record.ObjectName,
			"developerName": record.DeveloperName,
			"label":         record.Label,
		},
	}
}

func codeIntelDeclarationUses(symbols []CodeIntelSymbol) []CodeIntelUse {
	uses := make([]CodeIntelUse, 0, len(symbols))
	for _, symbol := range symbols {
		uses = append(uses, CodeIntelUse{
			SymbolID: symbol.ID,
			Kind:     "declaration",
			Name:     symbol.Name,
			File:     symbol.File,
			Range:    symbol.Range,
			Context:  symbol.Container,
			Resolved: true,
		})
	}
	return uses
}

func codeIntelMemberSignature(member ApexMember) string {
	params := make([]string, 0, len(member.Parameters))
	for _, param := range member.Parameters {
		params = append(params, strings.TrimSpace(param.Type))
	}
	return strings.TrimSpace(member.Type) + "(" + strings.Join(params, ",") + ")"
}

func cloneApexTypes(types []ApexType) []ApexType {
	return cloneJSON(types)
}

func cloneSchemaObjects(objects []schema.Object) []schema.Object {
	return cloneJSON(objects)
}

func cloneCustomMetadataRecords(records []schema.CustomMetadataRecord) []schema.CustomMetadataRecord {
	return cloneJSON(records)
}

func cloneJSON[T any](value T) T {
	data, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		return value
	}
	return out
}

func capturedSourceHash(artifact Artifact) string {
	artifact.SourceHash = ""
	artifact.BuiltAt = time.Time{}
	artifact.Capture = CaptureProvenance{}
	artifact.CodeIntelSymbolsVersion = 0
	artifact.CodeIntelSymbols = nil
	artifact.CodeIntelUsesVersion = 0
	artifact.CodeIntelUses = nil
	data, _ := json.Marshal(artifact)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func sortedUniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			seen[value] = true
		}
	}
	return sortedKeys(seen)
}

func sortedLightningBundles(bundles []LightningBundle) []LightningBundle {
	out := make([]LightningBundle, 0, len(bundles))
	for _, bundle := range bundles {
		bundle.Namespace = strings.TrimSpace(bundle.Namespace)
		bundle.Name = strings.TrimSpace(bundle.Name)
		bundle.Type = strings.TrimSpace(bundle.Type)
		if bundle.Name == "" {
			continue
		}
		out = append(out, bundle)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].QualifiedName() == out[j].QualifiedName() {
			return out[i].Type < out[j].Type
		}
		return out[i].QualifiedName() < out[j].QualifiedName()
	})
	return out
}

func sortedApexTypes(types []ApexType) []ApexType {
	out := append([]ApexType(nil), types...)
	for i := range out {
		sort.Slice(out[i].Members, func(j, k int) bool {
			if out[i].Members[j].Name == out[i].Members[k].Name {
				if out[i].Members[j].Kind == out[i].Members[k].Kind {
					return apexMemberSortKey(out[i].Members[j]) < apexMemberSortKey(out[i].Members[k])
				}
				return out[i].Members[j].Kind < out[i].Members[k].Kind
			}
			return out[i].Members[j].Name < out[i].Members[k].Name
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			if out[i].Name == out[j].Name {
				return out[i].Kind < out[j].Kind
			}
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}

func apexMemberSortKey(member ApexMember) string {
	return strings.TrimSpace(member.Type) + "|" + codeIntelMemberSignature(member)
}

func sortedSchemaObjects(objects []schema.Object) []schema.Object {
	out := append([]schema.Object(nil), objects...)
	for i := range out {
		sort.Slice(out[i].Fields, func(j, k int) bool {
			return out[i].Fields[j].Name < out[i].Fields[k].Name
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func sortedCustomMetadataRecords(records []schema.CustomMetadataRecord) []schema.CustomMetadataRecord {
	out := append([]schema.CustomMetadataRecord(nil), records...)
	for i := range out {
		sort.Slice(out[i].Values, func(j, k int) bool {
			return out[i].Values[j].Field < out[i].Values[k].Field
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ObjectName == out[j].ObjectName {
			if out[i].FullName == out[j].FullName {
				return out[i].DeveloperName < out[j].DeveloperName
			}
			return out[i].FullName < out[j].FullName
		}
		return out[i].ObjectName < out[j].ObjectName
	})
	return out
}

func artifactMetadataNames(values ...[]string) []string {
	seen := make(map[string]bool)
	for _, list := range values {
		for _, value := range list {
			value = strings.TrimSpace(value)
			if value != "" {
				seen[value] = true
			}
		}
	}
	return sortedKeys(seen)
}

type labelsXML struct {
	Labels []struct {
		FullName string `xml:"fullName"`
	} `xml:"labels"`
}

func packageLabelNames(namespace string, paths []string) []string {
	seen := make(map[string]bool)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var labels labelsXML
		if err := xml.Unmarshal(data, &labels); err != nil {
			continue
		}
		for _, label := range labels.Labels {
			name := namespaceMetadataName(namespace, strings.TrimSpace(label.FullName))
			if name != "" {
				seen[name] = true
			}
		}
	}
	return sortedKeys(seen)
}

func packageStaticResourceNames(namespace string, filePaths, metaPaths []string) []string {
	seen := make(map[string]bool)
	for _, path := range append(append([]string{}, filePaths...), metaPaths...) {
		name := staticResourceNameFromPath(path)
		name = namespaceMetadataName(namespace, name)
		if name != "" {
			seen[name] = true
		}
	}
	return sortedKeys(seen)
}

func staticResourceNameFromPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, "-meta.xml")
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return strings.TrimSuffix(base, ext)
}

func namespaceMetadataName(namespace, name string) string {
	if namespace == "" || name == "" || strings.Contains(name, "__") {
		return name
	}
	return namespace + "__" + name
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func apexTypeID(namespace, name string) string {
	return joinCodeIntelID("apex", "type", namespace, name)
}

func apexMemberID(namespace, typeName, kind, name, signature string) string {
	return joinCodeIntelID("apex", "member", namespace, typeName, kind, name, signature)
}

func sObjectID(name string) string {
	return joinCodeIntelID("schema", "object", name)
}

func sObjectFieldID(objectName, fieldName string) string {
	return joinCodeIntelID("schema", "field", objectName, fieldName)
}

func customMetadataID(objectName, developerName string) string {
	return joinCodeIntelID("schema", "custom_metadata", objectName, developerName)
}

func labelID(name string) string {
	return joinCodeIntelID("metadata", "label", name)
}

func staticResourceID(name string) string {
	return joinCodeIntelID("metadata", "static_resource", name)
}

func joinCodeIntelID(parts ...string) string {
	escaped := make([]string, len(parts))
	for i, part := range parts {
		escaped[i] = escapeCodeIntelIDPart(part)
	}
	return strings.Join(escaped, ":")
}

func escapeCodeIntelIDPart(part string) string {
	part = strings.ReplaceAll(part, "%", "%25")
	part = strings.ReplaceAll(part, ":", "%3A")
	return part
}
