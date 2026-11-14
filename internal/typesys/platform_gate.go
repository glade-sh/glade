package typesys

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/apexast"
)

// Gated platform namespaces. ConnectApi and Slack stubs are three quarters of
// the platform table, and the product supports neither namespace. The standard
// platform view leaves their stub types out until the process names the
// namespace: an Apex source or index signature that contains the word, or a
// dynamic lookup (Type.forName, typed JSON) of a qualified name in it. A
// request is sticky and only adds types, so a view after a request is the
// view the process had before the gate.
//
// Lookups that can reach a gated type without the namespace word (simple-name
// indexes, whole-table scans) read ExcludedPlatformTypes, which describes the
// left-out stubs by name and builds a stub symbol only when one is selected.
// A running VM late-binds a left-out class on a lookup miss (vm package).
const (
	gatedConnectAPINamespace uint8 = 1 << iota
	gatedSlackNamespace
	gatedAllNamespaces = gatedConnectAPINamespace | gatedSlackNamespace
)

type gatedPlatformNamespace struct {
	word string
	bit  uint8
	// named is set when a source, signature or dynamic lookup names the
	// namespace. For ConnectApi it is the captured-shape request: a named
	// ConnectApi namespace is the captured shape, as before the gate.
	named *atomic.Bool
	// included is set when the stubs must join the view without a mention: a
	// VM bound one of them, or a command reads whole tables. For ConnectApi
	// the view then carries the stub shape, as before the gate.
	included *atomic.Bool
}

var (
	slackNamespaceRequested atomic.Bool
	connectAPIStubsIncluded atomic.Bool
)

// gatedPlatformNamespaces lists each gated namespace with its lower-case word
// and request flags.
var gatedPlatformNamespaces = [...]gatedPlatformNamespace{
	{word: connectAPINamespaceWord, bit: gatedConnectAPINamespace, named: &connectAPIShapeRequested, included: &connectAPIStubsIncluded},
	{word: "slack", bit: gatedSlackNamespace, named: &slackNamespaceRequested, included: &slackNamespaceRequested},
}

// platformGateDisabled restores the ungated view for differential tests.
var platformGateDisabled atomic.Bool

// SetPlatformGateDisabled turns the namespace gate off (true) or on (false)
// for differential tests. With the gate off the view carries every stub
// namespace, exactly as before the gate; ConnectApi stays the stub shape until
// it is requested. Production code never calls it.
func SetPlatformGateDisabled(disabled bool) {
	platformGateDisabled.Store(disabled)
}

// RequestAllGatedNamespaces adds the stubs of every gated namespace to the
// platform view, without the captured ConnectApi shape. Commands that read
// whole platform tables call it first, so they see the view they saw before
// the gate.
func RequestAllGatedNamespaces() {
	for i := range gatedPlatformNamespaces {
		gatedPlatformNamespaces[i].included.Store(true)
	}
}

// RequestGatedPlatformNamespaceFor names the gated namespace a qualified type
// name (namespace.Type or a nested name) belongs to, exactly as a source that
// mentions it does (for ConnectApi, the captured shape), and reports whether
// it belongs to one. Dynamic lookups call it before they resolve the name.
func RequestGatedPlatformNamespaceFor(name string) bool {
	namespace := gatedPlatformNamespaceFor(name)
	if namespace == nil {
		return false
	}
	namespace.named.Store(true)
	return true
}

// IncludeGatedPlatformNamespaceFor adds the stubs of the gated namespace a
// qualified type name belongs to, without naming it (ConnectApi stays the stub
// shape), and reports whether it belongs to one. A VM that binds a stub class
// of the namespace calls it, so indexes derived from the view see the class
// as before the gate.
func IncludeGatedPlatformNamespaceFor(name string) bool {
	namespace := gatedPlatformNamespaceFor(name)
	if namespace == nil {
		return false
	}
	namespace.included.Store(true)
	return true
}

func gatedPlatformNamespaceFor(name string) *gatedPlatformNamespace {
	bit := gatedPlatformNamespaceBit(strings.TrimSpace(name))
	for i := range gatedPlatformNamespaces {
		if bit != 0 && gatedPlatformNamespaces[i].bit == bit {
			return &gatedPlatformNamespaces[i]
		}
	}
	return nil
}

