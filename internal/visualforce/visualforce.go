package visualforce

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

type Index struct {
	Pages      []Page      `json:"pages,omitempty"`
	Components []Component `json:"components,omitempty"`

	pagesByName      map[string]int
	pagesByFile      map[string]int
	componentsByName map[string]int
	componentsByFile map[string]int
}

type Page struct {
	Name                string           `json:"name"`
	File                string           `json:"file,omitempty"`
	APIVersion          string           `json:"apiVersion,omitempty"`
	Controller          string           `json:"controller,omitempty"`
	StandardController  string           `json:"standardController,omitempty"`
	RecordSetVar        string           `json:"recordSetVar,omitempty"`
	Extensions          []string         `json:"extensions,omitempty"`
	Action              string           `json:"action,omitempty"`
	ComponentAttributes []Attribute      `json:"componentAttributes,omitempty"`
	MergeReferences     []MergeReference `json:"mergeReferences,omitempty"`
}

type Component struct {
	Name            string           `json:"name"`
	File            string           `json:"file,omitempty"`
	APIVersion      string           `json:"apiVersion,omitempty"`
	Controller      string           `json:"controller,omitempty"`
	Extensions      []string         `json:"extensions,omitempty"`
	Attributes      []Attribute      `json:"attributes,omitempty"`
	MergeReferences []MergeReference `json:"mergeReferences,omitempty"`
}

type Attribute struct {
	Name            string           `json:"name"`
	Type            string           `json:"type,omitempty"`
	AssignTo        string           `json:"assignTo,omitempty"`
	Required        string           `json:"required,omitempty"`
	Default         string           `json:"default,omitempty"`
	Description     string           `json:"description,omitempty"`
	MergeReferences []MergeReference `json:"mergeReferences,omitempty"`
}

type MergeReference struct {
	Kind       string `json:"kind"`
	Expression string `json:"expression"`
	Root       string `json:"root,omitempty"`
	Name       string `json:"name,omitempty"`
}

// PageRejection names a page or component that the strict loader refused and
// the reason. PageNames keeps every other page; callers report these so a bad
// source file is visible without hiding unrelated Page.X references.
type PageRejection struct {
	File    string
	Message string
}

// PageNames returns the sorted names that LoadProject registers for the pages
// that load. One page that fails to parse or validate is left out; its siblings
// stay registered. Metadata no page owns (permission set and profile page and
// class entries) never hides a page. Use PageNamesWithDiagnostics for the
// rejected files.
func PageNames(p project.Project) []string {
	names, _ := PageNamesWithDiagnostics(p, nil)
	return names
}

