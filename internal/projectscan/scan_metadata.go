package projectscan

import (
	"path/filepath"
	"strings"

	metadatapkg "github.com/glade-sh/glade/internal/metadata"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
)

func addScanMetadataFile(path string, proj *project.Project, seen map[string]bool) {
	clean := filepath.Clean(path)
	if seen[clean] {
		return
	}
	lower := strings.ToLower(filepath.ToSlash(path))
	add := func(target *[]string) {
		*target = append(*target, path)
		seen[clean] = true
	}
	switch {
	case strings.HasSuffix(lower, ".cls"), strings.HasSuffix(lower, ".trigger"):
		add(&proj.ApexFiles)
	case strings.HasSuffix(lower, ".object-meta.xml"), strings.HasSuffix(lower, ".object") && strings.Contains(lower, "/objects/"):
		add(&proj.ObjectFiles)
	case strings.HasSuffix(lower, ".field-meta.xml"):
		add(&proj.FieldFiles)
	case strings.HasSuffix(lower, ".fieldset-meta.xml"):
		add(&proj.FieldSetFiles)
	case strings.HasSuffix(lower, ".recordtype-meta.xml"):
		add(&proj.RecordTypeFiles)
	case strings.HasSuffix(lower, ".validationrule-meta.xml"):
		add(&proj.ValidationRuleFiles)
	case strings.HasSuffix(lower, ".labels"), strings.HasSuffix(lower, ".labels-meta.xml"):
		add(&proj.LabelFiles)
	case strings.HasSuffix(lower, ".translation"), strings.HasSuffix(lower, ".translation-meta.xml"):
		add(&proj.TranslationFiles)
	case strings.HasSuffix(lower, ".resource-meta.xml"), strings.HasSuffix(lower, ".staticresource-meta.xml"):
		add(&proj.StaticResourceMetas)
	case strings.HasSuffix(lower, ".resource"):
		add(&proj.StaticResourceFiles)
	case strings.HasSuffix(lower, ".asset-meta.xml"):
		add(&proj.ContentAssetMetas)
	case strings.HasSuffix(lower, ".asset"):
		add(&proj.ContentAssetFiles)
	case strings.HasSuffix(lower, ".email"), strings.HasSuffix(lower, ".email-meta.xml"):
		add(&proj.EmailTemplateFiles)
	case strings.HasSuffix(lower, ".namedcredential"), strings.HasSuffix(lower, ".namedcredential-meta.xml"):
		add(&proj.NamedCredentialFiles)
	case strings.HasSuffix(lower, ".remotesite"), strings.HasSuffix(lower, ".remotesite-meta.xml"):
		add(&proj.RemoteSiteFiles)
	case strings.HasSuffix(lower, ".md-meta.xml"), strings.HasSuffix(lower, ".md") && isCustomMetadataPath(lower):
		add(&proj.CustomMetadataFiles)
	case strings.HasSuffix(lower, ".workflow-meta.xml"), strings.HasSuffix(lower, ".workflow"):
		add(&proj.WorkflowFiles)
	case strings.HasSuffix(lower, ".flow-meta.xml"), strings.HasSuffix(lower, ".flow"):
		add(&proj.FlowFiles)
	case strings.HasSuffix(lower, ".profile"), strings.HasSuffix(lower, ".profile-meta.xml"):
		add(&proj.ProfileFiles)
	case strings.HasSuffix(lower, ".permissionset"), strings.HasSuffix(lower, ".permissionset-meta.xml"):
		add(&proj.PermissionSetFiles)
	case strings.HasSuffix(lower, ".permissionsetassignment"), strings.HasSuffix(lower, ".permissionsetassignment-meta.xml"):
		add(&proj.PermissionAssignmentFiles)
	case strings.HasSuffix(lower, ".listview-meta.xml"):
		add(&proj.ListViewFiles)
	case strings.HasSuffix(lower, ".layout-meta.xml"), strings.HasSuffix(lower, ".layout"):
		add(&proj.LayoutFiles)
	case strings.HasSuffix(lower, ".compactlayout-meta.xml"):
		add(&proj.CompactLayoutFiles)
	case strings.HasSuffix(lower, ".tab"), strings.HasSuffix(lower, ".tab-meta.xml"):
		add(&proj.TabFiles)
	case strings.HasSuffix(lower, ".weblink"), strings.HasSuffix(lower, ".weblink-meta.xml"):
		add(&proj.WebLinkFiles)
	case strings.HasSuffix(lower, ".quickaction"), strings.HasSuffix(lower, ".quickaction-meta.xml"):
		add(&proj.QuickActionFiles)
	case strings.HasSuffix(lower, ".globalvalueset"), strings.HasSuffix(lower, ".globalvalueset-meta.xml"):
		add(&proj.GlobalValueSetFiles)
	case strings.HasSuffix(lower, ".standardvalueset"), strings.HasSuffix(lower, ".standardvalueset-meta.xml"):
		add(&proj.StandardValueSetFiles)
	case strings.HasSuffix(lower, ".flexipage"), strings.HasSuffix(lower, ".flexipage-meta.xml"):
		add(&proj.FlexiPageFiles)
	case strings.HasSuffix(lower, ".app-meta.xml"), strings.HasSuffix(lower, ".app") && strings.Contains(lower, "/applications/"):
		add(&proj.ApplicationFiles)
	case strings.HasSuffix(lower, ".page"), strings.HasSuffix(lower, ".page-meta.xml"):
		add(&proj.VisualforcePageFiles)
	case strings.HasSuffix(lower, ".component"):
		add(&proj.VisualforceComponentFiles)
	case strings.Contains(lower, "/aura/") && isAuraSourceFile(lower):
		add(&proj.AuraFiles)
	case strings.Contains(lower, "/lwc/") && strings.HasSuffix(lower, ".js"):
		add(&proj.LWCFiles)
	}
}

