package typesys

import (
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
)

// referenceStandardSymbolsFromSpecs is the table builder as it stood before
// the platform table was sized exactly and its identical modifier and
// parameter lists were shared. It anchors the equivalence tests.
func referenceStandardSymbolsFromSpecs(specs []StandardSymbolSpec) []TypeSymbol {
	specs = mergeStandardSymbolSpecs(specs)
	out := make([]TypeSymbol, 0, len(specs))
	seen := make(map[string]bool, len(specs))
	for _, spec := range specs {
		if spec.Name == "" {
			continue
		}
		key := strings.ToLower(spec.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		kind := spec.Kind
		if kind == "" {
			kind = apexast.DeclarationClass
		}
		superClass := spec.SuperClass
		if kind != apexast.DeclarationClass {
			superClass = ""
		}
		sym := TypeSymbol{
			Kind:                      kind,
			Name:                      spec.Name,
			File:                      "<standard-platform>",
			Dependency:                true,
			ConstructorsAuthoritative: spec.ReplaceConstructors,
			EnumHashBase:              spec.EnumHashBase,
			SuperClass:                superClass,
			Modifiers:                 append([]string(nil), spec.Modifiers...),
			Interfaces:                append([]string(nil), spec.Interfaces...),
		}
		if namespace, localName, ok := splitStandardSymbolName(spec.Name); ok {
			sym.Namespace = namespace
			sym.Name = localName
		}
		if kind == apexast.DeclarationClass {
			for _, ctor := range standardConstructorSpecs(spec) {
				sym.Members = append(sym.Members, MemberSymbol{
					Kind:       apexast.DeclarationConstructor,
					Name:       localStandardSymbolName(spec.Name),
					Modifiers:  []string{"public", "passive-generated"},
					Parameters: standardSpecParameters(ctor.Parameters),
				})
			}
		}
		for _, prop := range spec.Properties {
			modifiers := []string{"public"}
			if prop.Static {
				modifiers = append(modifiers, "static")
			}
			sym.Members = append(sym.Members, MemberSymbol{
				Kind:      apexast.DeclarationProperty,
				Name:      prop.Name,
				Type:      prop.Type,
				Modifiers: modifiers,
			})
		}
		for _, field := range spec.Fields {
			modifiers := []string{"public"}
			if field.Static {
				modifiers = append(modifiers, "static")
			}
			sym.Members = append(sym.Members, MemberSymbol{
				Kind:      apexast.DeclarationField,
				Name:      field.Name,
				Type:      field.Type,
				Modifiers: modifiers,
			})
		}
		for _, method := range spec.Methods {
			modifiers := append([]string(nil), method.Modifiers...)
			if !standardModifierContains(modifiers, "private") && !standardModifierContains(modifiers, "protected") && !standardModifierContains(modifiers, "public") && !standardModifierContains(modifiers, "global") {
				modifiers = append([]string{"public"}, modifiers...)
			}
			if method.Static {
				modifiers = append(modifiers, "static")
			}
			sym.Members = append(sym.Members, MemberSymbol{
				Kind:       apexast.DeclarationMethod,
				Name:       method.Name,
				Type:       method.ReturnType,
				Modifiers:  modifiers,
				Parameters: standardMethodParameters(spec.Name, method),
			})
		}
		out = append(out, sym)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace == out[j].Namespace {
			return out[i].Name < out[j].Name
		}
		return out[i].Namespace < out[j].Namespace
	})
	return out
}

func platformSpecsForTest(connectAPI bool) []StandardSymbolSpec {
	var captured []StandardSymbolSpec
	if connectAPI {
		captured = connectAPICapturedShape().specs
	}
	specs := append([]StandardSymbolSpec(nil), standardPlatformSymbolSpecs...)
	specs = append(specs, productNamespaceSymbolSpecs...)
	specs = append(specs, systemStubSymbolSpecs...)
	specs = append(specs, plan7RichMessagingSymbolSpecs...)
	specs = append(specs, plan7SFDWSymbolSpecs...)
	specs = append(specs, dataSourcePlatformSymbolOverlays...)
	specs = append(specs, standardPlatformSymbolOverlays...)
	specs = append(specs, cartHierarchyOverlays()...)
	specs = append(specs, commerceServiceOverlays()...)
	specs = append(specs, paymentPlatformSymbolOverlays...)
	return append(specs, captured...)
}

func TestStandardSymbolsFromSpecsMatchesReferenceBuilder(t *testing.T) {
	specs := platformSpecsForTest(false)
	want := referenceStandardSymbolsFromSpecs(specs)
	got := StandardSymbolsFromSpecs(specs)
	if len(got) != len(want) {
		t.Fatalf("type count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Fatalf("type %d (%s.%s) differs from the reference builder", i, want[i].Namespace, want[i].Name)
		}
	}
}