// DeclaredPageNames returns the page names a project's source declares,
// whether or not the page validates locally. An installed dependency's pages
// exist in the org that packaged them; a local validation failure there
// (for example a missing symbol in the dependency's own source) does not make
// Page.ns__Name unknown to a consumer.
func DeclaredPageNames(p project.Project) []string {
	var names []string
	seen := map[string]bool{}
	for _, path := range p.VisualforcePageFiles {
		name := nameFromPath(path, ".page")
		if name == "" || seen[strings.ToLower(name)] {
			continue
		}
		seen[strings.ToLower(name)] = true
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// PageNamesWithControllerCompilation is PageNames for a caller that already
// compiled the project's Apex. The strict loader's controller compile gate
// reuses that compilation when its index matches p; otherwise it compiles.
func PageNamesWithControllerCompilation(p project.Project, compilation *ControllerCompilation) []string {
	names, _ := PageNamesWithDiagnostics(p, compilation)
	return names
}

// PageNamesWithDiagnostics is PageNamesWithControllerCompilation plus the
// files the loader left out. A nil compilation makes the loader compile.
func PageNamesWithDiagnostics(p project.Project, compilation *ControllerCompilation) ([]string, []PageRejection) {
	cache := &loadCache{}
	names, rejected := pageNames(p, func(p project.Project, projectMetadata bool, only map[string]bool) (Index, error) {
		return loadProjectWithControllerCompilation(p, false, true, projectMetadata, compilation, only, cache)
	})
	recordPageRejections(p, rejected)
	return names, rejected
}

// pageLoader is the strict loader. projectMetadata false skips the checks that
// belong to no page (permission set and profile entries).
type pageLoader func(p project.Project, projectMetadata bool, only map[string]bool) (Index, error)

func pageNames(p project.Project, load pageLoader) ([]string, []PageRejection) {
	if len(p.VisualforcePageFiles) == 0 {
		return nil, nil
	}
	fallbackAPIVersion, err := apexversion.PreserveSource(p.SourceAPIVersion)
	if err != nil {
		return nil, nil
	}
	var rejected []PageRejection
	// Parse failures are unconditional in LoadProject. They drop the one page
	// (all of its files) or component, not the project.
	badPages := map[string]bool{}
	for _, path := range p.VisualforcePageFiles {
		if _, err := parsePageFile(path, fallbackAPIVersion); err != nil {
			badPages[strings.ToLower(nameFromPath(path, ".page"))] = true
			rejected = append(rejected, PageRejection{File: path, Message: err.Error()})
		}
	}
	badComponents := map[string]bool{}
	for _, path := range p.VisualforceComponentFiles {
		if _, err := parseComponentFile(path, fallbackAPIVersion); err != nil {
			badComponents[strings.ToLower(nameFromPath(path, ".component"))] = true
			rejected = append(rejected, PageRejection{File: path, Message: err.Error()})
		}
	}
	live := p
	live.VisualforcePageFiles = nil
	live.VisualforceComponentFiles = nil
	for _, path := range p.VisualforcePageFiles {
		if !badPages[strings.ToLower(nameFromPath(path, ".page"))] {
			live.VisualforcePageFiles = append(live.VisualforcePageFiles, path)
		}
	}
	for _, path := range p.VisualforceComponentFiles {
		if !badComponents[strings.ToLower(nameFromPath(path, ".component"))] {
			live.VisualforceComponentFiles = append(live.VisualforceComponentFiles, path)
		}
	}
	if len(live.VisualforcePageFiles) == 0 {
		return nil, rejected
	}
	collect := func(idx Index) []string {
		names := make([]string, 0, len(idx.Pages))
		for _, page := range idx.Pages {
			names = append(names, page.Name)
		}
		return names
	}
	idx, err := load(live, true, nil)
	if err == nil {
		return collect(idx), rejected
	}
	// Metadata no page owns (permission set and profile page and class entries)
	// is still an error for LoadProject, and callers see it here, but a page
	// does not disappear because a permission set names something else.
	var access *VisualforceAccessMetadataError
	if errors.As(err, &access) {
		rejected = append(rejected, PageRejection{File: access.File, Message: access.Message})
	}
	// Component validation reports each failing component and page by file.
	// Drop those files and load again; a page that uses a failed component is
	// among them.
	var projectErr error
	for {
		var idx Index
		idx, projectErr = load(live, false, nil)
		if projectErr == nil {
			return collect(idx), rejected
		}
		var components *ComponentValidationError
		if !errors.As(projectErr, &components) {
			break
		}
		drop := map[string]bool{}
		for _, diagnostic := range components.Diagnostics {
			drop[diagnostic.File] = true
			rejected = append(rejected, PageRejection{File: diagnostic.File, Message: diagnostic.Message})
		}
		before := len(live.VisualforcePageFiles) + len(live.VisualforceComponentFiles)
		live.VisualforcePageFiles = slices.DeleteFunc(slices.Clone(live.VisualforcePageFiles), func(path string) bool { return drop[path] })
		live.VisualforceComponentFiles = slices.DeleteFunc(slices.Clone(live.VisualforceComponentFiles), func(path string) bool { return drop[path] })
		if len(live.VisualforcePageFiles) == 0 {
			return nil, rejected
		}
		if len(live.VisualforcePageFiles)+len(live.VisualforceComponentFiles) == before {
			break
		}
	}
	// A page-owned failure: keep each page that loads on its own.
	groups := map[string][]string{}
	var order []string
	for _, path := range live.VisualforcePageFiles {
		key := strings.ToLower(nameFromPath(path, ".page"))
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], path)
	}
	var names []string
	for _, key := range order {
		own := map[string]bool{}
		for _, path := range groups[key] {
			own[path] = true
		}
		idx, err := load(live, false, own)
		if err != nil {
			rejected = append(rejected, PageRejection{File: groups[key][0], Message: err.Error()})
			continue
		}
		for _, page := range idx.Pages {
			if strings.EqualFold(page.Name, nameFromPath(groups[key][0], ".page")) {
				names = append(names, page.Name)
			}
		}
	}
	sort.Strings(names)
	return names, rejected
}

