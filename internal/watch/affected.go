package watch

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/apextest"
	"github.com/open-aer/oaer/internal/typesys"
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

func SelectAffectedTests(index typesys.Index, changes []Change) TestSelection {
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
				return allSelection(allTests, "changed Apex class may affect any test")
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