func TestPlatformSymbolTableMatchesReferenceBuilderInOrder(t *testing.T) {
	for _, connectAPI := range []bool{false, true} {
		got := buildStandardPlatformSymbols(connectAPI)
		want := referenceStandardSymbolsFromSpecs(platformSpecsWithHierarchy(connectAPI))
		if len(got) != len(want) {
			t.Fatalf("connectAPI=%v: type count = %d, want %d", connectAPI, len(got), len(want))
		}
		for i := range want {
			if got[i].Namespace != want[i].Namespace || got[i].Name != want[i].Name {
				t.Fatalf("connectAPI=%v: order differs at %d: %s.%s vs %s.%s", connectAPI, i, got[i].Namespace, got[i].Name, want[i].Namespace, want[i].Name)
			}
			if !reflect.DeepEqual(got[i], want[i]) {
				t.Fatalf("connectAPI=%v: type %s.%s differs from the reference builder", connectAPI, want[i].Namespace, want[i].Name)
			}
		}
	}
}

// platformSpecsWithHierarchy repeats the spec assembly of
// buildStandardPlatformSymbols so the reference builder sees the same input.
func platformSpecsWithHierarchy(connectAPI bool) []StandardSymbolSpec {
	specs := platformSpecsForTest(false)
	var captured *connectAPIShape
	if connectAPI {
		captured = connectAPICapturedShape()
		specs = append(specs, captured.specs...)
	}
	known := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		known[strings.ToLower(spec.Name)] = struct{}{}
	}
	for _, name := range standardPlatformTypeNames {
		key := strings.ToLower(name)
		if _, ok := known[key]; ok {
			continue
		}
		known[key] = struct{}{}
		specs = append(specs, StandardSymbolSpec{Name: name, Kind: apexast.DeclarationClass})
	}
	for i := range specs {
		if parent, ok := StandardExceptionParent(specs[i].Name); ok {
			specs[i].SuperClass = parent
		}
		if captured != nil && hasConnectAPIPrefix(specs[i].Name) {
			if parent, ok := captured.superClasses[strings.ToLower(specs[i].Name)]; ok {
				specs[i].SuperClass = parent
			}
		}
	}
	return specs
}

// platformTableBytes reports bytes allocated while building the table and the
// bytes still live after a collection.
func platformTableBytes() (allocated, live uint64, table []TypeSymbol) {
	runtime.GC()
	var before, mid, after runtime.MemStats
	runtime.ReadMemStats(&before)
	table = buildStandardPlatformSymbols(false)
	runtime.ReadMemStats(&mid)
	runtime.GC()
	runtime.ReadMemStats(&after)
	return mid.TotalAlloc - before.TotalAlloc, after.HeapAlloc - before.HeapAlloc, table
}

// The budgets sit about 10% above the sizes measured after the change (35 MB
// allocated, 22 MB live). The
// builder used to allocate 65 MB and keep 31 MB live.
const (
	platformTableAllocBudget = 38 << 20
	platformTableLiveBudget  = 24 << 20
)

func TestPlatformSymbolTableAllocationAndLiveBudget(t *testing.T) {
	allocated, live, table := platformTableBytes()
	runtime.KeepAlive(table)
	t.Logf("platform table build: %d bytes allocated, %d bytes live", allocated, live)
	if allocated > platformTableAllocBudget {
		t.Fatalf("allocated %d bytes, budget %d", allocated, platformTableAllocBudget)
	}
	if live > platformTableLiveBudget {
		t.Fatalf("live %d bytes, budget %d", live, platformTableLiveBudget)
	}
}

// Shared modifier and parameter lists must not let one member change another.
func TestPlatformSymbolViewSharedListsAreCapacityClamped(t *testing.T) {
	for _, symbol := range StandardPlatformSymbolView() {
		for _, member := range symbol.Members {
			if cap(member.Modifiers) != len(member.Modifiers) {
				t.Fatalf("%s.%s modifiers have spare capacity", symbol.Name, member.Name)
			}
			if cap(member.Parameters) != len(member.Parameters) {
				t.Fatalf("%s.%s parameters have spare capacity", symbol.Name, member.Name)
			}
		}
	}
}

func TestStandardPlatformSymbolsCopyIsIndependentOfView(t *testing.T) {
	view := StandardPlatformSymbolView()
	copied := StandardPlatformSymbols()
	for i := range copied {
		for j := range copied[i].Members {
			for k := range copied[i].Members[j].Modifiers {
				copied[i].Members[j].Modifiers[k] = "mutated"
			}
			for k := range copied[i].Members[j].Parameters {
				copied[i].Members[j].Parameters[k].Name = "mutated"
			}
		}
	}
	for _, symbol := range view {
		for _, member := range symbol.Members {
			for _, modifier := range member.Modifiers {
				if modifier == "mutated" {
					t.Fatalf("%s.%s: copy shares a modifier list with the view", symbol.Name, member.Name)
				}
			}
			for _, parameter := range member.Parameters {
				if parameter.Name == "mutated" {
					t.Fatalf("%s.%s: copy shares a parameter list with the view", symbol.Name, member.Name)
				}
			}
		}
	}
}

func TestPlatformSymbolTableConcurrentBuildsAgree(t *testing.T) {
	want := buildStandardPlatformSymbols(false)
	var wg sync.WaitGroup
	results := make([][]TypeSymbol, 6)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = buildStandardPlatformSymbols(false)
			_ = StandardPlatformSymbolView()
		}(i)
	}
	wg.Wait()
	for i, got := range results {
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("concurrent build %d differs", i)
		}
	}
}