func LoadProject(p project.Project) (Index, error) {
	return loadProject(p, false, true)
}

// LoadProjectForRender validates page and component metadata and markup while
// leaving expressions and deployment reference checks to their respective paths.
// Preview requests must locate the page and authorize the principal before
// displaying expression diagnostics; hosted components retain their runtime
// unsupported boundary even when their deployment dependencies are unavailable.
func LoadProjectForRender(p project.Project) (Index, error) {
	return loadProject(p, false, false)
}

func LoadProjectBestEffort(p project.Project) Index {
	idx, _ := loadProject(p, true, false)
	return idx
}

func loadProject(p project.Project, bestEffort, validateExpressions bool) (Index, error) {
	return loadProjectWithControllerCompilation(p, bestEffort, validateExpressions, true, nil, nil, nil)
}

// only, when non-nil, limits the per-file validation to those paths. Every file
// is still parsed and indexed, so a reference from a validated page to another
// page resolves against the whole project. cache, when non-nil, shares the
// project-wide work (Apex scan, controller compile gate, project checks, parsed
// index) between loads of the same project; a nil cache recomputes it.
func loadProjectWithControllerCompilation(p project.Project, bestEffort, validateExpressions, projectMetadata bool, compilation *ControllerCompilation, only map[string]bool, cache *loadCache) (Index, error) {
	idx := Index{}
	fallbackAPIVersion, err := apexversion.PreserveSource(p.SourceAPIVersion)
	if err != nil {
		if bestEffort {
			return idx, nil
		}
		return Index{}, fmt.Errorf("invalid project source API version: %w", err)
	}
	if !bestEffort && validateExpressions && projectMetadata {
		if err := ValidateVisualforceAccessMetadata(p); err != nil {
			return Index{}, err
		}
	}
	if !bestEffort && validateExpressions && len(p.VisualforceComponentFiles) != 0 {
		if err := cache.customComponentProject(p); err != nil {
			return Index{}, err
		}
	}
	if !bestEffort && validateExpressions {
		if only != nil {
			// A page-by-page load validates only its own page's alternative
			// rendering, so one page's stylesheet does not reject the others.
			if err := validateAlternativeRenderingPages(p, only); err != nil {
				return Index{}, err
			}
		} else if err := cache.alternativeRenderingProject(p); err != nil {
			return Index{}, err
		}
	}
	if bestEffort {
		for _, path := range p.VisualforcePageFiles {
			page, err := parsePageFileLenient(path, fallbackAPIVersion)
			if err != nil {
				continue
			}
			idx.Pages = append(idx.Pages, page)
		}
		for _, path := range p.VisualforceComponentFiles {
			component, err := parseComponentFileLenient(path, fallbackAPIVersion)
			if err != nil {
				continue
			}
			idx.Components = append(idx.Components, component)
		}
	} else {
		parsed, err := cache.strictIndex(p, fallbackAPIVersion)
		if err != nil {
			return Index{}, err
		}
		idx.Pages = append([]Page(nil), parsed.pages...)
		idx.Components = append([]Component(nil), parsed.components...)
	}
	idx.sortAndBuildLookups()
	if !bestEffort {
		var expressions *expressionValidationContext
		var rejectedControllers map[string]bool
		for _, path := range append(append([]string(nil), p.VisualforcePageFiles...), p.VisualforceComponentFiles...) {
			if strings.HasSuffix(strings.ToLower(path), ".page-meta.xml") {
				continue
			}
			if only != nil && !only[path] {
				continue
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return Index{}, err
			}
			tree, err := parseMarkupTree(string(source), nameFromPath(path, filepath.Ext(path)))
			if err != nil {
				return Index{}, err
			}
			if err := validateMarkupComponents(tree, &idx, p.Namespace); err != nil {
				return Index{}, err
			}
			if err := validateRemoteObjectDeclarations(tree, p); err != nil {
				return Index{}, err
			}
			if err := validatePresentationControllerTypes(tree, p); err != nil {
				return Index{}, err
			}
			if root := visualforceControllerRoot(tree); validateExpressions && hasVisualforceControllerCompilationContext(p, path) && root != nil {
				if err := validateStandardControllerObject(root, p); err != nil {
					return Index{}, err
				}
			}
			if root := visualforceControllerRoot(tree); validateExpressions && hasVisualforceControllerCompilationContext(p, path) && root != nil && (root.Attribute("controller") != "" || root.Attribute("extensions") != "") {
				if expressions == nil {
					expressions, err = newExpressionValidationContextWith(p, &idx, cache)
					if err != nil {
						return Index{}, err
					}
				}
				if rejectedControllers == nil {
					rejectedControllers, err = cache.rejectedControllers(p, compilation)
					if err != nil {
						return Index{}, err
					}
				}
				if err := validateCompiledControllerDeclarations(root, rejectedControllers); err != nil {
					return Index{}, err
				}
				if err := validateControllerDeclarations(root, expressions.classes); err != nil {
					return Index{}, err
				}
			}
			if validateExpressions && strings.Contains(string(source), "{!") {
				if expressions == nil {
					expressions, err = newExpressionValidationContextWith(p, &idx, cache)
					if err != nil {
						return Index{}, err
					}
				}
				if err := expressions.validate(tree, nameFromPath(path, filepath.Ext(path))); err != nil {
					return Index{}, err
				}
			}
		}
	}
	return idx, nil
}

