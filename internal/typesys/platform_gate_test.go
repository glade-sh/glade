package typesys

import (
	"reflect"
	"sort"
	"strings"
	"testing"
	"unsafe"

	"github.com/glade-sh/glade/internal/apexast"
)

// platformSymbolSpecName is the spec name a table symbol was built from.
func platformSymbolSpecName(symbol TypeSymbol) string {
	if symbol.Namespace == "" {
		return symbol.Name
	}
	return symbol.Namespace + "." + symbol.Name
}

func gatedTestNamespaceBit(symbol TypeSymbol) uint8 {
	return gatedPlatformNamespaceBit(platformSymbolSpecName(symbol))
}

func platformSymbolKey(symbol TypeSymbol) string {
	return strings.ToLower(platformSymbolSpecName(symbol))
}

// With every namespace included the gated build is the ungated table, for
// both ConnectApi shapes.
func TestGatedPlatformViewAllIncludedEqualsUngated(t *testing.T) {
	for _, connectAPI := range []bool{false, true} {
		view, excluded := buildGatedStandardPlatformSymbols(connectAPI, gatedAllNamespaces)
		if excluded.Len() != 0 {
			t.Fatalf("connectAPI=%v: %d excluded types with every namespace included", connectAPI, excluded.Len())
		}
		if want := buildStandardPlatformSymbols(connectAPI); !reflect.DeepEqual(view, want) {
			t.Fatalf("connectAPI=%v: gated view differs from the ungated table", connectAPI)
		}
	}
}

// With a namespace left out, the view is the ungated stub table minus exactly
// that namespace's types outside the closure, every kept symbol unchanged, and
// the excluded list names exactly the removed types, in table order, with the
// stub symbol each had.
func TestGatedPlatformViewExcludesOnlyGatedTypes(t *testing.T) {
	ungated := buildStandardPlatformSymbols(false)
	for _, included := range []uint8{0, gatedConnectAPINamespace, gatedSlackNamespace} {
		view, excluded := buildGatedStandardPlatformSymbols(false, included)
		closure := gatedPlatformClosure(standardPlatformSymbolSpecList(false), included)
		var wantView, wantExcluded []TypeSymbol
		for _, symbol := range ungated {
			bit := gatedTestNamespaceBit(symbol)
			if bit != 0 && included&bit == 0 && !closure[platformSymbolKey(symbol)] {
				wantExcluded = append(wantExcluded, symbol)
				continue
			}
			wantView = append(wantView, symbol)
		}
		if len(view) != len(wantView) {
			have := map[string]bool{}
			for _, symbol := range view {
				have[platformSymbolKey(symbol)] = true
			}
			for _, symbol := range wantView {
				if !have[platformSymbolKey(symbol)] {
					t.Errorf("missing %q %q", symbol.Namespace, symbol.Name)
				}
			}
			t.Fatalf("included=%b: gated view has %d symbols, want %d", included, len(view), len(wantView))
		}
		for i := range view {
			if !reflect.DeepEqual(view[i], wantView[i]) {
				t.Fatalf("included=%b: gated view symbol %d is %s.%s %+v, want %s.%s %+v", included, i, view[i].Namespace, view[i].Name, view[i], wantView[i].Namespace, wantView[i].Name, wantView[i])
			}
		}
		if excluded.Len() != len(wantExcluded) {
			t.Fatalf("included=%b: %d excluded, want %d", included, excluded.Len(), len(wantExcluded))
		}
		at := 0
		excluded.Each(func(i int, namespace, name string, kind apexast.DeclarationKind) {
			want := wantExcluded[at]
			at++
			if namespace != want.Namespace || name != want.Name || kind != want.Kind {
				t.Fatalf("included=%b: excluded %s.%s (%s), want %s.%s (%s)", included, namespace, name, kind, want.Namespace, want.Name, want.Kind)
			}
			if got := excluded.Symbol(i); !reflect.DeepEqual(*got, want) {
				t.Fatalf("included=%b: stub symbol of %s.%s differs from the ungated table", included, namespace, name)
			}
		})
		var merged []string
		EachPlatformTypeName(view, excluded, func(namespace, name string, _ apexast.DeclarationKind, _ *TypeSymbol, _ int) {
			merged = append(merged, namespace+"."+name)
		})
		if len(merged) != len(ungated) {
			t.Fatalf("included=%b: merged walk has %d names, want %d", included, len(merged), len(ungated))
		}
		for i, symbol := range ungated {
			if merged[i] != symbol.Namespace+"."+symbol.Name {
				t.Fatalf("included=%b: merged walk position %d is %s, want %s.%s", included, i, merged[i], symbol.Namespace, symbol.Name)
			}
		}
	}
}

