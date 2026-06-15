package watch

import (
	"path/filepath"
	"sort"
	"strings"

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

// SelectAffectedTests builds a fresh reference graph and selects the tests
// affected by the given changes. Callers that watch continuously should keep a
// RefGraph (see Refresh) and use SelectAffectedTestsWithRefGraph to avoid
// rebuilding the graph on every change.
func SelectAffectedTests(index typesys.Index, changes []Change) TestSelection {
	return SelectAffectedTestsWithRefGraph(index, changes, BuildReferenceGraph(index))
}

// SelectAffectedTestsWithRefGraph selects affected tests using a prebuilt
// reference graph. Selection never under-selects relative to running affected
// tests: a changed type with no reachable test, an unrecognized class, a
// trigger, or schema metadata all fall back to running every test.
func SelectAffectedTestsWithRefGraph(index typesys.Index, changes []Change, graph *RefGraph) TestSelection {
	allTests := graph.allTests
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
			if _, ok := graph.resolvedFile[cleanPath(typ.File)]; !ok {
				return allSelection(allTests, "changed Apex class is not resolved by code intelligence")
			}
			tests := graph.affectedTests(typ.Name)
			if len(tests) == 0 {
				return allSelection(allTests, "changed Apex class may affect any test")
			}
			for _, testClass := range tests {
				selected[testClass] = struct{}{}
			}
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
		Reason:      "changed types reach affected tests",
	}
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
