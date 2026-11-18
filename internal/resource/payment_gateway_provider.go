package resource

import (
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// applyPaymentGatewayProviders projects the captured, unmanaged metadata shape.
// The adapter reference is bound to the same ApexClass row used by queries.
func applyPaymentGatewayProviders(org *storage.OrgState, p project.Project, read func(string) ([]byte, error)) error {
	if org == nil || p.Namespace != "" || org.Namespace != "" {
		return nil
	}
	sourceDigest := sha256.Sum256([]byte(filepath.Clean(p.Root)))
	legacyUnit := fmt.Sprintf("glade:payment-gateway-provider:%x", sourceDigest)
	sourceUnit := legacyUnit
	if identity := projectDirectoryIdentity(p.Root); identity != "" {
		digest := sha256.Sum256([]byte(identity))
		sourceUnit = fmt.Sprintf("glade:payment-gateway-provider:dir:%x", digest)
	}
	// Upgrade rows produced by the earlier path-based owner while the project
	// is still at that path. Never claim rows belonging to another project.
	if sourceUnit != legacyUnit {
		for _, kind := range []string{"PaymentGatewayProvider", "ApexClass"} {
			if _, ok := org.Objects[kind]; !ok {
				continue
			}
			object, _ := storage.EnsureMutableObjectRecords(org, kind)
			for id, record := range object.Records {
				oldUnit, newUnit := legacyUnit, sourceUnit
				if kind == "ApexClass" {
					oldUnit += ":adapter"
					newUnit += ":adapter"
				}
				if record.MetadataSourceUnit == oldUnit {
					record.MetadataSourceUnit = newUnit
					object.PutRecord(id, record)
				}
			}
			org.Objects[kind] = *object
		}
	}
	classSourceUnit := ProjectApexClassSourceUnit(p.Root)
	seen := make(map[storage.ID]bool)
	for _, path := range p.PaymentGatewayProviderFiles {
		data, err := read(path)
		if err != nil {
			return err
		}
		var source struct {
			XMLName     xml.Name `xml:"PaymentGatewayProvider"`
			Adapter     string   `xml:"apexAdapter"`
			Label       string   `xml:"masterLabel"`
			Comments    string   `xml:"comments"`
			Idempotency string   `xml:"idempotencySupported"`
		}
		if err := xml.Unmarshal(data, &source); err != nil {
			return fmt.Errorf("payment gateway provider metadata: %w", err)
		}
		name := filepath.Base(path)
		for _, suffix := range []string{".paymentGatewayProvider-meta.xml", ".paymentGatewayProvider"} {
			if strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix)) {
				name = name[:len(name)-len(suffix)]
				break
			}
		}
		if name == "" || strings.Contains(name, "__") || strings.Contains(name, ".") {
			continue
		}
		storage.EnsureStandardObject(org, "PaymentGatewayProvider")
		object, _ := storage.EnsureMutableObjectRecords(org, "PaymentGatewayProvider")
		if object == nil || object.Definition.KeyPrefix == "" {
			continue
		}
		id := metadataRecordIDForIdentity(*object, func(r storage.Record) bool {
			return strings.EqualFold(r.Fields["DeveloperName"].String, name) && r.Fields["NamespacePrefix"].String == ""
		})
		if _, existing, exists := storage.LookupRecordByID(object.Records, id); exists && existing.MetadataSourceUnit != sourceUnit {
			continue
		}
		adapterBody, valid, err := projectAdapterSource(p, strings.TrimSpace(source.Adapter), read)
		if err != nil {
			return err
		}
		if !valid {
			continue
		}
		var adapter storage.ID
		for id, record := range org.Objects["ApexClass"].Records {
			if record.MetadataSourceUnit == classSourceUnit && strings.EqualFold(record.Fields["Name"].String, strings.TrimSpace(source.Adapter)) && record.Fields["NamespacePrefix"].String == "" && !record.System.IsDeleted && (adapter == "" || id < adapter) {
				adapter = id
			}
		}
		if adapter != "" {
			classes, _ := storage.EnsureMutableObjectRecords(org, "ApexClass")
			record := classes.Records[adapter]
			record.Fields = maps.Clone(record.Fields)
			record.Fields["Body"] = storage.StringValue(string(adapterBody))
			record.Fields["LengthWithoutComments"] = storage.IntegerValue(int64(len(adapterBody)))
			classes.PutRecord(adapter, record)
			org.Objects["ApexClass"] = *classes
		}
		if adapter == "" {
			adapter = projectPaymentAdapterClass(org, strings.TrimSpace(source.Adapter), sourceUnit+":adapter", adapterBody)
		}
		if adapter == "" {
			continue
		}
		seen[id] = true
		object.PutRecord(id, storage.Record{ID: id, Object: "PaymentGatewayProvider", MetadataSourceUnit: sourceUnit, Fields: map[string]storage.Value{
			"Id": storage.IDValue(id), "DeveloperName": storage.StringValue(name), "MasterLabel": storage.StringValue(strings.TrimSpace(source.Label)),
			"Comments": storage.StringValue(source.Comments), "IdempotencySupported": storage.StringValue(source.Idempotency),
			"NamespacePrefix": storage.NullValue(), "ApexAdapterId": storage.IDValue(adapter),
		}})
		org.Objects["PaymentGatewayProvider"] = *object
	}
	if existing, ok := org.Objects["PaymentGatewayProvider"]; ok && existing.Records != nil {
		object, _ := storage.EnsureMutableObjectRecords(org, "PaymentGatewayProvider")
		for id, record := range object.Records {
			if record.MetadataSourceUnit == sourceUnit && !seen[id] {
				object.DeleteRecord(id)
			}
		}
		org.Objects["PaymentGatewayProvider"] = *object
		storage.RebuildObjectIndexes(org, "PaymentGatewayProvider")
	}
	if existing, ok := org.Objects["ApexClass"]; ok && existing.Records != nil {
		object, _ := storage.EnsureMutableObjectRecords(org, "ApexClass")
		changed := false
		for id, record := range object.Records {
			if record.MetadataSourceUnit != sourceUnit+":adapter" {
				continue
			}
			body, valid, err := projectAdapterSource(p, record.Fields["Name"].String, read)
			if err != nil {
				return err
			}
			if valid {
				record.Fields = maps.Clone(record.Fields)
				record.Fields["Body"] = storage.StringValue(string(body))
				record.Fields["LengthWithoutComments"] = storage.IntegerValue(int64(len(body)))
				object.PutRecord(id, record)
				changed = true
				continue
			}
			referenced := false
			for _, provider := range org.Objects["PaymentGatewayProvider"].Records {
				if provider.Fields["ApexAdapterId"].ID == id {
					referenced = true
					break
				}
			}
			if !referenced {
				object.DeleteRecord(id)
				changed = true
			}
		}
		if changed {
			org.Objects["ApexClass"] = *object
			storage.RebuildObjectIndexes(org, "ApexClass")
		}
	}
	return nil
}