// Every gated stub symbol served for late binding is the stub table's symbol.
func TestGatedPlatformStubSymbolsMatchStubTable(t *testing.T) {
	count := 0
	for _, symbol := range buildStandardPlatformSymbols(false) {
		if gatedTestNamespaceBit(symbol) == 0 {
			continue
		}
		count++
		got, ok := GatedPlatformStubSymbol(strings.ToUpper(platformSymbolSpecName(symbol)))
		if !ok || !reflect.DeepEqual(*got, symbol) {
			t.Fatalf("GatedPlatformStubSymbol(%s.%s) differs from the stub table", symbol.Namespace, symbol.Name)
		}
		found := false
		for _, name := range GatedPlatformStubRuntimeNames(symbol.Name) {
			found = found || name == platformSymbolSpecName(symbol)
		}
		if !found {
			t.Fatalf("GatedPlatformStubRuntimeNames(%s) misses %s.%s", symbol.Name, symbol.Namespace, symbol.Name)
		}
	}
	if count < 5000 {
		t.Fatalf("only %d gated stub types", count)
	}
	if _, ok := GatedPlatformStubSymbol("System.String"); ok {
		t.Fatal("GatedPlatformStubSymbol served a type outside the gated namespaces")
	}
}

// The closure is pinned so a new stub cannot widen the base view unseen, and
// the base view names no type it leaves out.
func TestGatedPlatformClosurePinned(t *testing.T) {
	closure := gatedPlatformClosure(standardPlatformSymbolSpecList(false), 0)
	var got []string
	for name := range closure {
		got = append(got, name)
	}
	sort.Strings(got)
	want := []string{
		"connectapi.appointmentslotstatusresult",
		"connectapi.bookappointmentresult",
		"connectapi.bookappointmentsinput",
		"connectapi.cancelappointmentresult",
		"connectapi.cancelappointmentsinput",
		"connectapi.cpqattributecategoryoutputrepresentation",
		"connectapi.cpqattributepicklistoutputrepresentation",
		"connectapi.cpqattributepicklistvalueoutputrepresentation",
		"connectapi.cpqcatalogoutputrepresentation",
		"connectapi.cpqcategorydetailsoutputrepresentation",
		"connectapi.cpqcategorylistoutputrepresentation",
		"connectapi.cpqcategoryoutputrepresentation",
		"connectapi.cpqfacetvaluerepresentation",
		"connectapi.cpqguidedselectionsearchtermoutputrepresentation",
		"connectapi.cpqpricingmodeloutputrepresentation",
		"connectapi.cpqproductattributeoutputrepresentation",
		"connectapi.cpqproductclassificationoutputrepresentation",
		"connectapi.cpqproductcomponentgroupoutputrepresentation",
		"connectapi.cpqproductdetailsoutputrepresentation",
		"connectapi.cpqproductoutputrepresentation",
		"connectapi.cpqproductpricesoutputrepresentation",
		"connectapi.cpqproductquantityoutputrepresentation",
		"connectapi.cpqproductrelatedcomponentoutputrepresentation",
		"connectapi.cpqproductsellingmodeloptionoutputrepresentation",
		"connectapi.cpqproductsellingmodeloutputrepresentation",
		"connectapi.cpqproductspecificationrecordtypeoutputrepresentation",
		"connectapi.cpqproductspecificationtypeoutputrepresentation",
		"connectapi.cpqqualificationcontextoutputrepresentation",
		"connectapi.createpatientresult",
		"connectapi.getslotstatusinput",
		"connectapi.literaljson",
		"connectapi.placesalestransactionerrorresponse",
		"connectapi.qocqualificationoutputrepresentation",
		"connectapi.querysqlinput",
		"connectapi.querysqlmetadataitem",
		"connectapi.querysqlpageoutput",
		"connectapi.querysqlrow",
		"connectapi.querysqlrowrepresentation",
		"connectapi.unitofmeasureoutputrepresentation",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("closure =\n%s", strings.Join(got, "\n"))
	}
	view, _ := buildGatedStandardPlatformSymbols(false, 0)
	inView := make(map[string]bool, len(view))
	for _, symbol := range view {
		inView[platformSymbolKey(symbol)] = true
	}
	check := func(owner, text string) {
		forEachQualifiedTypeName(text, func(name string) {
			if gatedPlatformNamespaceBit(name) != 0 && !inView[strings.ToLower(name)] {
				if _, ok := GatedPlatformStubSymbol(name); ok {
					t.Errorf("%s names %s, which the base view leaves out", owner, name)
				}
			}
		})
	}
	for _, symbol := range view {
		owner := symbol.Namespace + "." + symbol.Name
		check(owner, symbol.SuperClass)
		for _, name := range symbol.Interfaces {
			check(owner, name)
		}
		for _, member := range symbol.Members {
			check(owner, member.Type)
			for _, param := range member.Parameters {
				check(owner, param.Type)
			}
		}
	}
}

