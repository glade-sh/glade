package compile

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/namespaceremap"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

type salesforceSchemaObject struct {
	Fields        map[string]bool     `json:"fields"`
	Relationships map[string][]string `json:"relationships"`
}

type salesforceImportConfig struct {
	OwnNamespace      string                            `json:"ownNamespace,omitempty"`
	OwnSchemaObjects  map[string]bool                   `json:"ownSchemaObjects,omitempty"`
	Schema            map[string]salesforceSchemaObject `json:"schema"`
	Labels            map[string]bool                   `json:"labels"`
	Resources         map[string]bool                   `json:"resources"`
	Assets            map[string]bool                   `json:"assets"`
	CustomPermissions map[string]bool                   `json:"customPermissions"`
}

// Collect decoded AST specifiers before resolving shared metadata references.
// The compiler's import validation consumes the same decoded references.
func collectSalesforceImportReferences(cfg compileConfig, roots compileRoots) ([]string, error) {
	payload, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	cmd := lwcCompilerCommand(roots)
	cmd.Args = append(cmd.Args, "--salesforce-import-references")
	cmd.Stdin = strings.NewReader(string(payload))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("lwc compile: %w\n%s", err, string(out))
	}
	var references []string
	if err := json.Unmarshal(out, &references); err != nil {
		return nil, fmt.Errorf("decode Salesforce import references: %w\n%s", err, string(out))
	}
	return references, nil
}

func loadSalesforceImports(p project.Project, references []string) (*salesforceImportConfig, error) {
	registry, err := resource.LoadProjectWithDependencies(p)
	if err != nil {
		return nil, err
	}
	cfg := &salesforceImportConfig{
		OwnNamespace: p.Namespace,
		Schema:       map[string]salesforceSchemaObject{}, Labels: map[string]bool{},
		Resources: map[string]bool{}, Assets: map[string]bool{}, CustomPermissions: map[string]bool{},
	}
	// Only declared label sections participate in compiler reference lookup.
	// Native controls accept C as well as c, but reject an unknown section.
	labelNamespaces := map[string]bool{"c": true}
	for _, label := range registry.Labels {
		if label.Namespace != "" {
			labelNamespaces[strings.ToLower(label.Namespace)] = true
		}
	}
	for _, r := range registry.StaticResources {
		cfg.Resources[r.Name] = true
	}
	for _, a := range registry.ContentAssets {
		cfg.Assets[a.Name] = true
	}
	projectSchema, err := loadSalesforceProjectMetadata(p, cfg)
	if err != nil {
		return nil, err
	}
	// Apply only the observed feature's contribution. Preserve the existing
	// compiler schema for all other feature and edition overlays.
	features := project.StandardFieldPolicyFeatures(p)
	withoutCurrency := make([]string, 0, len(features))
	multiCurrency := false
	for _, feature := range features {
		name, _, _ := strings.Cut(strings.TrimSpace(feature), ":")
		if strings.EqualFold(name, "MultiCurrency") {
			multiCurrency = true
			continue
		}
		withoutCurrency = append(withoutCurrency, feature)
	}
	schemaEntry := func(definition storage.ObjectDefinition) salesforceSchemaObject {
		entry := salesforceSchemaEntry(definition)
		if _, known := storage.ResolveKnownStandardObjectName(definition.APIName); !known || !multiCurrency {
			return entry
		}
		without, with := definition.Clone(), definition.Clone()
		storage.EnsureStandardObjectFieldsForFeatures(&without, withoutCurrency)
		storage.EnsureStandardObjectFieldsForFeatures(&with, features)
		off, on := salesforceSchemaEntry(without), salesforceSchemaEntry(with)
		for field := range on.Fields {
			if !off.Fields[field] {
				entry.Fields[field] = true
			}
		}
		for relationship, targets := range on.Relationships {
			if _, exists := off.Relationships[relationship]; !exists {
				if _, declared := entry.Relationships[relationship]; !declared {
					entry.Relationships[relationship] = targets
				}
			}
		}
		return entry
	}
	// Reuse the runtime's schema overlay, including implicit custom-object
	// fields and its custom-setting/master-detail exclusions.
	for name, object := range sobject.BuildDescribeRegistry(projectSchema).Objects {
		cfg.Schema[name] = schemaEntry(sobject.ToObjectDefinition(object))
	}
	cfg.OwnSchemaObjects = addOwnObjectImportAliases(cfg.Schema, p.Namespace)
	loadObject := func(name string) {
		if _, loaded := cfg.Schema[name]; loaded {
			return
		}
		if definition, ok := storage.StandardObjectDefinition(name); ok && definition.APIName == name {
			cfg.Schema[name] = schemaEntry(definition)
		}
	}
	for _, reference := range references {
		if label, ok := strings.CutPrefix(reference, "@salesforce/label/"); ok {
			namespace, name, qualified := strings.Cut(label, ".")
			if qualified && labelNamespaces[strings.ToLower(namespace)] {
				// The shared resolver handles c aliases, namespaced full names
				// and the registry's already-applied dependency remaps.
				if _, status := resource.ResolveLabel(registry, p.Namespace, namespace, name); status == resource.LabelLookupResolved {
					cfg.Labels[label] = true
				}
			}
			continue
		}
		name, schemaReference := strings.CutPrefix(reference, "@salesforce/schema/")
		if !schemaReference {
			continue
		}
		parts := strings.Split(name, ".")
		names := []string{parts[0]}
		for _, part := range parts[1:] {
			var next []string
			for _, name := range names {
				loadObject(name)
				next = append(next, cfg.Schema[name].Relationships[part]...)
			}
			names = next
		}
		for _, name := range names {
			loadObject(name)
		}
	}
	return cfg, nil
}