// IsGatedPlatformTypeName reports whether a qualified type name is in a
// gated namespace, in any letter case.
func IsGatedPlatformTypeName(name string) bool {
	return gatedPlatformNamespaceBit(strings.TrimSpace(name)) != 0
}

// IsGatedPlatformNamespace reports whether namespace is a gated namespace
// word, in any letter case.
func IsGatedPlatformNamespace(namespace string) bool {
	namespace = strings.TrimSpace(namespace)
	for i := range gatedPlatformNamespaces {
		word := gatedPlatformNamespaces[i].word
		if len(namespace) == len(word) && asciiFoldHasPrefix(namespace, word) {
			return true
		}
	}
	return false
}

func gatedPlatformNamespaceBit(name string) uint8 {
	for i := range gatedPlatformNamespaces {
		word := gatedPlatformNamespaces[i].word
		if len(name) > len(word) && name[len(word)] == '.' && asciiFoldHasPrefix(name, word) {
			return gatedPlatformNamespaces[i].bit
		}
	}
	return 0
}

func includedGatedPlatformNamespaces() uint8 {
	if platformGateDisabled.Load() {
		return gatedAllNamespaces
	}
	var included uint8
	for i := range gatedPlatformNamespaces {
		if gatedPlatformNamespaces[i].named.Load() || gatedPlatformNamespaces[i].included.Load() {
			included |= gatedPlatformNamespaces[i].bit
		}
	}
	return included
}

// buildGatedStandardPlatformSymbols builds the view for one request state.
// A namespace in included contributes every spec, exactly as the ungated
// table. Any other gated namespace contributes only the specs a kept spec
// names in a signature, transitively (the referential closure); the rest are
// described by the returned ExcludedPlatformTypes.
func buildGatedStandardPlatformSymbols(connectAPI bool, included uint8) ([]TypeSymbol, ExcludedPlatformTypes) {
	specs := standardPlatformSymbolSpecList(connectAPI)
	if included == gatedAllNamespaces {
		return standardSymbolsFromSpecs(specs, newStandardSymbolLists()), ExcludedPlatformTypes{}
	}
	closure := gatedPlatformClosure(specs, included)
	kept := specs[:0:0]
	for _, spec := range specs {
		if bit := gatedPlatformNamespaceBit(spec.Name); bit != 0 && included&bit == 0 && !closure[strings.ToLower(spec.Name)] {
			continue
		}
		kept = append(kept, spec)
	}
	view := standardSymbolsFromSpecs(kept, newStandardSymbolLists())
	return view, newExcludedPlatformTypes(gatedPlatformStubs(), included, closure)
}

// gatedPlatformClosure returns the lower-case names of left-out gated specs
// that a kept spec names in its superclass, interfaces, member types or
// parameter types, and what those name in turn.
func gatedPlatformClosure(specs []StandardSymbolSpec, included uint8) map[string]bool {
	excluded := func(name string) bool {
		bit := gatedPlatformNamespaceBit(name)
		return bit != 0 && included&bit == 0
	}
	byName := make(map[string][]int)
	for i := range specs {
		if excluded(specs[i].Name) {
			key := strings.ToLower(specs[i].Name)
			byName[key] = append(byName[key], i)
		}
	}
	closure := make(map[string]bool)
	var queue []string
	note := func(text string) {
		forEachQualifiedTypeName(text, func(name string) {
			if !excluded(name) {
				return
			}
			key := strings.ToLower(name)
			if closure[key] {
				return
			}
			if _, ok := byName[key]; !ok {
				return
			}
			closure[key] = true
			queue = append(queue, key)
		})
	}
	for i := range specs {
		if !excluded(specs[i].Name) {
			forEachSpecTypeReference(&specs[i], note)
		}
	}
	for len(queue) > 0 {
		key := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		for _, i := range byName[key] {
			forEachSpecTypeReference(&specs[i], note)
		}
	}
	return closure
}