// The base view stays small: under 9 MB of symbols, members and strings.
func TestGatedPlatformBaseViewBudget(t *testing.T) {
	view, excluded := buildGatedStandardPlatformSymbols(false, 0)
	var bytes uintptr
	for _, symbol := range view {
		bytes += unsafe.Sizeof(symbol) + uintptr(len(symbol.Name)+len(symbol.Namespace)+len(symbol.SuperClass))
		for _, member := range symbol.Members {
			bytes += unsafe.Sizeof(member) + uintptr(len(member.Name)+len(member.Type))
			bytes += uintptr(len(member.Parameters)) * unsafe.Sizeof(apexast.Parameter{})
		}
	}
	if bytes > 9<<20 {
		t.Fatalf("base view is %d bytes, budget %d", bytes, 9<<20)
	}
	if excluded.Len() < 5000 {
		t.Fatalf("base view leaves out only %d types", excluded.Len())
	}
}

func TestGatedPlatformNamespaceNames(t *testing.T) {
	for _, name := range []string{"ConnectApi.FeedItem", "connectapi.x", "Slack.App", "SLACK.App.Builder"} {
		if !IsGatedPlatformTypeName(name) {
			t.Errorf("IsGatedPlatformTypeName(%q) = false", name)
		}
	}
	for _, name := range []string{"ConnectApi", "Slack", "Slackware.App", "MySlack.App", "System.Slack", ""} {
		if IsGatedPlatformTypeName(name) {
			t.Errorf("IsGatedPlatformTypeName(%q) = true", name)
		}
	}
	for _, namespace := range []string{"ConnectApi", "slack", " Slack "} {
		if !IsGatedPlatformNamespace(namespace) {
			t.Errorf("IsGatedPlatformNamespace(%q) = false", namespace)
		}
	}
	if IsGatedPlatformNamespace("Slack.App") || IsGatedPlatformNamespace("Slacks") {
		t.Error("IsGatedPlatformNamespace accepted a non-namespace")
	}
	var names []string
	forEachQualifiedTypeName("Map<String, List<ConnectApi.FeedItem>>", func(name string) { names = append(names, name) })
	if !reflect.DeepEqual(names, []string{"Map", "String", "List", "ConnectApi.FeedItem"}) {
		t.Fatalf("type names = %q", names)
	}
}