func ParsePageFile(path string) (Page, error) {
	return parsePageFile(path, "")
}

func parsePageFile(path, fallbackAPIVersion string) (Page, error) {
	return parsePageFileWithStructureValidation(path, fallbackAPIVersion, true)
}

func parsePageFileLenient(path, fallbackAPIVersion string) (Page, error) {
	return parsePageFileWithStructureValidation(path, fallbackAPIVersion, false)
}

func parsePageFileWithStructureValidation(path, fallbackAPIVersion string, validateStructure bool) (Page, error) {
	metadataOnly := strings.HasSuffix(strings.ToLower(path), ".page-meta.xml")
	if validateStructure && !metadataOnly {
		if err := validateVisualforceFileStructure(path, "page"); err != nil {
			return Page{}, err
		}
	}
	if validateStructure {
		if err := validateVisualforceMetadata(path, "ApexPage"); err != nil {
			return Page{}, err
		}
	}
	doc, err := parseMarkup(path)
	if err != nil {
		return Page{}, err
	}
	page := Page{Name: nameFromPath(path, ".page"), File: path}
	page.APIVersion, err = resource.EffectiveVisualforceAPIVersion(path, fallbackAPIVersion)
	if err != nil {
		return Page{}, err
	}
	for _, token := range doc.Tokens {
		if token.Start {
			if strings.EqualFold(token.Local, "page") && page.Controller == "" && page.StandardController == "" {
				page.Controller = attr(token.Attrs, "controller")
				page.StandardController = attr(token.Attrs, "standardController")
				page.RecordSetVar = attr(token.Attrs, "recordSetVar")
				page.Extensions = splitCSV(attr(token.Attrs, "extensions"))
				page.Action = attr(token.Attrs, "action")
			}
			if strings.EqualFold(token.Local, "attribute") {
				page.ComponentAttributes = append(page.ComponentAttributes, attributeFromToken(token))
			}
			for _, value := range token.Attrs {
				page.MergeReferences = append(page.MergeReferences, ExtractMergeReferences(value)...)
			}
			continue
		}
		page.MergeReferences = append(page.MergeReferences, ExtractMergeReferences(token.Text)...)
	}
	page.MergeReferences = dedupeMergeReferences(page.MergeReferences)
	return page, nil
}

