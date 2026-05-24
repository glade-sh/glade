package capability

import (
	"fmt"
	"go/format"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type productNamespaceSymbolSpec struct {
	Name         string
	Kind         string
	SuperClass   string
	Constructors [][]string
	Methods      []productNamespaceMethodSpec
	Properties   []productNamespacePropertySpec
}

type productNamespaceMethodSpec struct {
	Name       string
	ReturnType string
	Parameters []string
	Static     bool
}

type productNamespacePropertySpec struct {
	Name   string
	Type   string
	Static bool
}

func BuildProductNamespaceSymbolSpecs(catalog Catalog, tooling *ToolingCompletions) []productNamespaceSymbolSpec {
	specs := map[string]*productNamespaceSymbolSpec{}
	order := make([]string, 0)
	addSpec := func(name string) *productNamespaceSymbolSpec {
		name = canonicalProductNamespaceTypeName(name)
		if name == "" || !strings.Contains(name, ".") {
			return nil
		}
		key := strings.ToLower(name)
		spec := specs[key]
		if spec == nil {
			spec = &productNamespaceSymbolSpec{Name: name}
			specs[key] = spec
			order = append(order, key)
		}
		return spec
	}

	for _, entry := range catalog.Entries {
		if entry.Area != "Product namespaces" || entry.Namespace == "" || entry.TypeName == "" {
			continue
		}
		spec := addSpec(entry.Namespace + "." + entry.TypeName)
		if spec == nil {
			continue
		}
		if kind := productNamespaceDeclarationKind(entry.Kind); kind != "" && spec.Kind == "" {
			spec.Kind = kind
		}
		if entry.MemberName == "" {
			continue
		}
		memberName := cleanToolingMemberName(entry.MemberName)
		switch strings.ToLower(entry.Kind) {
		case "constructor":
			spec.Constructors = appendUniqueProductNamespaceConstructors(spec.Constructors, [][]string{unknownDocParameters(entry.Signature)})
		case "property", "member":
			spec.Properties = appendUniqueProductNamespaceProperties(spec.Properties, []productNamespacePropertySpec{{
				Name: memberName,
				Type: "Object",
			}})
		case "method":
			spec.Methods = appendUniqueProductNamespaceMethods(spec.Methods, []productNamespaceMethodSpec{{
				Name:       memberName,
				ReturnType: "Object",
				Parameters: unknownDocParameters(entry.Signature),
			}})
		}
	}

	if tooling != nil {
		NormalizeToolingCompletions(tooling)
		for namespace, classes := range tooling.PublicDeclarations {
			canonicalNamespace := canonicalProductNamespaceNamespace(namespace)
			if canonicalNamespace == "" || strings.EqualFold(canonicalNamespace, "System") {
				continue
			}
			for className, decl := range classes {
				spec := addSpec(canonicalNamespace + "." + className)
				if spec == nil {
					continue
				}
				for _, ctor := range decl.Constructors {
					spec.Constructors = appendUniqueProductNamespaceConstructors(spec.Constructors, [][]string{toolingParameterTypes(ctor.Parameters)})
				}
				methods := make([]productNamespaceMethodSpec, 0, len(decl.Methods))
				for _, method := range decl.Methods {
					methods = append(methods, productNamespaceMethodSpec{
						Name:       cleanToolingMemberName(method.Name),
						ReturnType: normalizeProductNamespaceType(method.ReturnType),
						Parameters: toolingMethodParameterTypes(method),
						Static:     method.IsStatic,
					})
				}
				spec.Methods = appendUniqueProductNamespaceMethods(spec.Methods, methods)
				properties := make([]productNamespacePropertySpec, 0, len(decl.Properties))
				for _, prop := range decl.Properties {
					properties = append(properties, productNamespacePropertySpec{
						Name: cleanToolingMemberName(prop.Name),
						Type: normalizeProductNamespaceType(prop.Type),
					})
				}
				spec.Properties = appendUniqueProductNamespaceProperties(spec.Properties, properties)
			}
		}
	}

	out := make([]productNamespaceSymbolSpec, 0, len(order))
	for _, key := range order {
		spec := *specs[key]
		pruneWeakProductNamespaceMembers(&spec)
		sortProductNamespaceSpec(&spec)
		out = append(out, spec)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func WriteProductNamespaceSymbolsGo(w io.Writer, catalog Catalog, tooling *ToolingCompletions) error {
	specs := BuildProductNamespaceSymbolSpecs(catalog, tooling)
	var b strings.Builder
	b.WriteString("package typesys\n\n")
	if productNamespaceSpecsUseApexAST(specs) {
		b.WriteString("import \"github.com/glade-sh/glade/internal/apexast\"\n\n")
	}
	b.WriteString("// Code generated from public Salesforce product namespace declarations. DO NOT EDIT.\n\n")
	b.WriteString("var productNamespaceSymbolSpecs = []StandardSymbolSpec{\n")
	for _, spec := range specs {
		writeProductNamespaceSpecGo(&b, spec)
	}
	b.WriteString("}\n")
	formatted, err := format.Source([]byte(b.String()))
	if err != nil {
		return err
	}
	_, err = w.Write(formatted)
	return err
}

func productNamespaceSpecsUseApexAST(specs []productNamespaceSymbolSpec) bool {
	for _, spec := range specs {
		if spec.Kind != "" {
			return true
		}
	}
	return false
}

func writeProductNamespaceSpecGo(b *strings.Builder, spec productNamespaceSymbolSpec) {
	b.WriteString("{\n")
	fmt.Fprintf(b, "Name: %s,\n", strconv.Quote(spec.Name))
	if spec.Kind != "" {
		fmt.Fprintf(b, "Kind: apexast.%s,\n", spec.Kind)
	}
	if spec.SuperClass != "" {
		fmt.Fprintf(b, "SuperClass: %s,\n", strconv.Quote(spec.SuperClass))
	}
	if len(spec.Constructors) > 0 {
		b.WriteString("Constructors: [][]string{")
		for _, ctor := range spec.Constructors {
			b.WriteString("{")
			for _, param := range ctor {
				fmt.Fprintf(b, "%s,", strconv.Quote(param))
			}
			b.WriteString("},")
		}
		b.WriteString("},\n")
	}
	if len(spec.Methods) > 0 {
		b.WriteString("Methods: []StandardMethodSpec{\n")
		for _, method := range spec.Methods {
			fmt.Fprintf(b, "{Name: %s, ReturnType: %s", strconv.Quote(method.Name), strconv.Quote(method.ReturnType))
			if len(method.Parameters) > 0 {
				b.WriteString(", Parameters: []string{")
				for _, param := range method.Parameters {
					fmt.Fprintf(b, "%s,", strconv.Quote(param))
				}
				b.WriteString("}")
			}
			if method.Static {
				b.WriteString(", Static: true")
			}
			b.WriteString("},\n")
		}
		b.WriteString("},\n")
	}
	if len(spec.Properties) > 0 {
		b.WriteString("Properties: []StandardPropertySpec{\n")
		for _, prop := range spec.Properties {
			fmt.Fprintf(b, "{Name: %s, Type: %s", strconv.Quote(prop.Name), strconv.Quote(prop.Type))
			if prop.Static {
				b.WriteString(", Static: true")
			}
			b.WriteString("},\n")
		}
		b.WriteString("},\n")
	}
	b.WriteString("},\n")
}

func productNamespaceDeclarationKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "enum":
		return "DeclarationEnum"
	case "interface":
		return "DeclarationInterface"
	default:
		return ""
	}
}

func canonicalProductNamespaceTypeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = canonicalProductNamespaceIdentifier(part)
	}
	if len(parts) > 0 {
		parts[0] = canonicalProductNamespaceNamespace(parts[0])
	}
	return strings.Join(parts, ".")
}