func forEachSpecTypeReference(spec *StandardSymbolSpec, visit func(string)) {
	visit(spec.SuperClass)
	for _, name := range spec.Interfaces {
		visit(name)
	}
	for _, ctor := range spec.Constructors {
		for _, typ := range ctor {
			visit(typ)
		}
	}
	for _, ctor := range spec.ConstructorSpecs {
		for _, param := range ctor.Parameters {
			visit(param.Type)
		}
	}
	for _, method := range spec.Methods {
		visit(method.ReturnType)
		for _, typ := range method.Parameters {
			visit(typ)
		}
		for _, param := range method.ParameterSpecs {
			visit(param.Type)
		}
	}
	for _, prop := range spec.Properties {
		visit(prop.Type)
	}
	for _, field := range spec.Fields {
		visit(field.Type)
	}
}

// forEachQualifiedTypeName calls visit for each dotted identifier in a type
// text such as "Map<String, List<ConnectApi.FeedItem>>".
func forEachQualifiedTypeName(text string, visit func(string)) {
	start := -1
	for i := 0; i <= len(text); i++ {
		identifier := i < len(text) && (text[i] == '.' || text[i] == '_' || text[i] >= '0' && text[i] <= '9' || text[i]|0x20 >= 'a' && text[i]|0x20 <= 'z')
		if identifier {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			visit(text[start:i])
			start = -1
		}
	}
}

// gatedPlatformStubDirectory describes every gated stub type of the stub-only
// platform table, in view order, without building its symbol. Symbols are
// built one at a time on first use and kept.
type gatedPlatformStubDirectory struct {
	entries   []gatedPlatformStubEntry
	byRuntime map[string]int
	byLocal   map[string][]int
	specs     []StandardSymbolSpec

	fieldNamesOnce sync.Once
	fieldNames     map[string]uint8
}

type gatedPlatformStubEntry struct {
	namespace string
	name      string
	runtime   string
	kind      apexast.DeclarationKind
	bit       uint8
	specs     []int
	once      sync.Once
	symbol    TypeSymbol
}

var gatedPlatformStubDirectoryState struct {
	once      sync.Once
	directory *gatedPlatformStubDirectory
}

func gatedPlatformStubs() *gatedPlatformStubDirectory {
	gatedPlatformStubDirectoryState.once.Do(func() {
		gatedPlatformStubDirectoryState.directory = buildGatedPlatformStubDirectory(standardPlatformSymbolSpecList(false))
	})
	return gatedPlatformStubDirectoryState.directory
}

func buildGatedPlatformStubDirectory(all []StandardSymbolSpec) *gatedPlatformStubDirectory {
	dir := &gatedPlatformStubDirectory{byRuntime: make(map[string]int), byLocal: make(map[string][]int)}
	byKey := make(map[string]int)
	for i := range all {
		spec := all[i]
		bit := gatedPlatformNamespaceBit(spec.Name)
		if bit == 0 || spec.Name == "" {
			continue
		}
		specIndex := len(dir.specs)
		dir.specs = append(dir.specs, spec)
		key := strings.ToLower(spec.Name)
		if at, ok := byKey[key]; ok {
			entry := &dir.entries[at]
			if spec.Kind != "" {
				entry.kind = spec.Kind
			}
			entry.specs = append(entry.specs, specIndex)
			continue
		}
		namespace, name, _ := splitStandardSymbolName(spec.Name)
		kind := spec.Kind
		if kind == "" {
			kind = apexast.DeclarationClass
		}
		byKey[key] = len(dir.entries)
		dir.entries = append(dir.entries, gatedPlatformStubEntry{namespace: namespace, name: name, kind: kind, bit: bit, specs: []int{specIndex}})
	}
	sort.Slice(dir.entries, func(i, j int) bool {
		return platformSymbolLess(dir.entries[i].namespace, dir.entries[i].name, dir.entries[j].namespace, dir.entries[j].name)
	})
	for i := range dir.entries {
		entry := &dir.entries[i]
		entry.runtime = entry.name
		if entry.namespace != "" {
			entry.runtime = entry.namespace + "." + entry.name
		}
		dir.byRuntime[strings.ToLower(entry.runtime)] = i
		local := strings.ToLower(entry.name)
		dir.byLocal[local] = append(dir.byLocal[local], i)
	}
	return dir
}

