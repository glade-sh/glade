package uicontroller

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/typesys"
)

type Index struct {
	AuraBundles []AuraBundle          `json:"auraBundles,omitempty"`
	LWCBundles  []LWCBundle           `json:"lwcBundles,omitempty"`
	ApexMethods []ApexMethodReference `json:"apexMethods,omitempty"`
}

type AuraBundle struct {
	Name                 string                `json:"name"`
	Dir                  string                `json:"dir"`
	Files                []UIFile              `json:"files,omitempty"`
	ControllerReferences []ControllerReference `json:"controllerReferences,omitempty"`
	ComponentReferences  []AuraComponentRef    `json:"componentReferences,omitempty"`
	ActionReferences     []AuraActionReference `json:"actionReferences,omitempty"`
}

type LWCBundle struct {
	Name    string      `json:"name"`
	Dir     string      `json:"dir"`
	Files   []UIFile    `json:"files,omitempty"`
	Imports []LWCImport `json:"imports,omitempty"`
	Wires   []LWCWire   `json:"wires,omitempty"`
}

type UIFile struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type SourceRef struct {
	File string `json:"file"`
	Line int    `json:"line,omitempty"`
}

type ControllerReference struct {
	Name string `json:"name"`
	SourceRef
}

type AuraComponentRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	SourceRef
}

type AuraActionReference struct {
	Name       string              `json:"name"`
	ClassName  string              `json:"className,omitempty"`
	Resolved   bool                `json:"resolved,omitempty"`
	ReturnType string              `json:"returnType,omitempty"`
	Parameters []apexast.Parameter `json:"parameters,omitempty"`
	SourceRef
}