// projectPaymentAdapterClass seeds a declared class when a caller has not
// already populated ApexClass records. It does not invent absent classes.
func projectPaymentAdapterClass(org *storage.OrgState, name, sourceUnit string, body []byte) storage.ID {
	storage.EnsureStandardObject(org, "ApexClass")
	object, _ := storage.EnsureMutableObjectRecords(org, "ApexClass")
	if object == nil || object.Definition.KeyPrefix == "" {
		return ""
	}
	id := metadataRecordIDForIdentity(*object, func(r storage.Record) bool {
		return !r.System.IsDeleted && r.MetadataSourceUnit == sourceUnit && strings.EqualFold(r.Fields["Name"].String, name) && r.Fields["NamespacePrefix"].String == ""
	})
	object.PutRecord(id, storage.Record{ID: id, Object: "ApexClass", MetadataSourceUnit: sourceUnit, Fields: map[string]storage.Value{
		"Id": storage.IDValue(id), "Name": storage.StringValue(name), "NamespacePrefix": storage.NullValue(),
		"Body": storage.StringValue(string(body)), "ApiVersion": storage.DecimalValue("65.0"), "LengthWithoutComments": storage.IntegerValue(int64(len(body))),
	}})
	org.Objects["ApexClass"] = *object
	storage.RebuildObjectIndexes(org, "ApexClass")
	return id
}