// project.Load has already resolved package directories, dependency projects
// and namespace remaps. Consume that graph without scanning for more sources.
// addOwnObjectImportAliases supplies the unqualified names used in the
// owning namespace for source objects and their own custom fields. A separately
// declared unqualified object keeps priority. Other namespaces are unchanged.
func addOwnObjectImportAliases(objects map[string]salesforceSchemaObject, namespace string) map[string]bool {
	own := make(map[string]bool)
	if namespace == "" {
		return own
	}
	prefix := namespace + "__"
	for name, object := range objects {
		if !strings.HasPrefix(name, prefix) || !strings.Contains(name[len(prefix):], "__") {
			continue
		}
		own[name] = true
		short := name[len(prefix):]
		if _, exists := objects[short]; exists {
			continue
		}
		fields := make(map[string]bool, len(object.Fields))
		for field, present := range object.Fields {
			fields[field] = present
		}
		for field, present := range object.Fields {
			if strings.HasPrefix(field, prefix) && strings.Contains(field[len(prefix):], "__") {
				shortField := field[len(prefix):]
				if _, exists := fields[shortField]; !exists {
					fields[shortField] = present
				}
			}
		}
		objects[short] = salesforceSchemaObject{Fields: fields, Relationships: object.Relationships}
		own[short] = true
	}
	return own
}

func loadSalesforceProjectMetadata(p project.Project, cfg *salesforceImportConfig) (schema.Schema, error) {
	var combined schema.Schema
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status != "loaded" || dep.Project == nil {
			continue
		}
		loaded, err := loadSalesforceProjectMetadata(*dep.Project, cfg)
		if err != nil {
			return schema.Schema{}, err
		}
		combined.Objects = append(combined.Objects, loaded.Objects...)
	}
	loaded, err := schema.LoadProject(p)
	if err != nil {
		return schema.Schema{}, err
	}
	// The shared describe registry merges object fragments and supplies implicit
	// fields. Consumer metadata overlays dependency metadata there.
	combined.Objects = append(combined.Objects, loaded.Objects...)
	for _, path := range p.CustomPermissionFiles {
		name := namespaceremap.ApplyMetadataName(p.NamespaceRemaps, strings.TrimSuffix(filepath.Base(path), ".customPermission-meta.xml"))
		cfg.CustomPermissions[name] = true
		if p.Namespace != "" && !strings.Contains(name, "__") {
			cfg.CustomPermissions[p.Namespace+"__"+name] = true
		}
	}
	return combined, nil
}

func salesforceSchemaEntry(definition storage.ObjectDefinition) salesforceSchemaObject {
	entry := salesforceSchemaObject{Fields: map[string]bool{}, Relationships: map[string][]string{}}
	for name, f := range definition.Fields {
		entry.Fields[name] = true
		if f.RelationshipName != "" && len(f.ReferenceTo) > 0 {
			entry.Relationships[f.RelationshipName] = f.ReferenceTo
		}
	}
	return entry
}
