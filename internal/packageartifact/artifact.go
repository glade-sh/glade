package packageartifact

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

type Artifact struct {
	Namespace             string                        `json:"namespace"`
	PackageName           string                        `json:"packageName,omitempty"`
	Version               string                        `json:"version,omitempty"`
	SourceRoot            string                        `json:"sourceRoot,omitempty"`
	SourceHash            string                        `json:"sourceHash,omitempty"`
	SourceAPIVersion      string                        `json:"sourceApiVersion,omitempty"`
	BuiltAt               time.Time                     `json:"builtAt"`
	ApexTypes             []ApexType                    `json:"apexTypes,omitempty"`
	Objects               []schema.Object               `json:"objects,omitempty"`
	CustomMetadataRecords []schema.CustomMetadataRecord `json:"customMetadataRecords,omitempty"`
	Labels                int                           `json:"labels"`
	StaticResources       int                           `json:"staticResources"`
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

func Build(namespace, version string, p project.Project, s schema.Schema, apexTypes []ApexType) (Artifact, error) {
	sourceHash, err := SourceHash(p)
	if err != nil {
		return Artifact{}, err
	}
	return Artifact{
		Namespace:             namespace,
		Version:               version,
		SourceRoot:            p.Root,
		SourceHash:            sourceHash,
		SourceAPIVersion:      p.SourceAPIVersion,
		BuiltAt:               time.Now().UTC(),
		ApexTypes:             globalContractTypes(apexTypes),
		Objects:               namespaceObjects(namespace, s.Objects),
		CustomMetadataRecords: namespaceCustomMetadataRecords(namespace, s.CustomMetadataRecords),
		Labels:                len(p.LabelFiles),
		StaticResources:       len(p.StaticResourceFiles) + len(p.StaticResourceMetas),
	}, nil
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