func ParseComponentFile(path string) (Component, error) {
	return parseComponentFile(path, "")
}

func parseComponentFile(path, fallbackAPIVersion string) (Component, error) {
	return parseComponentFileWithStructureValidation(path, fallbackAPIVersion, true)
}

func parseComponentFileLenient(path, fallbackAPIVersion string) (Component, error) {
	return parseComponentFileWithStructureValidation(path, fallbackAPIVersion, false)
}

func parseComponentFileWithStructureValidation(path, fallbackAPIVersion string, validateStructure bool) (Component, error) {
	if validateStructure {
		if err := validateVisualforceFileStructure(path, "component"); err != nil {
			return Component{}, err
		}
		if err := validateVisualforceMetadata(path, "ApexComponent"); err != nil {
			return Component{}, err
		}
	}
	doc, err := parseMarkup(path)
	if err != nil {
		return Component{}, err
	}
	component := Component{Name: nameFromPath(path, ".component"), File: path}
	component.APIVersion, err = resource.EffectiveVisualforceAPIVersion(path, fallbackAPIVersion)
	if err != nil {
		return Component{}, err
	}
	for _, token := range doc.Tokens {
		if token.Start {
			if strings.EqualFold(token.Local, "component") && component.Controller == "" {
				component.Controller = attr(token.Attrs, "controller")
				component.Extensions = splitCSV(attr(token.Attrs, "extensions"))
			}
			if strings.EqualFold(token.Local, "attribute") {
				component.Attributes = append(component.Attributes, attributeFromToken(token))
			}
			for _, value := range token.Attrs {
				component.MergeReferences = append(component.MergeReferences, ExtractMergeReferences(value)...)
			}
			continue
		}
		component.MergeReferences = append(component.MergeReferences, ExtractMergeReferences(token.Text)...)
	}
	component.MergeReferences = dedupeMergeReferences(component.MergeReferences)
	return component, nil
}

func (i Index) Page(name string) (Page, bool) {
	name = strings.TrimSpace(name)
	if hasPrefixFold(name, "page.") {
		name = name[len("Page."):]
	}
	idx, ok := i.pagesByName[lookupKey(name)]
	if !ok {
		idx, ok = i.pagesByName[lookupKey(stripPageNamespace(name))]
	}
	if !ok {
		return Page{}, false
	}
	return i.Pages[idx], true
}

func (i Index) PageReference(name string) (Page, bool) {
	return i.Page(name)
}

func (i Index) HasPageReference(name string) bool {
	_, ok := i.Page(name)
	return ok
}

func (i Index) PageFile(path string) (Page, bool) {
	idx, ok := i.pagesByFile[filepath.Clean(path)]
	if !ok {
		return Page{}, false
	}
	return i.Pages[idx], true
}

func (i Index) Component(name string) (Component, bool) {
	idx, ok := i.componentsByName[lookupKey(name)]
	if !ok {
		return Component{}, false
	}
	return i.Components[idx], true
}

func (i Index) ComponentFile(path string) (Component, bool) {
	idx, ok := i.componentsByFile[filepath.Clean(path)]
	if !ok {
		return Component{}, false
	}
	return i.Components[idx], true
}

