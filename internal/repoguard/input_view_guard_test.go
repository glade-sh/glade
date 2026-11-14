package repoguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// inputViewPackages are the loaders that read project inputs for a debugger
// startup. Their non-test code reads through the project's input view, so
// every input the loaded state used is pinned and rechecked.
var inputViewPackages = []string{
	"internal/project",
	"internal/config",
	"internal/schema",
	"internal/automation",
	"internal/resource",
}

// inputViewForbidden lists the direct filesystem functions, by import path,
// that bypass the view. A reference counts, not only a call: a reader default
// such as `read = os.ReadFile` bypasses it too.
var inputViewForbidden = map[string][]string{
	"os":            {"ReadFile", "ReadDir", "Stat", "Lstat", "Open", "OpenFile", "DirFS", "Readlink", "OpenRoot"},
	"io/fs":         {"ReadFile", "ReadDir", "Stat", "WalkDir", "Glob"},
	"io/ioutil":     {"ReadFile", "ReadDir", "ReadAll"},
	"path/filepath": {"WalkDir", "Walk", "Glob", "EvalSymlinks"},
	"github.com/glade-sh/glade/internal/config":  {"LoadNearest", "LoadFile"},
	"github.com/glade-sh/glade/internal/project": {"Load", "OrgDomainURL", "OrgRuntimeSettings"},
}

// inputViewAllowed names file:func entries that may read disk directly. Each
// is off every load path or reads inputs the view does not own.
var inputViewAllowed = map[string]string{
	"internal/project/org_suggest.go:SuggestOrgSetup":                               "setup hints for doctor and test-failure output; not a loader",
	"internal/project/org_suggest.go:missingMetadataSuggestions":                    "setup hints; reached only from SuggestOrgSetup",
	"internal/project/org_suggest.go:recordTypeMetadataNames":                       "setup hints; reached only from SuggestOrgSetup",
	"internal/project/org_suggest.go:roleMetadataNames":                             "setup hints; reached only from SuggestOrgSetup",
	"internal/project/org_suggest.go:scratchFeatureSuggestions":                     "setup hints; reached only from SuggestOrgSetup",
	"internal/project/org_suggest_field_metadata.go:setupCustomPersonAccountFields": "setup hints; reached only from SuggestOrgSetup",
	"internal/project/scratch_diagnostics.go:ScratchDefinitionWarnings":             "CLI warning text; not part of loaded state",
	"internal/project/project.go:EffectiveSourceAPIVersion":                         "Apex -meta.xml companion for semantic checks; Apex inputs stay on the source digest path",
	"internal/automation/flow_declarations.go:LoadFlowDeclarations":                 "nil-reader default; every caller passes a reader",
	"internal/resource/event_subscriber.go:ApplyProjectEventSubscribers":            "reads Apex trigger bodies, which stay on the source digest path",
	"internal/resource/layout_metadata.go:LoadLayoutMetadata":                       "nil-reader default; project loaders pass the project reader",
	"internal/resource/feature_parameters.go:loadFeatureParameterMetadata":          "nil-reader default; project loaders pass the project reader",
}

func TestInputViewPackagesReadThroughTheView(t *testing.T) {
	root := canonicalRepoRoot
	fset := token.NewFileSet()
	var findings []string
	used := map[string]bool{}
	for _, pkg := range inputViewPackages {
		entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(pkg)))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			name := entry.Name()
			if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			rel := pkg + "/" + name
			file, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			for _, finding := range inputViewBypasses(fset, file) {
				key := rel + ":" + finding.function
				if _, ok := inputViewAllowed[key]; ok {
					used[key] = true
					continue
				}
				findings = append(findings, fmt.Sprintf("%s:%d: %s in %s bypasses the project input view; read through p.ReadFile, p.ReadDir, p.Stat or p.WalkDir, or a passed *inputview.View", rel, finding.line, finding.selector, finding.function))
			}
		}
	}
	for key := range inputViewAllowed {
		if !used[key] {
			findings = append(findings, "stale input view allowlist entry "+key)
		}
	}
	sort.Strings(findings)
	if len(findings) > 0 {
		t.Fatalf("direct filesystem reads in input view packages:\n%s", strings.Join(findings, "\n"))
	}
}

// The runtime compiler lives outside the guarded loader packages. Its
// project and configuration reads must use the captured view wrappers.
func TestDAPCompilePathAvoidsUnviewedProjectLoads(t *testing.T) {
	fset := token.NewFileSet()
	path := filepath.Join(canonicalRepoRoot, "internal", "apextest", "runner.go")
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	targets := map[string]bool{
		"runtimeKeyWithDigestLookup":      true,
		"prepareRuntimeGeneration":        true,
		"visualforcePageNamesWithSources": true,
		"orgFromIndexWithError":           true,
	}
	for _, finding := range inputViewBypasses(fset, file) {
		if targets[finding.function] && (strings.HasPrefix(finding.selector, "project.") || strings.HasPrefix(finding.selector, "config.")) {
			t.Errorf("%s:%d: %s bypasses the captured input view", finding.function, finding.line, finding.selector)
		}
	}
}

type inputViewBypass struct {
	line     int
	selector string
	function string
}

func inputViewBypasses(fset *token.FileSet, file *ast.File) []inputViewBypass {
	aliases := map[string]string{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if _, ok := inputViewForbidden[path]; !ok {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		aliases[name] = path
	}
	if len(aliases) == 0 {
		return nil
	}
	var out []inputViewBypass
	for _, decl := range file.Decls {
		function := "<package>"
		if fn, ok := decl.(*ast.FuncDecl); ok {
			function = fn.Name.Name
		}
		ast.Inspect(decl, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			path, ok := aliases[ident.Name]
			if !ok {
				return true
			}
			for _, name := range inputViewForbidden[path] {
				if selector.Sel.Name == name {
					out = append(out, inputViewBypass{line: fset.Position(selector.Pos()).Line, selector: ident.Name + "." + name, function: function})
				}
			}
			return true
		})
	}
	return out
}
