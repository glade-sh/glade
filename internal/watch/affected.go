package watch

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/typesys"
)

type SelectionMode string

const (
	SelectionNone   SelectionMode = "none"
	SelectionDirect SelectionMode = "direct"
	SelectionAll    SelectionMode = "all"
)

type TestSelection struct {
	Mode        SelectionMode `json:"mode"`
	TestClasses []string      `json:"testClasses,omitempty"`
	Reason      string        `json:"reason,omitempty"`
}

type DependencyGraph struct {
	ByType map[string][]string `json:"byType"`
}

func BuildDependencyGraph(index typesys.Index) DependencyGraph {
	graph := DependencyGraph{ByType: make(map[string][]string)}
	productionTypes := productionTypeNames(index)
	for _, testCase := range apextest.Discover(index, apextest.Options{}) {
		source, err := os.ReadFile(testCase.File)
		if err != nil {
			continue
		}
		text := strings.ToLower(string(source))
		for _, name := range productionTypes {
			if containsIdentifier(text, strings.ToLower(name)) {
				graph.ByType[name] = appendUniqueSorted(graph.ByType[name], testCase.ClassName)
			}
		}
	}
	return graph
}

func SelectAffectedTests(index typesys.Index, changes []Change) TestSelection {
	return SelectAffectedTestsWithGraph(index, changes, BuildDependencyGraph(index))
}

func SelectAffectedTestsWithGraph(index typesys.Index, changes []Change, graph DependencyGraph) TestSelection {
	allTests := allTestClasses(index)
	if len(changes) == 0 || len(allTests) == 0 {
		return TestSelection{Mode: SelectionNone}
	}

	selected := make(map[string]struct{})
	for _, change := range changes {
		switch change.Kind {
		case FileKindApexClass:
			typ, ok := typeForClassChange(index, change)
			if !ok {
				return allSelection(allTests, "changed Apex class is not present in the type index")
			}
			if !typeHasTests(index, typ.Name) {
				dependent := graph.ByType[typ.Name]
				if len(dependent) == 0 {
					return allSelection(allTests, "changed Apex class may affect any test")
				}
				for _, testClass := range dependent {
					selected[testClass] = struct{}{}
				}
				continue
			}
			selected[typ.Name] = struct{}{}
		case FileKindApexTrigger:
			return allSelection(allTests, "changed trigger may affect any test")
		case FileKindObjectMeta, FileKindFieldMeta:
			return allSelection(allTests, "changed schema metadata may affect any test")
		}
	}

	testClasses := sortedKeys(selected)
	if len(testClasses) == 0 {
		return TestSelection{Mode: SelectionNone}
	}
	return TestSelection{
		Mode:        SelectionDirect,
		TestClasses: testClasses,
		Reason:      "changed test class matched directly",
	}
}

func productionTypeNames(index typesys.Index) []string {
	var names []string
	for _, typ := range index.Types {
		if typ.Kind != apexast.DeclarationClass || typeHasTests(index, typ.Name) {
			continue
		}
		names = append(names, typ.Name)
	}
	sort.Strings(names)
	return names
}

func containsIdentifier(text, name string) bool {
	for offset := 0; offset < len(text); {
		next := strings.Index(text[offset:], name)
		if next < 0 {
			return false
		}
		start := offset + next
		end := start + len(name)
		if isIdentifierBoundary(text, start-1) && isIdentifierBoundary(text, end) {
			return true
		}
		offset = end
	}
	return false
}

func isIdentifierBoundary(text string, offset int) bool {
	return offset < 0 || offset >= len(text) || !isIdentifierByte(text[offset])
}

func isIdentifierByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_'
}

func appendUniqueSorted(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	values = append(values, value)
	sort.Strings(values)
	return values
}

func typeForClassChange(index typesys.Index, change Change) (typesys.TypeSymbol, bool) {
	changePath := cleanPath(change.Path)
	for _, typ := range index.Types {
		if cleanPath(typ.File) == changePath {
			return typ, true
		}
	}
	for _, typ := range index.Types {
		if strings.EqualFold(typ.Name, change.Name) {
			return typ, true
		}
	}
	return typesys.TypeSymbol{}, false
}

func typeHasTests(index typesys.Index, className string) bool {
	for _, testCase := range apextest.Discover(index, apextest.Options{}) {
		if testCase.ClassName == className {
			return true
		}
	}
	return false
}

func allTestClasses(index typesys.Index) []string {
	seen := make(map[string]struct{})
	for _, testCase := range apextest.Discover(index, apextest.Options{}) {
		seen[testCase.ClassName] = struct{}{}
	}
	return sortedKeys(seen)
}

func allSelection(testClasses []string, reason string) TestSelection {
	return TestSelection{
		Mode:        SelectionAll,
		TestClasses: testClasses,
		Reason:      reason,
	}
}

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func cleanPath(path string) string {
	abs, err := filepath.Abs(path)
	if err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(path)
}