// platformSymbolLess is the order standardSymbolsFromSpecs sorts the view in.
func platformSymbolLess(namespaceA, nameA, namespaceB, nameB string) bool {
	if namespaceA == namespaceB {
		return nameA < nameB
	}
	return namespaceA < namespaceB
}

// symbol returns the stub symbol of entry i: the symbol the stub-only
// platform table holds for that type.
func (dir *gatedPlatformStubDirectory) symbol(i int) *TypeSymbol {
	entry := &dir.entries[i]
	entry.once.Do(func() {
		specs := make([]StandardSymbolSpec, 0, len(entry.specs))
		for _, at := range entry.specs {
			specs = append(specs, dir.specs[at])
		}
		entry.symbol = standardSymbolsFromSpecs(specs, nil)[0]
	})
	return &entry.symbol
}

// fieldNameIndex maps each field or property name (lower case) of a gated
// stub type to the namespaces whose stubs declare it.
func (dir *gatedPlatformStubDirectory) fieldNameIndex() map[string]uint8 {
	dir.fieldNamesOnce.Do(func() {
		names := make(map[string]uint8)
		for i := range dir.entries {
			bit := dir.entries[i].bit
			for _, at := range dir.entries[i].specs {
				spec := &dir.specs[at]
				for _, prop := range spec.Properties {
					names[strings.ToLower(prop.Name)] |= bit
				}
				for _, field := range spec.Fields {
					names[strings.ToLower(field.Name)] |= bit
				}
			}
		}
		dir.fieldNames = names
	})
	return dir.fieldNames
}

// ExcludedPlatformTypes describes the gated stub types one platform view
// leaves out, in view order. The zero value describes none.
type ExcludedPlatformTypes struct {
	dir      *gatedPlatformStubDirectory
	excluded []bool
	count    int
	included uint8
}

func newExcludedPlatformTypes(dir *gatedPlatformStubDirectory, included uint8, closure map[string]bool) ExcludedPlatformTypes {
	out := ExcludedPlatformTypes{dir: dir, excluded: make([]bool, len(dir.entries)), included: included}
	for i := range dir.entries {
		entry := &dir.entries[i]
		if included&entry.bit != 0 || closure[strings.ToLower(entry.namespace+"."+entry.name)] {
			continue
		}
		out.excluded[i] = true
		out.count++
	}
	return out
}

// Len is the number of left-out types.
func (x ExcludedPlatformTypes) Len() int {
	return x.count
}

// Each calls visit for every left-out type in view order with its index,
// namespace, simple name and kind.
func (x ExcludedPlatformTypes) Each(visit func(i int, namespace, name string, kind apexast.DeclarationKind)) {
	if x.count == 0 {
		return
	}
	for i := range x.dir.entries {
		if x.excluded[i] {
			entry := &x.dir.entries[i]
			visit(i, entry.namespace, entry.name, entry.kind)
		}
	}
}

// Symbol returns the stub symbol of the left-out type at index i (as passed
// to Each or returned by WithName). The symbol is shared and read-only.
func (x ExcludedPlatformTypes) Symbol(i int) *TypeSymbol {
	return x.dir.symbol(i)
}

// WithName returns the indexes of left-out types whose simple name equals
// name, ignoring case, in view order.
func (x ExcludedPlatformTypes) WithName(name string) []int {
	if x.count == 0 {
		return nil
	}
	var out []int
	for _, i := range x.dir.byLocal[strings.ToLower(strings.TrimSpace(name))] {
		if x.excluded[i] {
			out = append(out, i)
		}
	}
	return out
}

// HasFieldName reports whether a gated stub type of a left-out namespace
// declares a field or property with name, ignoring case. A closure type of
// such a namespace is in the view, so its names are found there either way.
func (x ExcludedPlatformTypes) HasFieldName(name string) bool {
	if x.count == 0 {
		return false
	}
	return x.dir.fieldNameIndex()[strings.ToLower(name)]&^x.included != 0
}

// StandardPlatformSymbolViewWithExcluded returns the view and the stub types
// it leaves out, from one snapshot.
func StandardPlatformSymbolViewWithExcluded() ([]TypeSymbol, ExcludedPlatformTypes) {
	snapshot := currentPlatformSymbolSnapshot()
	return snapshot.view, snapshot.excluded
}