func isAuraSourceFile(path string) bool {
	return hasAnySuffix(path,
		".cmp", ".cmp-meta.xml",
		".app", ".app-meta.xml",
		".evt", ".evt-meta.xml",
		".intf", ".intf-meta.xml",
		".design", ".design-meta.xml",
		".js", ".css", ".svg", ".auradoc",
	)
}

func (ctx *scanContext) addPresentationAssetFiles(assets []metadatapkg.NamedAsset) {
	for _, asset := range assets {
		ctx.present[filepath.Clean(asset.File)] = true
	}
}
func (ctx *scanContext) addPresentationFields(idx metadatapkg.Index, proj project.Project) {
	ctx.fieldSets = make(map[string]bool, len(idx.FieldSets))
	add := func(objectName, fieldName string) {
		objectName = strings.TrimSpace(objectName)
		fieldName = strings.TrimSpace(fieldName)
		if objectName == "" || fieldName == "" {
			return
		}
		ctx.addPresentationFieldPath(objectName, fieldName)
		if strings.Contains(fieldName, ".") {
			return
		}
		resolved, ok := storage.ResolveObjectName(ctx.org, objectName)
		if !ok {
			return
		}
		state := ctx.org.Objects[resolved]
		if state.Definition.Fields == nil {
			state.Definition.Fields = make(map[string]storage.Field)
		}
		if _, ok := state.Definition.Fields[fieldName]; !ok {
			state.Definition.Fields[fieldName] = storage.Field{APIName: fieldName, Type: storage.FieldAny}
			ctx.org.Objects[resolved] = state
		}
	}
	for _, fieldSet := range idx.FieldSets {
		ctx.fieldSets[schemaPathKey([]string{fieldSet.ObjectName, fieldSet.Name})] = true
		if stripped := stripAnyNamespaceToken(fieldSet.ObjectName); stripped != fieldSet.ObjectName {
			ctx.fieldSets[schemaPathKey([]string{stripped, fieldSet.Name})] = true
		}
		for _, member := range fieldSet.Fields {
			add(fieldSet.ObjectName, member.Field)
		}
	}
	for _, path := range proj.CompactLayoutFiles {
		objectName := objectNameFromObjectMetadataPath(path, "compactLayouts")
		for _, fieldName := range loadPresentationFields(path) {
			add(objectName, fieldName)
		}
	}
	for _, path := range proj.LayoutFiles {
		objectName := objectNameFromLayoutPath(path)
		for _, fieldName := range loadPresentationFields(path) {
			add(objectName, fieldName)
		}
	}
	for _, path := range proj.QuickActionFiles {
		objectName := objectNameFromQuickActionPath(path)
		fields, target := loadQuickActionPresentationFields(path)
		if target != "" {
			objectName = target
		}
		for _, fieldName := range fields {
			add(objectName, fieldName)
		}
	}
}
func (ctx *scanContext) addPresentationFieldPath(objectName, fieldPath string) {
	if ctx.presentationPaths == nil {
		ctx.presentationPaths = make(map[string]bool)
	}
	parts := schemaReferenceTokens(objectName + "." + fieldPath)
	if len(parts) < 2 {
		return
	}
	ctx.presentationPaths[schemaPathKey(parts)] = true
	if stripped := stripAnyNamespaceToken(parts[0]); stripped != parts[0] {
		alt := append([]string{stripped}, parts[1:]...)
		ctx.presentationPaths[schemaPathKey(alt)] = true
	}
}
func classifyByPath(rel, path string, ctx *scanContext) []Finding {
	lower := strings.ToLower(rel)
	var findings []Finding
	add := func(capability, metadataType, symbol string) {
		findings = append(findings, makeFinding(capability, rel, 0, metadataType, symbol, "metadata file"))
	}

	switch {
	case strings.HasSuffix(lower, ".component"):
		if ctx != nil && ctx.resolvesVisualforceComponentMetadata(path) {
			return findings
		}
		add("visualforce.component-test", "VisualforceComponent", baseNoExt(path))
	case strings.Contains(lower, "/aura/"):
		if isPassiveAuraArtifact(lower) {
			return findings
		}
		if ctx != nil && ctx.hasAuraActionMetadata(path) {
			add("aura.action-metadata", "AuraBundle", auraOrLWCBundle(rel, "aura"))
			return findings
		}
		if ctx != nil && ctx.resolvesAuraFile(path) {
			return findings
		}
		add("aura.controller-test", "AuraBundle", auraOrLWCBundle(rel, "aura"))
	case strings.HasSuffix(lower, ".workflow-meta.xml"), strings.HasSuffix(lower, ".workflow"):
		if ctx != nil && ctx.resolvesAutomationFile(path) {
			return findings
		}
		add("workflow.save-order", "Workflow", baseNoExt(path))
	case strings.HasSuffix(lower, ".flow-meta.xml"), strings.HasSuffix(lower, ".flow"):
		if ctx != nil && ctx.resolvesAutomationFile(path) {
			return findings
		}
		add("flow.save-order", "Flow", baseNoExt(path))
	case strings.HasSuffix(lower, ".email"), strings.HasSuffix(lower, ".email-meta.xml"):
		if ctx != nil && ctx.resolvesEmailTemplate(baseNoExt(path), path) {
			return findings
		}
		add("email.templates", "EmailTemplate", baseNoExt(path))
	case strings.HasSuffix(lower, ".object"):
		if ctx != nil && ctx.loadedFiles[filepath.Clean(path)] {
			return findings
		}
		add("metadata.legacy-source", "LegacyObject", baseNoExt(path))
	case hasAnySuffix(lower, ".layout", ".layout-meta.xml", ".profile", ".profile-meta.xml", ".permissionset", ".permissionset-meta.xml", ".tab", ".tab-meta.xml", ".weblink", ".weblink-meta.xml", ".quickaction-meta.xml", ".globalvalueset-meta.xml", ".standardvalueset-meta.xml", ".flexipage", ".flexipage-meta.xml", ".application", ".app-meta.xml"):
		if ctx != nil && ctx.present[filepath.Clean(path)] {
			return findings
		}
		add("ui.presentation-metadata", "UIPresentationMetadata", baseNoExt(path))
	case isReportMetadataPath(lower):
		return findings
	case isDashboardMetadataPath(lower):
		return findings
	}
	return findings
}
func namespaceAliases(proj project.Project, sch schema.Schema, idx *metadatapkg.Index) map[string]bool {
	aliases := make(map[string]bool)
	add := func(name string) {
		token := namespaceToken(name)
		if token != "" {
			aliases[strings.ToLower(token)] = true
		}
		prefix := metadataNamespacePrefix(name)
		if prefix != "" {
			aliases[strings.ToLower(prefix)] = true
		}
	}
	add(proj.Namespace)
	for _, path := range proj.ApexFiles {
		add(baseNoExt(path))
	}
	for _, path := range proj.StaticResourceFiles {
		add(baseNoExt(path))
	}
	for _, path := range proj.StaticResourceMetas {
		add(baseNoExt(path))
	}
	for _, path := range proj.VisualforcePageFiles {
		add(baseNoExt(path))
	}
	for _, path := range proj.VisualforceComponentFiles {
		add(baseNoExt(path))
	}
	for _, object := range sch.Objects {
		add(object.Name)
		for _, field := range object.Fields {
			add(field.Name)
			add(field.RelationshipName)
			for _, target := range field.ReferenceTo {
				add(target)
			}
		}
	}
	for _, record := range sch.CustomMetadataRecords {
		add(record.FullName)
		add(record.ObjectName)
		for _, value := range record.Values {
			add(value.Field)
			add(value.Value)
		}
	}
	if idx != nil {
		for _, record := range idx.CustomMetadataRecords {
			add(record.FullName)
			add(record.ObjectName)
			for _, value := range record.Values {
				add(value.Field)
				add(value.Value)
			}
		}
		for _, fieldSet := range idx.FieldSets {
			add(fieldSet.ObjectName)
			for _, field := range fieldSet.Fields {
				add(field.Field)
			}
		}
		for _, profile := range idx.Profiles {
			for _, permission := range profile.ObjectPermissions {
				add(permission.Object)
			}
			for _, permission := range profile.FieldPermissions {
				add(permission.Field)
			}
		}
		for _, permissionSet := range idx.PermissionSets {
			for _, permission := range permissionSet.ObjectPermissions {
				add(permission.Object)
			}
			for _, permission := range permissionSet.FieldPermissions {
				add(permission.Field)
			}
		}
	}
	if len(aliases) == 0 {
		return nil
	}
	return aliases
}