func canonicalProductNamespaceNamespace(namespace string) string {
	namespace = canonicalProductNamespaceIdentifier(namespace)
	switch {
	case strings.EqualFold(namespace, "cache"):
		return "Cache"
	default:
		return namespace
	}
}

func canonicalProductNamespaceIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	switch {
	case strings.EqualFold(value, "cache"):
		return "Cache"
	case strings.EqualFold(value, "connectapi"):
		return "ConnectApi"
	default:
		return value
	}
}

func normalizeProductNamespaceType(typ string) string {
	typ = strings.TrimSpace(typ)
	if typ == "" || strings.EqualFold(typ, "APEX_OBJECT") || strings.EqualFold(typ, "any") || strings.EqualFold(typ, "unknown") {
		return "Object"
	}
	typ = normalizeProductNamespaceGenericTypes(typ)
	for _, primitive := range []string{"Blob", "Boolean", "Date", "Datetime", "Decimal", "Double", "Id", "Integer", "Long", "Object", "String", "Time", "Type", "void"} {
		typ = replaceTypeTokenCaseInsensitive(typ, "System."+primitive, primitive)
	}
	typ = replaceTypeTokenCaseInsensitive(typ, "System.Cache", "Cache")
	typ = replaceTypeTokenCaseInsensitive(typ, "cache", "Cache")
	typ = replaceTypeTokenCaseInsensitive(typ, "connectapi", "ConnectApi")
	typ = replaceTypeTokenCaseInsensitive(typ, "APEX_OBJECT", "Object")
	typ = replaceTypeTokenCaseInsensitive(typ, "ANY", "Object")
	typ = replaceTypeTokenCaseInsensitive(typ, "unknown", "Object")
	return typ
}

