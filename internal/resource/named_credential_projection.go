package resource

import (
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// NamedCredentialMetadataFiles includes credentials read from loaded dependencies.
func NamedCredentialMetadataFiles(p project.Project) []string {
	files := append([]string(nil), p.NamedCredentialFiles...)
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status == "loaded" && dep.Project != nil {
			files = append(files, NamedCredentialMetadataFiles(*dep.Project)...)
		}
	}
	return files
}

// Legacy named credentials expose their configured root URL as Endpoint through
// SOQL. Project only the nonsecret, unmanaged no-authentication form here; modern named
// credentials store their URL in parameters instead of the legacy Endpoint field.
// https://developer.salesforce.com/docs/atlas.en-us.object_reference.meta/object_reference/sforce_api_objects_namedcredential.htm
func ensureNamedCredentialObject(org *storage.OrgState, p project.Project, readMetadata func(string) ([]byte, error)) {
	if org == nil || strings.TrimSpace(org.Namespace) != "" || strings.TrimSpace(p.Namespace) != "" {
		return
	}
	sourceDigest := sha256.Sum256([]byte(filepath.Clean(p.Root)))
	legacyUnit := fmt.Sprintf("glade:legacy-named-credential:%x", sourceDigest)
	sourceUnit := legacyUnit
	if identity := projectDirectoryIdentity(p.Root); identity != "" {
		digest := sha256.Sum256([]byte(identity))
		sourceUnit = fmt.Sprintf("glade:legacy-named-credential:dir:%x", digest)
	}
	if sourceUnit != legacyUnit {
		if _, ok := org.Objects["NamedCredential"]; ok {
			object, _ := storage.EnsureMutableObjectRecords(org, "NamedCredential")
			for id, record := range object.Records {
				if record.MetadataSourceUnit == legacyUnit {
					record.MetadataSourceUnit = sourceUnit
					object.PutRecord(id, record)
				}
			}
			org.Objects["NamedCredential"] = *object
		}
	}
	ensureSecuredEndpointNamedCredentialRows(org, p, sourceUnit, readMetadata)
	sourceFiles := make(map[string]bool, len(p.NamedCredentialFiles))
	sourceNames := make(map[string]bool, len(p.NamedCredentialFiles))
	for _, path := range p.NamedCredentialFiles {
		sourceFiles[path] = true
		sourceNames[strings.ToLower(namedCredentialNameFromPath(path))] = true
	}
	for _, endpoint := range org.Metadata.Endpoints {
		name := strings.TrimSpace(endpoint.Name)
		if !strings.EqualFold(endpoint.Kind, "NamedCredential") || !endpoint.Active || !sourceFiles[endpoint.File] ||
			name == "" || strings.Contains(name, "__") || strings.Contains(name, ".") || strings.TrimSpace(endpoint.URL) == "" ||
			!strings.EqualFold(endpoint.Protocol, "NoAuthentication") || !legacyNamedCredentialPrincipal(endpoint.PrincipalType) {
			continue
		}
		label, valid := legacyNamedCredentialSource(endpoint.File, readMetadata)
		if !valid {
			continue
		}
		storage.EnsureStandardObject(org, "NamedCredential")
		object, _ := storage.EnsureMutableObjectRecords(org, "NamedCredential")
		if object == nil || object.Definition.KeyPrefix == "" {
			continue
		}
		matches := func(record storage.Record) bool {
			return strings.EqualFold(record.Fields["DeveloperName"].String, name) && record.Fields["NamespacePrefix"].String == ""
		}
		id := metadataRecordIDForIdentity(*object, matches)
		if key, existing, exists := storage.LookupRecordByID(object.Records, id); exists {
			_, hasPrincipal := existing.Fields["PrincipalType"]
			_, hasLabel := existing.Fields["MasterLabel"]
			owned := existing.MetadataSourceUnit == sourceUnit
			legacyRow := existing.MetadataSourceUnit == "" && !hasPrincipal && !hasLabel && existing.Fields["Endpoint"].String == endpoint.URL
			if owned || (strings.EqualFold(endpoint.PrincipalType, "Anonymous") && legacyRow) {
				existing.Fields = maps.Clone(existing.Fields)
				existing.Fields["Endpoint"] = storage.StringValue(endpoint.URL)
				if strings.EqualFold(endpoint.PrincipalType, "Anonymous") {
					existing.Fields["PrincipalType"] = storage.StringValue("Anonymous")
					if label != "" {
						existing.Fields["MasterLabel"] = storage.StringValue(label)
					}
				} else {
					delete(existing.Fields, "PrincipalType")
					delete(existing.Fields, "MasterLabel")
				}
				existing.MetadataSourceUnit = sourceUnit
				object.PutRecord(key, existing)
				org.Objects["NamedCredential"] = *object
				storage.RebuildObjectIndexes(org, "NamedCredential")
			}
			continue
		}
		object.PutRecord(id, storage.Record{ID: id, Object: "NamedCredential", MetadataSourceUnit: sourceUnit, Fields: map[string]storage.Value{
			"Id":              storage.IDValue(id),
			"DeveloperName":   storage.StringValue(name),
			"Endpoint":        storage.StringValue(endpoint.URL),
			"NamespacePrefix": storage.NullValue(),
		}})
		if strings.EqualFold(endpoint.PrincipalType, "Anonymous") {
			record := object.Records[id]
			record.Fields["PrincipalType"] = storage.StringValue("Anonymous")
			if label != "" {
				record.Fields["MasterLabel"] = storage.StringValue(label)
			}
			object.PutRecord(id, record)
		}
		org.Objects["NamedCredential"] = *object
		storage.RebuildObjectIndexes(org, "NamedCredential")
	}
	if existing, ok := org.Objects["NamedCredential"]; ok && existing.Records != nil {
		object, _ := storage.EnsureMutableObjectRecords(org, "NamedCredential")
		changed := false
		for id, record := range object.Records {
			if record.MetadataSourceUnit == sourceUnit && !sourceNames[strings.ToLower(record.Fields["DeveloperName"].String)] {
				object.DeleteRecord(id)
				changed = true
			}
		}
		if changed {
			org.Objects["NamedCredential"] = *object
			storage.RebuildObjectIndexes(org, "NamedCredential")
		}
	}
}

