package project_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

const policyDataFile = "internal/storage/standard_field_policy.json.gz"

// consumerDirs hold the code that reads org features at runtime.
var consumerDirs = []string{"internal/storage", "internal/vm", "internal/resource", "internal/project"}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func parseNonTest(t *testing.T, path string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return file
}

func stringLit(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	return value, err == nil
}

func calleeName(expr ast.Expr) string {
	switch fn := expr.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

// consumedFeatureLiterals returns every feature-name literal that runtime code
// passes to a feature check, switches on through canonicalFeatureName, or lists
// as a legacy or setting-trigger feature.
func consumedFeatureLiterals(t *testing.T, root string) map[string]string {
	t.Helper()
	consumed := map[string]string{}
	for _, dir := range consumerDirs {
		paths, err := filepath.Glob(filepath.Join(root, dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			if strings.HasSuffix(path, "_test.go") || filepath.Base(path) == "feature_registry.go" {
				continue
			}
			rel, _ := filepath.Rel(root, path)
			ast.Inspect(parseNonTest(t, path), func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.CallExpr:
					switch calleeName(n.Fun) {
					case "HasFeature", "hasCanonicalFeature", "enableFeature":
						for _, arg := range n.Args {
							if value, ok := stringLit(arg); ok {
								consumed[value] = rel
							}
						}
					}
				case *ast.SwitchStmt:
					call, ok := n.Tag.(*ast.CallExpr)
					if !ok || calleeName(call.Fun) != "canonicalFeatureName" {
						return true
					}
					for _, clause := range n.Body.List {
						for _, expr := range clause.(*ast.CaseClause).List {
							if value, ok := stringLit(expr); ok {
								consumed[value] = rel
							}
						}
					}
				case *ast.ValueSpec:
					for i, name := range n.Names {
						if i >= len(n.Values) {
							continue
						}
						list, ok := n.Values[i].(*ast.CompositeLit)
						if !ok {
							continue
						}
						switch name.Name {
						case "legacyFeatures":
							for _, elt := range list.Elts {
								if value, ok := stringLit(elt); ok {
									consumed[value] = rel
								}
							}
						case "scratchSettingTriggers":
							for _, elt := range list.Elts {
								row, ok := elt.(*ast.CompositeLit)
								if !ok || len(row.Elts) == 0 {
									continue
								}
								if value, ok := stringLit(row.Elts[len(row.Elts)-1]); ok {
									consumed[value] = rel
								}
							}
						}
					}
				}
				return true
			})
		}
	}
	return consumed
}

func registryNames() map[string]string {
	names := map[string]string{}
	for _, desc := range project.ScratchFeatureDescriptors() {
		names[strings.ToLower(desc.Name)] = desc.Name
		for _, alias := range desc.Aliases {
			names[strings.ToLower(alias)] = desc.Name
		}
	}
	return names
}

// A feature that runtime code reads but the registry omits would be reported as
// unread. This test derives the read set from the consumer source and from the
// embedded policy data, so adding a consumer without a registry entry fails.
func TestScratchFeatureRegistryCoversRuntimeConsumers(t *testing.T) {
	root := repoRoot(t)
	registered := registryNames()
	consumed := consumedFeatureLiterals(t, root)
	if len(consumed) < 10 {
		t.Fatalf("consumer scan found only %d feature literals; scan is broken: %v", len(consumed), consumed)
	}
	for _, name := range storage.PolicyFeatureNames() {
		if _, ok := consumed[name]; !ok {
			consumed[name] = policyDataFile
		}
	}
	// UnspecifiedStandardFieldShape is an internal marker, never a scratch feature.
	var missing []string
	for name, where := range consumed {
		if _, ok := registered[strings.ToLower(name)]; !ok {
			missing = append(missing, name+" (read in "+where+")")
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("features read by runtime code but absent from the scratch feature registry:\n%s", strings.Join(missing, "\n"))
	}
}

// Every registered consumer must still name its feature: the file must exist
// and contain the feature as a string literal (or, for policy data, be listed by
// storage.PolicyFeatureNames).
func TestScratchFeatureRegistryConsumersResolve(t *testing.T) {
	root := repoRoot(t)
	policy := map[string]bool{}
	for _, name := range storage.PolicyFeatureNames() {
		policy[strings.ToLower(name)] = true
	}
	for _, desc := range project.ScratchFeatureDescriptors() {
		if len(desc.Consumers) == 0 {
			t.Errorf("feature %q has no consumers", desc.Name)
		}
		names := append([]string{desc.Name}, desc.Aliases...)
		for _, consumer := range desc.Consumers {
			if consumer.File == policyDataFile {
				if !policy[strings.ToLower(desc.Name)] {
					t.Errorf("feature %q cites %s but the policy data does not read it", desc.Name, policyDataFile)
				}
				continue
			}
			found := false
			ast.Inspect(parseNonTest(t, filepath.Join(root, consumer.File)), func(node ast.Node) bool {
				value, ok := stringLit0(node)
				if !ok {
					return true
				}
				for _, name := range names {
					if strings.EqualFold(value, name) || (desc.Prefix && strings.HasPrefix(strings.ToLower(value), strings.ToLower(name))) {
						found = true
					}
				}
				return !found
			})
			if !found {
				t.Errorf("feature %q: %s has no string literal naming it", desc.Name, consumer.File)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, policyDataFile)); err != nil {
		t.Errorf("policy data file moved: %v", err)
	}
}

func stringLit0(node ast.Node) (string, bool) {
	expr, ok := node.(ast.Expr)
	if !ok {
		return "", false
	}
	return stringLit(expr)
}

func TestScratchFeatureMapping(t *testing.T) {
	for feature, want := range map[string]bool{
		"PersonAccounts": true, "personaccounts": true, " Chatter ": true,
		"FlowSites": true, "ProviderFreePlatformCache": true,
		"HealthCloud": true, "HealthCloud:2": true, "healthcloudfoo": true,
		"AddCustomApps:3": true, "addcustomapps:12": true,
		"AddCustomApps": false, "AddCustomApps:": false, "AddCustomApps:x": false,
		"B2BCommerce": true, "CommerceEnabled": true, "LiveAgent": true, "LiveAgentEnabled": true,
		"OrderManagement": true, "Quotes": true, "ApexUserModeWithPermset": true,
		"MultiCurrency:1": false, "Knowledge": false, "ServiceCloud": false, "": false,
	} {
		if got := project.IsMappedScratchFeature(feature); got != want {
			t.Errorf("IsMappedScratchFeature(%q) = %v, want %v", feature, got, want)
		}
	}
}
