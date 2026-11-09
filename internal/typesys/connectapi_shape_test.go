package typesys

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
)

// The archive carries the whole captured namespace; these counts are the
// generator's own (scripts/generate-connectapi-symbols.mjs prints them).
func TestConnectAPIShapeArchiveDecodes(t *testing.T) {
	shape, err := decodeConnectAPIShape(connectAPIShapeArchive)
	if err != nil {
		t.Fatal(err)
	}
	enums, typed := 0, 0
	for _, spec := range shape.specs {
		if !strings.HasPrefix(spec.Name, "ConnectApi.") {
			t.Fatalf("spec %q is outside the ConnectApi namespace", spec.Name)
		}
		if spec.Kind == apexast.DeclarationEnum {
			enums++
		}
		for _, prop := range spec.Properties {
			if prop.Force {
				typed++
			}
		}
	}
	for name, want := range map[string][2]int{
		"types":         {len(shape.specs), 6673},
		"enums":         {enums, 895},
		"typed props":   {typed, 21404},
		"superclasses":  {len(shape.superClasses), 746},
		"member ranges": {len(shape.memberAvailability), 10783},
		"exact ranges":  {len(shape.exactAvailability), 388},
		"output types":  {len(shape.outputTypes), 3794},
		"json profiles": {len(shape.jsonProfiles), 2476},
	} {
		if want[0] != want[1] {
			t.Errorf("%s = %d, want %d", name, want[0], want[1])
		}
	}
	byName := map[string]StandardSymbolSpec{}
	for _, spec := range shape.specs {
		byName[spec.Name] = spec
	}
	input := byName["ConnectApi.AbortCartOperationInput"]
	if len(input.ConstructorSpecs) != 1 || input.ConstructorSpecs[0].Parameters == nil || len(input.ConstructorSpecs[0].Parameters) != 0 {
		t.Fatalf("AbortCartOperationInput constructors = %#v, want one empty non-nil parameter list", input.ConstructorSpecs)
	}
	if len(input.Methods) != 4 || input.Methods[1].Name != "equals" || input.Methods[1].ParameterSpecs[0].Type != "Object" || input.Methods[0].ParameterSpecs != nil {
		t.Fatalf("AbortCartOperationInput methods = %#v", input.Methods)
	}
	if len(input.Properties) != 2 || input.Properties[1] != (StandardPropertySpec{Name: "type", Type: "ConnectApi.CartOperationType", Force: true}) {
		t.Fatalf("AbortCartOperationInput properties = %#v", input.Properties)
	}
	visibility := byName["ConnectApi.AIModelVisibilityEnum"]
	if visibility.Kind != apexast.DeclarationEnum || len(visibility.Properties) != 2 || !visibility.Properties[0].Static || visibility.Properties[0].Force {
		t.Fatalf("AIModelVisibilityEnum = %#v", visibility)
	}
	if !byName["ConnectApi.AbstractCartItem"].ReplaceConstructors {
		t.Fatal("AbstractCartItem should reject new")
	}
	if got := shape.superClasses["connectapi.feeditem"]; got != "ConnectApi.FeedElement" {
		t.Fatalf("FeedItem superclass = %q", got)
	}
	if rng, ok := shape.memberAvailability["connectapi.abortcartoperationinput.jobid"]; !ok || rng.Since != 67 {
		t.Fatalf("jobId range = %#v %v", rng, ok)
	}
}

// Without a request the table is the stub-only table: no captured-only type,
// no captured superclass. With it, the captured shape is the last overlay.
func TestStandardPlatformSymbolsGateConnectAPIShape(t *testing.T) {
	find := func(symbols []TypeSymbol, name string) (TypeSymbol, bool) {
		for _, symbol := range symbols {
			if strings.EqualFold(symbol.Namespace+"."+symbol.Name, name) {
				return symbol, true
			}
		}
		return TypeSymbol{}, false
	}
	stubOnly := buildStandardPlatformSymbols(false)
	if _, ok := find(stubOnly, "ConnectApi.AbortCartOperationInput"); ok {
		t.Fatal("stub-only table carries a captured-only ConnectApi type")
	}
	feedItem, ok := find(stubOnly, "ConnectApi.FeedItem")
	if !ok {
		t.Fatal("stub-only table lost the stub ConnectApi.FeedItem")
	}
	if feedItem.SuperClass == "ConnectApi.FeedElement" {
		t.Fatal("stub-only table applied the captured FeedItem superclass")
	}
	captured := buildStandardPlatformSymbols(true)
	if _, ok := find(captured, "ConnectApi.AbortCartOperationInput"); !ok {
		t.Fatal("captured table lacks ConnectApi.AbortCartOperationInput")
	}
	if feedItem, ok = find(captured, "ConnectApi.FeedItem"); !ok || feedItem.SuperClass != "ConnectApi.FeedElement" {
		t.Fatalf("captured FeedItem superclass = %q", feedItem.SuperClass)
	}
	if len(captured) <= len(stubOnly) {
		t.Fatalf("captured table has %d types, stub-only %d", len(captured), len(stubOnly))
	}
	// Non-ConnectApi symbols are identical in both tables.
	stubByName := map[string]TypeSymbol{}
	for _, symbol := range stubOnly {
		if !strings.EqualFold(symbol.Namespace, "ConnectApi") {
			stubByName[symbol.Namespace+"."+symbol.Name] = symbol
		}
	}
	others := 0
	for _, symbol := range captured {
		if strings.EqualFold(symbol.Namespace, "ConnectApi") {
			continue
		}
		others++
		want, ok := stubByName[symbol.Namespace+"."+symbol.Name]
		if !ok || len(want.Members) != len(symbol.Members) || want.SuperClass != symbol.SuperClass {
			t.Fatalf("non-ConnectApi symbol %s.%s differs between tables", symbol.Namespace, symbol.Name)
		}
	}
	if others != len(stubByName) {
		t.Fatalf("non-ConnectApi symbols: %d with capture, %d without", others, len(stubByName))
	}
}