func (i *Index) sortAndBuildLookups() {
	sort.Slice(i.Pages, func(a, b int) bool { return i.Pages[a].Name < i.Pages[b].Name })
	sort.Slice(i.Components, func(a, b int) bool { return i.Components[a].Name < i.Components[b].Name })
	i.pagesByName = make(map[string]int, len(i.Pages))
	i.pagesByFile = make(map[string]int, len(i.Pages))
	for n, page := range i.Pages {
		i.pagesByName[lookupKey(page.Name)] = n
		i.pagesByFile[filepath.Clean(page.File)] = n
	}
	i.componentsByName = make(map[string]int, len(i.Components))
	i.componentsByFile = make(map[string]int, len(i.Components))
	for n, component := range i.Components {
		i.componentsByName[lookupKey(component.Name)] = n
		i.componentsByFile[filepath.Clean(component.File)] = n
	}
}

type markupDoc struct {
	Tokens []markupToken
}

type markupToken struct {
	Start bool
	Local string
	Attrs map[string]string
	Text  string
}

func parseMarkup(path string) (markupDoc, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return markupDoc{}, err
	}
	doc := markupDoc{}
	source := normalizeExpressionQuotes(string(content))
	doc.Tokens = append(doc.Tokens, scanMarkupStartTokens(source)...)
	if text := strings.TrimSpace(source); text != "" {
		doc.Tokens = append(doc.Tokens, markupToken{Text: text})
	}
	if len(doc.Tokens) == 0 {
		return markupDoc{}, fmt.Errorf("no Visualforce markup found in %s", path)
	}
	return doc, nil
}

var attrRE = regexp.MustCompile(`(?s)([A-Za-z_][A-Za-z0-9_.:-]*)\s*=\s*(?:"([^"]*)"|'([^']*)')`)

func scanMarkupStartTokens(source string) []markupToken {
	var tokens []markupToken
	for offset := 0; offset < len(source); {
		start := strings.IndexByte(source[offset:], '<')
		if start < 0 {
			break
		}
		start += offset
		nameStart := start + 1
		for nameStart < len(source) && isMarkupWhitespace(source[nameStart]) {
			nameStart++
		}
		if nameStart >= len(source) {
			break
		}
		switch source[nameStart] {
		case '/', '!', '?':
			next := findStartTagEnd(source, nameStart+1)
			if next < 0 {
				return tokens
			}
			offset = next + 1
			continue
		}
		nameEnd := nameStart
		for nameEnd < len(source) && isTagNameByte(source[nameEnd]) {
			nameEnd++
		}
		if nameEnd == nameStart {
			offset = start + 1
			continue
		}
		end := findStartTagEnd(source, nameEnd)
		if end < 0 {
			return tokens
		}
		rawName := strings.TrimSpace(source[nameStart:nameEnd])
		name := rawName
		if idx := strings.LastIndex(name, ":"); idx >= 0 {
			name = name[idx+1:]
		}
		attrs := make(map[string]string)
		for _, attrMatch := range attrRE.FindAllStringSubmatch(source[nameEnd:end], -1) {
			value := attrMatch[2]
			if value == "" {
				value = attrMatch[3]
			}
			attrName := attrMatch[1]
			if idx := strings.LastIndex(attrName, ":"); idx >= 0 {
				attrName = attrName[idx+1:]
			}
			attrs[lookupKey(attrName)] = strings.TrimSpace(value)
		}
		tokens = append(tokens, markupToken{Start: true, Local: name, Attrs: attrs})
		offset = end + 1
	}
	return tokens
}

func isMarkupWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func attributeFromToken(token markupToken) Attribute {
	attribute := Attribute{
		Name:        attr(token.Attrs, "name"),
		Type:        attr(token.Attrs, "type"),
		AssignTo:    attr(token.Attrs, "assignTo"),
		Required:    attr(token.Attrs, "required"),
		Default:     attr(token.Attrs, "default"),
		Description: attr(token.Attrs, "description"),
	}
	attribute.MergeReferences = dedupeMergeReferences(ExtractMergeReferences(attribute.AssignTo))
	return attribute
}