type LWCImport struct {
	LocalName     string `json:"localName,omitempty"`
	Source        string `json:"source"`
	Kind          string `json:"kind"`
	ClassName     string `json:"className,omitempty"`
	MethodName    string `json:"methodName,omitempty"`
	LabelName     string `json:"labelName,omitempty"`
	ResourceName  string `json:"resourceName,omitempty"`
	SchemaName    string `json:"schemaName,omitempty"`
	Module        string `json:"module,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	ComponentName string `json:"componentName,omitempty"`
	SourceRef
}

type LWCWire struct {
	Adapter            string   `json:"adapter"`
	AdapterKind        string   `json:"adapterKind,omitempty"`
	ApexClassName      string   `json:"apexClassName,omitempty"`
	ApexMethodName     string   `json:"apexMethodName,omitempty"`
	Target             string   `json:"target,omitempty"`
	ReactiveParameters []string `json:"reactiveParameters,omitempty"`
	SourceRef
}

type ApexMethodReference struct {
	Framework  string              `json:"framework"`
	ClassName  string              `json:"className"`
	MethodName string              `json:"methodName"`
	Resolved   bool                `json:"resolved,omitempty"`
	ReturnType string              `json:"returnType,omitempty"`
	Parameters []apexast.Parameter `json:"parameters,omitempty"`
	SourceRef
}

var (
	auraControllerAttrRe = regexp.MustCompile(`(?i)\bcontroller\s*=\s*["']([A-Za-z_][A-Za-z0-9_.]*)["']`)
	auraComponentTagRe   = regexp.MustCompile(`<\s*([A-Za-z_][A-Za-z0-9_]*)\s*:\s*([A-Za-z_][A-Za-z0-9_-]*)`)
	auraMarkupActionRe   = regexp.MustCompile(`\{!\s*c\.([A-Za-z_][A-Za-z0-9_]*)`)
	auraJSActionRe       = regexp.MustCompile(`\b(?:component|cmp)\s*\.\s*get\s*\(\s*["']c\.([A-Za-z_][A-Za-z0-9_]*)["']\s*\)`)
	lwcImportRe          = regexp.MustCompile(`(?m)^\s*import\s+(.+?)\s+from\s+["']([^"']+)["']`)
	lwcWireRe            = regexp.MustCompile(`(?s)@wire\s*\(\s*([A-Za-z_][A-Za-z0-9_]*)\s*(?:,\s*(\{.*?\}))?\s*\)\s*([A-Za-z_][A-Za-z0-9_]*)?`)
	reactiveParamRe      = regexp.MustCompile(`["']\$([A-Za-z_][A-Za-z0-9_.]*)["']`)
)

func Build(p project.Project, apex typesys.Index) (Index, error) {
	idx := Index{}
	for _, group := range groupByBundle(p.AuraFiles) {
		bundle, err := parseAuraBundle(group, apex)
		if err != nil {
			return Index{}, err
		}
		idx.AuraBundles = append(idx.AuraBundles, bundle)
		for _, action := range bundle.ActionReferences {
			if action.ClassName == "" {
				continue
			}
			idx.ApexMethods = append(idx.ApexMethods, ApexMethodReference{
				Framework:  "aura",
				ClassName:  action.ClassName,
				MethodName: action.Name,
				Resolved:   action.Resolved,
				ReturnType: action.ReturnType,
				Parameters: action.Parameters,
				SourceRef:  action.SourceRef,
			})
		}
	}
	for _, group := range groupByBundle(p.LWCFiles) {
		bundle, err := parseLWCBundle(group)
		if err != nil {
			return Index{}, err
		}
		idx.LWCBundles = append(idx.LWCBundles, bundle)
		for _, imp := range bundle.Imports {
			if imp.Kind != "apex" {
				continue
			}
			resolved, returnType, params := resolveApex(apex, imp.ClassName, imp.MethodName)
			idx.ApexMethods = append(idx.ApexMethods, ApexMethodReference{
				Framework:  "lwc",
				ClassName:  imp.ClassName,
				MethodName: imp.MethodName,
				Resolved:   resolved,
				ReturnType: returnType,
				Parameters: params,
				SourceRef:  imp.SourceRef,
			})
		}
	}
	sort.Slice(idx.AuraBundles, func(i, j int) bool { return idx.AuraBundles[i].Name < idx.AuraBundles[j].Name })
	sort.Slice(idx.LWCBundles, func(i, j int) bool { return idx.LWCBundles[i].Name < idx.LWCBundles[j].Name })
	sort.Slice(idx.ApexMethods, func(i, j int) bool {
		if idx.ApexMethods[i].ClassName == idx.ApexMethods[j].ClassName {
			return idx.ApexMethods[i].MethodName < idx.ApexMethods[j].MethodName
		}
		return idx.ApexMethods[i].ClassName < idx.ApexMethods[j].ClassName
	})
	return idx, nil
}

func parseAuraBundle(paths []string, apex typesys.Index) (AuraBundle, error) {
	bundle := AuraBundle{Name: filepath.Base(filepath.Dir(paths[0])), Dir: filepath.Dir(paths[0])}
	controllers := make(map[string]ControllerReference)
	components := make(map[string]AuraComponentRef)
	actions := make(map[string]AuraActionReference)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return AuraBundle{}, err
		}
		source := string(data)
		bundle.Files = append(bundle.Files, UIFile{Path: path, Kind: auraFileKind(path)})
		for _, match := range auraControllerAttrRe.FindAllStringSubmatchIndex(source, -1) {
			name := source[match[2]:match[3]]
			controllers[lookupKey(name)] = ControllerReference{Name: name, SourceRef: SourceRef{File: path, Line: lineAt(source, match[0])}}
		}
		for _, match := range auraComponentTagRe.FindAllStringSubmatchIndex(source, -1) {
			ns := source[match[2]:match[3]]
			name := source[match[4]:match[5]]
			if strings.EqualFold(ns, "aura") {
				continue
			}
			key := lookupKey(ns + ":" + name)
			components[key] = AuraComponentRef{Namespace: ns, Name: name, SourceRef: SourceRef{File: path, Line: lineAt(source, match[0])}}
		}
		for _, match := range auraMarkupActionRe.FindAllStringSubmatchIndex(source, -1) {
			name := source[match[2]:match[3]]
			actions[actionKey(path, name, match[0])] = AuraActionReference{Name: name, SourceRef: SourceRef{File: path, Line: lineAt(source, match[0])}}
		}
		if strings.HasSuffix(strings.ToLower(path), ".js") {
			for _, match := range auraJSActionRe.FindAllStringSubmatchIndex(source, -1) {
				name := source[match[2]:match[3]]
				actions[actionKey(path, name, match[0])] = AuraActionReference{Name: name, SourceRef: SourceRef{File: path, Line: lineAt(source, match[0])}}
			}
		}
	}
	controllerName := firstControllerName(controllers)
	bundle.ControllerReferences = sortedControllers(controllers)
	bundle.ComponentReferences = sortedComponents(components)
	for _, action := range actions {
		action.ClassName = controllerName
		action.Resolved, action.ReturnType, action.Parameters = resolveApex(apex, action.ClassName, action.Name)
		bundle.ActionReferences = append(bundle.ActionReferences, action)
	}
	sort.Slice(bundle.ActionReferences, func(i, j int) bool {
		if bundle.ActionReferences[i].File == bundle.ActionReferences[j].File {
			return bundle.ActionReferences[i].Line < bundle.ActionReferences[j].Line
		}
		return bundle.ActionReferences[i].File < bundle.ActionReferences[j].File
	})
	sort.Slice(bundle.Files, func(i, j int) bool { return bundle.Files[i].Path < bundle.Files[j].Path })
	return bundle, nil
}