var productNamespaceGenericRE = regexp.MustCompile(`(?i)\b(list|set|map)\s*<`)

func normalizeProductNamespaceGenericTypes(typ string) string {
	return productNamespaceGenericRE.ReplaceAllStringFunc(typ, func(token string) string {
		switch strings.ToUpper(strings.TrimSuffix(token, "<")) {
		case "LIST":
			return "List<"
		case "SET":
			return "Set<"
		case "MAP":
			return "Map<"
		default:
			return token
		}
	})
}

func replaceTypeTokenCaseInsensitive(value, old, replacement string) string {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(old) + `\b`)
	return re.ReplaceAllString(value, replacement)
}

func toolingParameterTypes(params []ToolingParameter) []string {
	out := make([]string, 0, len(params))
	for _, param := range params {
		out = append(out, normalizeProductNamespaceType(param.Type))
	}
	return out
}

func toolingMethodParameterTypes(method ToolingMethod) []string {
	if len(method.Parameters) > 0 {
		return toolingParameterTypes(method.Parameters)
	}
	out := make([]string, 0, len(method.ArgTypes))
	for _, typ := range method.ArgTypes {
		out = append(out, normalizeProductNamespaceType(typ))
	}
	return out
}

func unknownDocParameters(signature string) []string {
	start := strings.IndexByte(signature, '(')
	end := strings.LastIndexByte(signature, ')')
	if start < 0 || end <= start {
		return nil
	}
	body := strings.TrimSpace(signature[start+1 : end])
	if body == "" {
		return nil
	}
	parts := strings.Split(body, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		out = append(out, "Object")
	}
	return out
}

func appendUniqueProductNamespaceConstructors(values, additions [][]string) [][]string {
	seen := map[string]bool{}
	for _, value := range values {
		seen[productNamespaceTypeListKey(value)] = true
	}
	for _, addition := range additions {
		key := productNamespaceTypeListKey(addition)
		if seen[key] {
			continue
		}
		seen[key] = true
		values = append(values, addition)
	}
	return values
}

func appendUniqueProductNamespaceMethods(values, additions []productNamespaceMethodSpec) []productNamespaceMethodSpec {
	seen := map[string]bool{}
	for _, value := range values {
		seen[productNamespaceMethodKey(value)] = true
	}
	for _, addition := range additions {
		if addition.Name == "" {
			continue
		}
		if addition.ReturnType == "" {
			addition.ReturnType = "Object"
		}
		key := productNamespaceMethodKey(addition)
		if seen[key] {
			continue
		}
		seen[key] = true
		values = append(values, addition)
	}
	return values
}

