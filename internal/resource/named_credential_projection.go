package resource

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Legacy named credentials expose their configured root URL as Endpoint through
// SOQL. Project only the nonsecret, unmanaged no-authentication form here; modern named
// credentials store their URL in parameters instead of the legacy Endpoint field.
// https://developer.salesforce.com/docs/atlas.en-us.object_reference.meta/object_reference/sforce_api_objects_namedcredential.htm
func ensureNamedCredentialObject(org *storage.OrgState, p project.Project) {
	if org == nil || strings.TrimSpace(org.Namespace) != "" || strings.TrimSpace(p.Namespace) != "" {
		return
	}
	ensureSecuredEndpointNamedCredentialRows(org, p)
	sourceFiles := make(map[string]bool, len(p.NamedCredentialFiles))
	for _, path := range p.NamedCredentialFiles {
		sourceFiles[path] = true
	}
	for _, endpoint := range org.Metadata.Endpoints {
		name := strings.TrimSpace(endpoint.Name)
		if !strings.EqualFold(endpoint.Kind, "NamedCredential") || !endpoint.Active || !sourceFiles[endpoint.File] ||
			name == "" || strings.Contains(name, "__") || strings.Contains(name, ".") || strings.TrimSpace(endpoint.URL) == "" ||
			!strings.EqualFold(endpoint.Protocol, "NoAuthentication") || !legacyNamedCredentialPrincipal(endpoint.PrincipalType) ||
			!legacyNamedCredentialSource(endpoint.File) {
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
		if _, _, exists := storage.LookupRecordByID(object.Records, id); exists {
			continue
		}
		object.Records[id] = storage.Record{ID: id, Object: "NamedCredential", Fields: map[string]storage.Value{
			"Id":              storage.IDValue(id),
			"DeveloperName":   storage.StringValue(name),
			"Endpoint":        storage.StringValue(endpoint.URL),
			"NamespacePrefix": storage.NullValue(),
		}}
		org.Objects["NamedCredential"] = *object
	}
}

// Scratch-org row (API 62, v031 indra): a legacy NoAuthentication credential
// with principalType NamedUser exposes DeveloperName and Endpoint like the
// Anonymous form. Per-user and unspecified principals stay unprojected.
func legacyNamedCredentialPrincipal(principal string) bool {
	return strings.EqualFold(principal, "Anonymous") || strings.EqualFold(principal, "NamedUser")
}

func legacyNamedCredentialSource(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var source struct {
		XMLName    xml.Name   `xml:"NamedCredential"`
		Type       string     `xml:"namedCredentialType"`
		Parameters []struct{} `xml:"namedCredentialParameters"`
	}
	if err := xml.Unmarshal(data, &source); err != nil {
		return false
	}
	typeName := strings.TrimSpace(source.Type)
	return (typeName == "" || strings.EqualFold(typeName, "Legacy")) && len(source.Parameters) == 0
}

// Scratch-org row (API 62): a SecuredEndpoint named credential answers
// SELECT DeveloperName, MasterLabel, Endpoint, PrincipalType, NamespacePrefix
// with its label and null Endpoint, PrincipalType and NamespacePrefix. Its URL
// lives in parameters, so the legacy Endpoint field stays empty.
func ensureSecuredEndpointNamedCredentialRows(org *storage.OrgState, p project.Project) {
	for _, path := range p.NamedCredentialFiles {
		data, err := os.ReadFile(path)
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
		if _, _, exists := storage.LookupRecordByID(object.Records, id); exists {
			continue
		}
		label := strings.TrimSpace(source.Label)
		if label == "" {
			label = name
		}
		object.Records[id] = storage.Record{ID: id, Object: "NamedCredential", Fields: map[string]storage.Value{
			"Id":              storage.IDValue(id),
			"DeveloperName":   storage.StringValue(name),
			"MasterLabel":     storage.StringValue(label),
			"Endpoint":        storage.NullValue(),
			"PrincipalType":   storage.NullValue(),
			"NamespacePrefix": storage.NullValue(),
		}}
		org.Objects["NamedCredential"] = *object
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