func TestMentionsConnectAPIFoldsCase(t *testing.T) {
	for _, source := range []string{"ConnectApi.FeedItem f;", "CONNECTAPI.FEEDITEM f;", "x = connectapi", "cOnNeCtApI", "a\nConnectApi"} {
		if !mentionsConnectAPI(source) {
			t.Errorf("mentionsConnectAPI(%q) = false", source)
		}
	}
	for _, source := range []string{"", "Connect Api", "ConnectAp", "connect.api", "Connect_Api.Feed", "ApiConnect"} {
		if mentionsConnectAPI(source) {
			t.Errorf("mentionsConnectAPI(%q) = true", source)
		}
	}
}

func TestConnectAPIAccessorsIgnoreOtherNamespaces(t *testing.T) {
	if _, ok := ConnectAPIMemberAvailability("System.String", "length"); ok {
		t.Fatal("System member reported a ConnectApi range")
	}
	if _, ok := ConnectAPIExactAvailability("apex:system.string.length()"); ok {
		t.Fatal("System surface reported a ConnectApi range")
	}
	if ConnectAPIOutputType("Schema.SObjectType") || ConnectAPICapturedType("System.String") {
		t.Fatal("non-ConnectApi type reported as captured")
	}
	if rng, ok := ConnectAPIMemberAvailability(" ConnectApi.AbortCartOperationInput ", "JobId"); !ok || rng.Since != 67 {
		t.Fatalf("jobId = %#v %v", rng, ok)
	}
	if !ConnectAPICapturedType("connectapi.feeditem") || !ConnectAPIOutputType("ConnectApi.FeedItem") {
		t.Fatal("FeedItem is a captured output type")
	}
}

// A source that names ConnectApi flips the process to the captured view; the
// view generation moves and PlatformViewCache holders rebuild from it.
func TestNoteApexSourceRequestsConnectAPIShapeAndRebuildsCaches(t *testing.T) {
	hasCaptured := func(view []TypeSymbol) bool {
		for _, symbol := range view {
			if symbol.Namespace == "ConnectApi" && symbol.Name == "AbortCartOperationInput" {
				return true
			}
		}
		return false
	}
	var cache PlatformViewCache[int]
	builds := 0
	count := func(view []TypeSymbol) int {
		builds++
		return len(view)
	}
	if _, ok := cache.Cached(); ok {
		t.Fatal("empty cache reports a value")
	}
	before := cache.Get(count)
	generation := StandardPlatformSymbolGeneration()
	alreadyRequested := ConnectAPIShapeRequested()
	if !alreadyRequested && hasCaptured(StandardPlatformSymbolView()) {
		t.Fatal("captured type present before any request")
	}
	NoteApexSource("public class Poster { ConnectApi.FeedItemInput input; }")
	if !ConnectAPIShapeRequested() {
		t.Fatal("source naming ConnectApi did not request the shape")
	}
	if !hasCaptured(StandardPlatformSymbolView()) {
		t.Fatal("view lacks the captured shape after the request")
	}
	after := cache.Get(count)
	if alreadyRequested {
		if builds != 1 || after != before || StandardPlatformSymbolGeneration() != generation {
			t.Fatalf("already-requested process rebuilt: builds=%d before=%d after=%d", builds, before, after)
		}
		return
	}
	if StandardPlatformSymbolGeneration() <= generation {
		t.Fatal("view generation did not advance")
	}
	if builds != 2 || after <= before {
		t.Fatalf("cache builds=%d before=%d after=%d; want a rebuild from the larger view", builds, before, after)
	}
	if cached, ok := cache.Cached(); !ok || cached != after {
		t.Fatalf("Cached() = %d %v, want %d", cached, ok, after)
	}
	if cache.Get(count) != after || builds != 2 {
		t.Fatal("stable view rebuilt the cache")
	}
}