func appendUniqueProductNamespaceProperties(values, additions []productNamespacePropertySpec) []productNamespacePropertySpec {
	seen := map[string]bool{}
	for i, value := range values {
		seen[productNamespacePropertyKey(value)] = true
		if strings.EqualFold(value.Type, "Object") {
			for _, addition := range additions {
				if strings.EqualFold(value.Name, addition.Name) && !strings.EqualFold(normalizeProductNamespaceType(addition.Type), "Object") {
					values[i] = addition
					break
				}
			}
		}
	}
	for _, addition := range additions {
		if addition.Name == "" {
			continue
		}
		if addition.Type == "" {
			addition.Type = "Object"
		}
		key := productNamespacePropertyKey(addition)
		if seen[key] {
			continue
		}
		seen[key] = true
		values = append(values, addition)
	}
	return values
}

func pruneWeakProductNamespaceMembers(spec *productNamespaceSymbolSpec) {
	strongConstructors := map[int]bool{}
	for _, ctor := range spec.Constructors {
		if !productNamespaceAllObjectTypes(ctor) {
			strongConstructors[len(ctor)] = true
		}
	}
	constructors := spec.Constructors[:0]
	for _, ctor := range spec.Constructors {
		if productNamespaceAllObjectTypes(ctor) && strongConstructors[len(ctor)] {
			continue
		}
		constructors = append(constructors, ctor)
	}
	spec.Constructors = constructors

	strongMethods := map[string]bool{}
	for _, method := range spec.Methods {
		if !strings.EqualFold(method.ReturnType, "Object") || !productNamespaceAllObjectTypes(method.Parameters) {
			strongMethods[strings.ToLower(method.Name)+"|"+strconv.Itoa(len(method.Parameters))] = true
		}
	}
	methods := spec.Methods[:0]
	for _, method := range spec.Methods {
		key := strings.ToLower(method.Name) + "|" + strconv.Itoa(len(method.Parameters))
		if strings.EqualFold(method.ReturnType, "Object") && productNamespaceAllObjectTypes(method.Parameters) && strongMethods[key] {
			continue
		}
		methods = append(methods, method)
	}
	spec.Methods = methods
}

func productNamespaceAllObjectTypes(types []string) bool {
	for _, typ := range types {
		if !strings.EqualFold(typ, "Object") {
			return false
		}
	}
	return true
}

func productNamespaceMethodKey(method productNamespaceMethodSpec) string {
	return strings.ToLower(method.Name) + "|" + strconv.FormatBool(method.Static) + "|" + productNamespaceTypeListKey(method.Parameters)
}

func productNamespacePropertyKey(prop productNamespacePropertySpec) string {
	return strings.ToLower(prop.Name) + "|" + strconv.FormatBool(prop.Static)
}

func productNamespaceTypeListKey(types []string) string {
	normalized := make([]string, 0, len(types))
	for _, typ := range types {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(typ)))
	}
	return strings.Join(normalized, ",")
}

func sortProductNamespaceSpec(spec *productNamespaceSymbolSpec) {
	sort.Slice(spec.Constructors, func(i, j int) bool {
		return productNamespaceTypeListKey(spec.Constructors[i]) < productNamespaceTypeListKey(spec.Constructors[j])
	})
	sort.Slice(spec.Methods, func(i, j int) bool {
		if !strings.EqualFold(spec.Methods[i].Name, spec.Methods[j].Name) {
			return strings.ToLower(spec.Methods[i].Name) < strings.ToLower(spec.Methods[j].Name)
		}
		return productNamespaceMethodKey(spec.Methods[i]) < productNamespaceMethodKey(spec.Methods[j])
	})
	sort.Slice(spec.Properties, func(i, j int) bool {
		return strings.ToLower(spec.Properties[i].Name) < strings.ToLower(spec.Properties[j].Name)
	})
}