var mergeRE = regexp.MustCompile(`\{!\s*([^}]+?)\s*\}`)
var resourceInURLFORRE = regexp.MustCompile(`(?i)\$Resource\.([A-Za-z_][A-Za-z0-9_]*)`)
var urlforPathArgRE = regexp.MustCompile(`(?i)^URLFOR\s*\([^,]+,\s*['"]([^'"]+)['"]`)

func ExtractMergeReferences(value string) []MergeReference {
	matches := mergeRE.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return nil
	}
	refs := make([]MergeReference, 0, len(matches))
	for _, match := range matches {
		expr := strings.TrimSpace(match[1])
		refs = append(refs, classifyMergeExpression(expr))
	}
	return dedupeMergeReferences(refs)
}

func ResolveResourceURL(registry storage.MetadataRegistry, expr string) (string, bool) {
	ref := classifyMergeExpression(strings.TrimSpace(expr))
	if ref.Kind == "URLFOR" && ref.Root == "$Resource" {
		return resource.URLForStaticResource(registry, ref.Name, urlforPathArg(ref.Expression))
	}
	if ref.Kind == "StaticResource" && ref.Root == "$Resource" {
		return resource.URLForStaticResource(registry, ref.Name, "")
	}
	return "", false
}

func classifyMergeExpression(expr string) MergeReference {
	lower := strings.ToLower(expr)
	if strings.HasPrefix(lower, "urlfor") {
		ref := MergeReference{Kind: "URLFOR", Expression: expr}
		if match := resourceInURLFORRE.FindStringSubmatch(expr); len(match) == 2 {
			ref.Root = "$Resource"
			ref.Name = match[1]
		}
		return ref
	}
	for _, candidate := range []struct {
		prefix string
		kind   string
		root   string
	}{
		{"$Label.", "Label", "$Label"},
		{"$ObjectType.", "ObjectType", "$ObjectType"},
		{"$Resource.", "StaticResource", "$Resource"},
		{"$Site.", "Site", "$Site"},
	} {
		if strings.HasPrefix(lower, strings.ToLower(candidate.prefix)) {
			name := strings.TrimSpace(expr[len(candidate.prefix):])
			return MergeReference{Kind: candidate.kind, Expression: expr, Root: candidate.root, Name: name}
		}
	}
	return MergeReference{Kind: "ControllerExpression", Expression: expr, Name: expr}
}

func urlforPathArg(expr string) string {
	if match := urlforPathArgRE.FindStringSubmatch(strings.TrimSpace(expr)); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func dedupeMergeReferences(refs []MergeReference) []MergeReference {
	if len(refs) == 0 {
		return nil
	}
	seen := make(map[MergeReference]bool, len(refs))
	out := make([]MergeReference, 0, len(refs))
	for _, ref := range refs {
		if ref.Expression == "" {
			continue
		}
		if seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}

func attr(attrs map[string]string, name string) string {
	return strings.TrimSpace(attrs[lookupKey(name)])
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func nameFromPath(path, suffix string) string {
	base := filepath.Base(path)
	if (suffix == ".page" || suffix == ".component") && hasSuffixFold(base, suffix+"-meta.xml") {
		return base[:len(base)-len(suffix+"-meta.xml")]
	}
	if hasSuffixFold(base, suffix) {
		return base[:len(base)-len(suffix)]
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func stripPageNamespace(name string) string {
	name = strings.TrimSpace(name)
	if idx := strings.Index(name, "__"); idx > 0 {
		return name[idx+len("__"):]
	}
	return name
}

func hasPrefixFold(value, prefix string) bool {
	return len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix)
}

func hasSuffixFold(value, suffix string) bool {
	return len(value) >= len(suffix) && strings.EqualFold(value[len(value)-len(suffix):], suffix)
}

func lookupKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