// EachPlatformTypeName calls visit for every symbol of view and every type
// excluded leaves out of it, in the order the ungated view would list them.
// For a view symbol, symbol is set and excludedIndex is -1; for a left-out
// type, symbol is nil and excludedIndex is its index for excluded.Symbol.
func EachPlatformTypeName(view []TypeSymbol, excluded ExcludedPlatformTypes, visit func(namespace, name string, kind apexast.DeclarationKind, symbol *TypeSymbol, excludedIndex int)) {
	next := 0
	var entries []gatedPlatformStubEntry
	if excluded.count != 0 {
		entries = excluded.dir.entries
	}
	advance := func() {
		for next < len(entries) && !excluded.excluded[next] {
			next++
		}
	}
	advance()
	for i := range view {
		for next < len(entries) && platformSymbolLess(entries[next].namespace, entries[next].name, view[i].Namespace, view[i].Name) {
			visit(entries[next].namespace, entries[next].name, entries[next].kind, nil, next)
			next++
			advance()
		}
		visit(view[i].Namespace, view[i].Name, view[i].Kind, &view[i], -1)
	}
	for next < len(entries) {
		visit(entries[next].namespace, entries[next].name, entries[next].kind, nil, next)
		next++
		advance()
	}
}

// VisitStandardPlatformSymbols visits the platform symbols in view order as if
// no namespace were gated: a left-out stub type is visited in its place when
// want accepts it, and its symbol is built only then. visit returns false to
// stop. Symbols are shared and read-only.
func VisitStandardPlatformSymbols(want func(namespace, name string, kind apexast.DeclarationKind) bool, visit func(*TypeSymbol) bool) {
	view, excluded := StandardPlatformSymbolViewWithExcluded()
	stopped := false
	EachPlatformTypeName(view, excluded, func(namespace, name string, kind apexast.DeclarationKind, symbol *TypeSymbol, excludedIndex int) {
		if stopped {
			return
		}
		if symbol == nil {
			if !want(namespace, name, kind) {
				return
			}
			symbol = excluded.Symbol(excludedIndex)
		}
		if !visit(symbol) {
			stopped = true
		}
	})
}

// GatedPlatformStubSymbol returns the stub-only symbol of a gated type by its
// runtime name (Namespace.Name, nested names included), ignoring case. It is
// the symbol a view without the captured ConnectApi shape holds for the type,
// whatever the current request state. A VM registered from a gated view
// late-binds a class from it.
func GatedPlatformStubSymbol(runtimeName string) (*TypeSymbol, bool) {
	runtimeName = strings.TrimSpace(runtimeName)
	if gatedPlatformNamespaceBit(runtimeName) == 0 {
		return nil, false
	}
	dir := gatedPlatformStubs()
	i, ok := dir.byRuntime[strings.ToLower(runtimeName)]
	if !ok {
		return nil, false
	}
	return dir.symbol(i), true
}

// GatedPlatformStubRuntimeNames returns the runtime names (Namespace.Name) of
// the gated stub types whose simple name equals name, ignoring case.
func GatedPlatformStubRuntimeNames(name string) []string {
	var out []string
	EachGatedPlatformStubRuntimeName(name, func(runtimeName string) {
		out = append(out, runtimeName)
	})
	return out
}

// EachGatedPlatformStubRuntimeName calls visit with the runtime name of each
// gated stub type whose simple name equals name, ignoring case.
func EachGatedPlatformStubRuntimeName(name string, visit func(runtimeName string)) {
	dir := gatedPlatformStubs()
	matches, ok := dir.byLocal[name]
	if !ok {
		matches = dir.byLocal[strings.ToLower(strings.TrimSpace(name))]
	}
	for _, i := range matches {
		visit(dir.entries[i].runtime)
	}
}

// GatedPlatformStubRuntimeNamesInNamespace returns the runtime names of every
// gated stub type whose namespace (the qualifier before its simple name) is
// namespace, ignoring case.
func GatedPlatformStubRuntimeNamesInNamespace(namespace string) []string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil
	}
	dir := gatedPlatformStubs()
	var out []string
	for i := range dir.entries {
		if strings.EqualFold(dir.entries[i].namespace, namespace) {
			out = append(out, dir.entries[i].runtime)
		}
	}
	return out
}