func projectAdapterSource(p project.Project, name string, read func(string) ([]byte, error)) ([]byte, bool, error) {
	seen := make(map[string]bool)
	var original []byte
	implements, methodFound := false, false
	for name != "" && !strings.ContainsAny(name, ".:") && !seen[strings.ToLower(name)] {
		seen[strings.ToLower(name)] = true
		path := adapterClassPath(p, name)
		if path == "" {
			return nil, false, nil
		}
		body, err := read(path)
		if err != nil {
			return nil, false, fmt.Errorf("read payment adapter source: %w", err)
		}
		file := apexast.ParseSource(path, string(body))
		if len(file.Diagnostics) != 0 {
			return nil, false, nil
		}
		if original == nil {
			original = body
		}
		parent := ""
		found := false
		for _, decl := range file.Declarations {
			if decl.Kind != apexast.DeclarationClass || !strings.EqualFold(decl.Name, name) {
				continue
			}
			found = true
			parent = decl.SuperClass
			for _, implemented := range decl.Interfaces {
				if strings.EqualFold(strings.ReplaceAll(implemented, " ", ""), "commercepayments.PaymentGatewayAdapter") {
					implements = true
				}
			}
			for _, method := range decl.Members {
				if method.Kind == apexast.DeclarationMethod && strings.EqualFold(method.Name, "processRequest") && strings.EqualFold(method.Type, "commercepayments.GatewayResponse") && len(method.Parameters) == 1 && strings.EqualFold(method.Parameters[0].Type, "commercepayments.PaymentGatewayContext") {
					methodFound = true
				}
			}
			break
		}
		if !found {
			return nil, false, nil
		}
		if implements && methodFound {
			return original, true, nil
		}
		name = parent
	}
	return nil, false, nil
}

func adapterClassPath(p project.Project, name string) string {
	for _, path := range p.ApexFiles {
		base := filepath.Base(path)
		if strings.EqualFold(filepath.Ext(base), ".cls") && strings.EqualFold(strings.TrimSuffix(base, filepath.Ext(base)), name) {
			return path
		}
	}
	return ""
}

// PaymentGatewayProviderSourceFiles lists local adapter sources read by the loader.
func PaymentGatewayProviderSourceFiles(p project.Project, read func(string) ([]byte, error)) ([]string, error) {
	var paths []string
	for _, metadataPath := range p.PaymentGatewayProviderFiles {
		data, err := read(metadataPath)
		if err != nil {
			return nil, err
		}
		var source struct {
			Adapter string `xml:"apexAdapter"`
		}
		if err := xml.Unmarshal(data, &source); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(source.Adapter)
		seen := make(map[string]bool)
		for name != "" && !strings.ContainsAny(name, ".:") && !seen[strings.ToLower(name)] {
			seen[strings.ToLower(name)] = true
			path := adapterClassPath(p, name)
			if path == "" {
				break
			}
			paths = append(paths, path)
			body, err := read(path)
			if err != nil {
				return nil, err
			}
			file := apexast.ParseSource(path, string(body))
			parent := ""
			for _, decl := range file.Declarations {
				if decl.Kind == apexast.DeclarationClass && strings.EqualFold(decl.Name, name) {
					parent = decl.SuperClass
					break
				}
			}
			name = parent
		}
	}
	return paths, nil
}