func parseLWCBundle(paths []string) (LWCBundle, error) {
	bundle := LWCBundle{Name: filepath.Base(filepath.Dir(paths[0])), Dir: filepath.Dir(paths[0])}
	importsByLocal := make(map[string]LWCImport)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return LWCBundle{}, err
		}
		source := string(data)
		bundle.Files = append(bundle.Files, UIFile{Path: path, Kind: "javascript"})
		for _, imp := range parseLWCImports(path, source) {
			bundle.Imports = append(bundle.Imports, imp)
			if imp.LocalName != "" {
				importsByLocal[imp.LocalName] = imp
			}
		}
		bundle.Wires = append(bundle.Wires, parseWires(path, source, importsByLocal)...)
	}
	sort.Slice(bundle.Files, func(i, j int) bool { return bundle.Files[i].Path < bundle.Files[j].Path })
	sort.Slice(bundle.Imports, func(i, j int) bool {
		if bundle.Imports[i].File == bundle.Imports[j].File {
			return bundle.Imports[i].Line < bundle.Imports[j].Line
		}
		return bundle.Imports[i].File < bundle.Imports[j].File
	})
	sort.Slice(bundle.Wires, func(i, j int) bool {
		if bundle.Wires[i].File == bundle.Wires[j].File {
			return bundle.Wires[i].Line < bundle.Wires[j].Line
		}
		return bundle.Wires[i].File < bundle.Wires[j].File
	})
	return bundle, nil
}

func parseLWCImports(path, source string) []LWCImport {
	var imports []LWCImport
	for _, match := range lwcImportRe.FindAllStringSubmatchIndex(source, -1) {
		spec := strings.TrimSpace(source[match[2]:match[3]])
		module := strings.TrimSpace(source[match[4]:match[5]])
		localNames := importLocalNames(spec)
		if len(localNames) == 0 {
			localNames = []string{""}
		}
		for _, local := range localNames {
			imp := classifyLWCImport(module)
			imp.LocalName = local
			imp.Source = module
			imp.SourceRef = SourceRef{File: path, Line: lineAt(source, match[0])}
			imports = append(imports, imp)
		}
	}
	return imports
}

func parseWires(path, source string, importsByLocal map[string]LWCImport) []LWCWire {
	var wires []LWCWire
	for _, match := range lwcWireRe.FindAllStringSubmatchIndex(source, -1) {
		adapter := source[match[2]:match[3]]
		wire := LWCWire{Adapter: adapter, SourceRef: SourceRef{File: path, Line: lineAt(source, match[0])}}
		if match[6] >= 0 {
			wire.Target = strings.TrimSpace(source[match[6]:match[7]])
		}
		if imp, ok := importsByLocal[adapter]; ok {
			wire.AdapterKind = imp.Kind
			wire.ApexClassName = imp.ClassName
			wire.ApexMethodName = imp.MethodName
		}
		if match[4] >= 0 {
			params := source[match[4]:match[5]]
			seen := make(map[string]bool)
			for _, paramMatch := range reactiveParamRe.FindAllStringSubmatch(params, -1) {
				if !seen[paramMatch[1]] {
					wire.ReactiveParameters = append(wire.ReactiveParameters, paramMatch[1])
					seen[paramMatch[1]] = true
				}
			}
		}
		wires = append(wires, wire)
	}
	return wires
}