// Scratch-org row (API 62, v031 indra): a legacy NoAuthentication credential
// with principalType NamedUser exposes DeveloperName and Endpoint like the
// Anonymous form. Per-user and unspecified principals stay unprojected.
func legacyNamedCredentialPrincipal(principal string) bool {
	return strings.EqualFold(principal, "Anonymous") || strings.EqualFold(principal, "NamedUser")
}

func legacyNamedCredentialSource(path string, readMetadata func(string) ([]byte, error)) (string, bool) {
	data, err := readMetadata(path)
	if err != nil {
		return "", false
	}
	var source struct {
		XMLName    xml.Name   `xml:"NamedCredential"`
		Label      string     `xml:"label"`
		Type       string     `xml:"namedCredentialType"`
		Parameters []struct{} `xml:"namedCredentialParameters"`
	}
	if err := xml.Unmarshal(data, &source); err != nil {
		return "", false
	}
	typeName := strings.TrimSpace(source.Type)
	return strings.TrimSpace(source.Label), (typeName == "" || strings.EqualFold(typeName, "Legacy")) && len(source.Parameters) == 0
}

// Scratch-org row (API 62): a SecuredEndpoint named credential answers
// SELECT DeveloperName, MasterLabel, Endpoint, PrincipalType, NamespacePrefix
// with its label and null Endpoint, PrincipalType and NamespacePrefix. Its URL
// lives in parameters, so the legacy Endpoint field stays empty.
func ensureSecuredEndpointNamedCredentialRows(org *storage.OrgState, p project.Project, sourceUnit string, readMetadata func(string) ([]byte, error)) {
	for _, path := range p.NamedCredentialFiles {
		data, err := readMetadata(path)
		if err != nil {
			continue
		}
		var source struct {
			XMLName xml.Name `xml:"NamedCredential"`
			Label   string   `xml:"label"`
			Type    string   `xml:"namedCredentialType"`
		}
		if err := xml.Unmarshal(data, &source); err != nil || !strings.EqualFold(strings.TrimSpace(source.Type), "SecuredEndpoint") {
			continue
		}
		name := namedCredentialNameFromPath(path)
		if name == "" || strings.Contains(name, "__") || strings.Contains(name, ".") {
			continue
		}
		storage.EnsureStandardObject(org, "NamedCredential")
		object, _ := storage.EnsureMutableObjectRecords(org, "NamedCredential")
		if object == nil || object.Definition.KeyPrefix == "" {
			continue
		}
		matches := func(record storage.Record) bool {
			return strings.EqualFold(record.Fields["DeveloperName"].String, name) && record.Fields["NamespacePrefix"].String == ""
		}
		id := metadataRecordIDForIdentity(*object, matches)
		if _, existing, exists := storage.LookupRecordByID(object.Records, id); exists {
			if existing.MetadataSourceUnit != sourceUnit {
				continue
			}
		}
		label := strings.TrimSpace(source.Label)
		if label == "" {
			label = name
		}
		object.PutRecord(id, storage.Record{ID: id, Object: "NamedCredential", MetadataSourceUnit: sourceUnit, Fields: map[string]storage.Value{
			"Id":              storage.IDValue(id),
			"DeveloperName":   storage.StringValue(name),
			"MasterLabel":     storage.StringValue(label),
			"Endpoint":        storage.NullValue(),
			"PrincipalType":   storage.NullValue(),
			"NamespacePrefix": storage.NullValue(),
		}})
		org.Objects["NamedCredential"] = *object
		storage.RebuildObjectIndexes(org, "NamedCredential")
	}
}

func namedCredentialNameFromPath(path string) string {
	name := filepath.Base(path)
	for _, suffix := range []string{".namedCredential-meta.xml", ".namedCredential"} {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSpace(strings.TrimSuffix(name, suffix))
		}
	}
	return ""
}
