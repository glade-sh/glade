package visualforce

import (
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"golang.org/x/net/html"
)

var remoteObjectJSReferenceRE = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$]*(\.[A-Za-z_$][A-Za-z0-9_$]*)*$`)

// Native e_annotation_* captures reject the dependent page when a remoting
// declaration fails. Only declaration errors on RemoteAction members enter
// this path; errors in unrelated methods and method bodies keep their existing
// controller-loading behavior.
func isRemotingControllerDiagnostic(index typesys.Index, diag diagnostic.Diagnostic) bool {
	if diag.Severity != diagnostic.Error || diag.Range == nil {
		return false
	}
	switch diag.Code {
	case "GLADESEMA002", "GLADESEMA031", "GLADESEMA032":
	default:
		return false
	}
	for _, typ := range index.Types {
		if typ.Dependency || filepath.Clean(typ.File) != filepath.Clean(diag.File) {
			continue
		}
		for _, member := range typ.Members {
			atDeclaration := member.Range.Start == diag.Range.Start
			for _, annotation := range member.Annotations {
				atDeclaration = atDeclaration || annotation.Range.Start == diag.Range.Start
			}
			if !atDeclaration {
				continue
			}
			if remotingMemberAnnotated(member) {
				return true
			}
			// A duplicate method may be unannotated even though the original
			// declaration was remoted (e_annotation_duplicate_method).
			if diag.Code == "GLADESEMA031" && member.Kind == apexast.DeclarationMethod {
				for _, original := range typ.Members {
					if remotingMemberAnnotated(original) && remotingMethodSignatureEqual(original, member) {
						return true
					}
				}
			}
		}
	}
	return false
}

func remotingMemberAnnotated(member typesys.MemberSymbol) bool {
	if hasRemoteActionAnnotation(member.Modifiers) {
		return true
	}
	for _, annotation := range member.Annotations {
		if strings.EqualFold(annotation.Name, "RemoteAction") {
			return true
		}
	}
	return false
}

func remotingMethodSignatureEqual(a, b typesys.MemberSymbol) bool {
	if a.Kind != apexast.DeclarationMethod || b.Kind != apexast.DeclarationMethod || !strings.EqualFold(a.Name, b.Name) || len(a.Parameters) != len(b.Parameters) {
		return false
	}
	for i := range a.Parameters {
		if !strings.EqualFold(a.Parameters[i].Type, b.Parameters[i].Type) {
			return false
		}
	}
	return true
}

func (ctx *expressionValidationContext) validateRemoteActionReference(parts []string) error {
	// Only a complete Class.method reference is checked here. The caller keeps
	// references accepted when controller source is supplied later by the VM.
	if len(parts) != 3 {
		return nil
	}
	className, methodName := parts[1], parts[2]
	seen, methodFound := map[string]bool{}, false
	for name := className; name != ""; {
		key := strings.ToLower(name)
		class, known := ctx.classes[key]
		if !known || seen[key] {
			break
		}
		seen[key] = true
		for _, member := range class.Members {
			if member.Kind != apexast.DeclarationMethod || !strings.EqualFold(member.Name, methodName) {
				continue
			}
			methodFound = true
			annotations := append([]string(nil), member.Modifiers...)
			for _, annotation := range member.Annotations {
				annotations = append(annotations, annotation.Name)
			}
			if ValidateRemoteActionExposure(RemoteActionMethod{
				ClassName: className, MethodName: methodName,
				Annotations: annotations, Modifiers: member.Modifiers,
			}) == nil {
				return nil
			}
		}
		name = class.SuperClass
	}
	if methodFound {
		return fmt.Errorf("No remoted actions found to resolve '%s'", strings.Join(parts, "."))
	}
	return fmt.Errorf("No remoted actions found for '%s.%s'.", className, methodName)
}

// Source validation retains attribute presence and the native diagnostic
// location immediately after the opening tag.
func validateRemoteObjectsSource(source, sourceName string) error {
	z := html.NewTokenizer(strings.NewReader(source))
	z.AllowCDATA(true)
	offset := 0
	for {
		kind := z.Next()
		offset += len(z.Raw())
		if kind == html.ErrorToken {
			if z.Err() == io.EOF {
				return nil
			}
			return z.Err()
		}
		if kind != html.StartTagToken && kind != html.SelfClosingTagToken {
			continue
		}
		token := z.Token()
		if kind == html.SelfClosingTagToken {
			z.NextIsNotRawText()
		}
		var tag string
		switch strings.ToLower(token.Data) {
		case "apex:remoteobjects":
			tag = "remoteObjects"
		case "apex:remoteobjectmodel":
			tag = "remoteObjectModel"
		case "apex:remoteobjectfield":
			tag = "remoteObjectField"
		default:
			continue
		}
		line, column := lineColumnAt(source, offset)
		location := fmt.Sprintf(" at line %d column %d", line, column)
		if sourceName != "" {
			location = " in " + sourceName + location
		}
		values := map[string]string{}
		for _, attr := range token.Attr {
			values[strings.ToLower(attr.Key)] = attr.Val
		}
		if tag != "remoteObjects" {
			if _, present := values["name"]; !present {
				return fmt.Errorf("Missing required attribute name in <apex:%s>%s", tag, location)
			}
		}
		spec, _ := StandardComponentSpec("apex", tag)
		for _, attr := range token.Attr {
			supported := false
			for _, name := range spec.Attributes {
				supported = supported || strings.EqualFold(name, attr.Key)
			}
			if !supported {
				return fmt.Errorf("Unsupported attribute %s in <apex:%s>%s", strings.ToLower(attr.Key), tag, location)
			}
		}
		if namespace, present := values["jsnamespace"]; tag == "remoteObjects" && present && !strings.Contains(namespace, "{!") && !remoteObjectJSReferenceRE.MatchString(namespace) {
			message := `Wrong type for attribute <apex:remoteObjects jsNamespace="">. Expected valid javascript reference name, found`
			if namespace != "" {
				message += " " + namespace
			}
			return fmt.Errorf("%s", message)
		}
	}
}

func validateRemoteObjectDeclarations(root *MarkupNode, p project.Project) error {
	if findVisualforceComponent(root, "apex", "remoteobjectmodel") == nil && findVisualforceComponent(root, "apex", "remoteobjectfield") == nil {
		return nil
	}
	metadata, err := schema.LoadProject(p)
	if err != nil {
		return err
	}
	declared := RemoteObjectSchema{}
	for _, object := range metadata.Objects {
		definition := storage.ObjectDefinition{APIName: object.Name, SharingModel: object.SharingModel, Fields: map[string]storage.Field{}}
		if object.NameField.Type != "" {
			definition.Fields["Name"] = storage.Field{APIName: "Name"}
		}
		for _, field := range object.Fields {
			definition.Fields[field.Name] = storage.Field{APIName: field.Name}
		}
		storage.EnsureStandardObjectFields(&definition)
		fields := make([]string, 0, len(definition.Fields))
		for fieldName, field := range definition.Fields {
			fields = append(fields, firstNonEmpty(field.APIName, fieldName))
		}
		declared[object.Name] = fields
	}
	var visit func(*MarkupNode, *MarkupNode) error
	visit = func(node, parent *MarkupNode) error {
		if node == nil {
			return nil
		}
		switch {
		case isVisualforceComponent(node, "apex", "remoteobjectmodel"):
			if !isVisualforceComponent(parent, "apex", "remoteobjects") {
				return fmt.Errorf("<apex:remoteObjectModel> must be a direct child of one of the following: <apex:remoteObjects>")
			}
			name := strings.TrimSpace(node.Attribute("name"))
			if _, known := remoteObjectSchemaFields(declared, name); !known {
				if definition, found := storage.StandardObjectDefinition(name); found {
					fields := make([]string, 0, len(definition.Fields))
					for fieldName, field := range definition.Fields {
						fields = append(fields, firstNonEmpty(field.APIName, fieldName))
					}
					declared[name] = fields
				}
			}
			if _, err := buildRemoteObjectModel(node, declared); err != nil {
				return err
			}
		case isVisualforceComponent(node, "apex", "remoteobjectfield"):
			if !isVisualforceComponent(parent, "apex", "remoteobjectmodel") {
				return fmt.Errorf("<apex:remoteObjectField> must be a direct child of one of the following: <apex:remoteObjectModel>")
			}
		}
		for _, child := range node.Children {
			if err := visit(child, node); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(root, nil)
}