func classifyLWCImport(module string) LWCImport {
	switch {
	case strings.HasPrefix(module, "@salesforce/apex/"):
		parts := strings.SplitN(strings.TrimPrefix(module, "@salesforce/apex/"), ".", 2)
		imp := LWCImport{Kind: "apex"}
		if len(parts) == 2 {
			imp.ClassName, imp.MethodName = parts[0], parts[1]
		}
		return imp
	case strings.HasPrefix(module, "@salesforce/label/"):
		return LWCImport{Kind: "label", LabelName: strings.TrimPrefix(module, "@salesforce/label/")}
	case strings.HasPrefix(module, "@salesforce/resourceUrl/"):
		return LWCImport{Kind: "resourceUrl", ResourceName: strings.TrimPrefix(module, "@salesforce/resourceUrl/")}
	case strings.HasPrefix(module, "@salesforce/schema/"):
		return LWCImport{Kind: "schema", SchemaName: strings.TrimPrefix(module, "@salesforce/schema/")}
	case module == "lightning/navigation", module == "lightning/uiRecordApi", module == "lightning/uiObjectInfoApi":
		return LWCImport{Kind: "lightning", Module: module}
	case strings.HasPrefix(module, "c/"):
		return LWCImport{Kind: "local", Namespace: "c", ComponentName: strings.TrimPrefix(module, "c/")}
	default:
		return LWCImport{Kind: "module", Module: module}
	}
}

func importLocalNames(spec string) []string {
	spec = strings.TrimSpace(spec)
	if strings.HasPrefix(spec, "{") && strings.HasSuffix(spec, "}") {
		spec = strings.TrimPrefix(strings.TrimSuffix(spec, "}"), "{")
		var names []string
		for _, part := range strings.Split(spec, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			fields := strings.Fields(part)
			if len(fields) == 3 && strings.EqualFold(fields[1], "as") {
				names = append(names, fields[2])
			} else {
				names = append(names, fields[0])
			}
		}
		return names
	}
	fields := strings.Fields(spec)
	if len(fields) == 0 {
		return nil
	}
	return []string{fields[0]}
}

func resolveApex(idx typesys.Index, className, methodName string) (bool, string, []apexast.Parameter) {
	if className == "" || methodName == "" {
		return false, "", nil
	}
	for _, typ := range idx.Types {
		if !strings.EqualFold(typ.Name, className) {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind == apexast.DeclarationMethod && strings.EqualFold(member.Name, methodName) {
				return true, member.Type, member.Parameters
			}
		}
	}
	return false, "", nil
}

func groupByBundle(paths []string) [][]string {
	byDir := make(map[string][]string)
	for _, path := range paths {
		byDir[filepath.Dir(path)] = append(byDir[filepath.Dir(path)], path)
	}
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	out := make([][]string, 0, len(dirs))
	for _, dir := range dirs {
		sort.Strings(byDir[dir])
		out = append(out, byDir[dir])
	}
	return out
}

func firstControllerName(in map[string]ControllerReference) string {
	names := sortedControllers(in)
	if len(names) == 0 {
		return ""
	}
	return names[0].Name
}

func sortedControllers(in map[string]ControllerReference) []ControllerReference {
	out := make([]ControllerReference, 0, len(in))
	for _, item := range in {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedComponents(in map[string]AuraComponentRef) []AuraComponentRef {
	out := make([]AuraComponentRef, 0, len(in))
	for _, item := range in {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}

func auraFileKind(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".cmp":
		return "component"
	case ".app":
		return "application"
	case ".evt":
		return "event"
	case ".design":
		return "design"
	case ".js":
		base := strings.ToLower(filepath.Base(path))
		if strings.Contains(base, "helper") {
			return "helper"
		}
		return "controller"
	default:
		return "unknown"
	}
}

func actionKey(path, name string, offset int) string {
	return path + ":" + name + ":" + strconv.Itoa(offset)
}

func lineAt(source string, offset int) int {
	if offset < 0 {
		return 0
	}
	line := 1
	for i := 0; i < offset && i < len(source); i++ {
		if source[i] == '\n' {
			line++
		}
	}
	return line
}

func lookupKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
