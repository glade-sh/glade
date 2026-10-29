package apextest

import (
	"strings"
	"sync"
	"sync/atomic"
	"weak"

	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// One immutable, registered platform prefix. Project declarations are linked
// into a frozen clone using the VM's existing method and static COW boundaries.
// A project that can shadow any prefix alias uses the original registration
// order instead; registration order is observable for overlapping declarations.
//
// Only compiled runtime cache entries hold the prefix strongly. When the last
// entry that uses it is invalidated, the prefix is collected and the next cold
// build registers a fresh one; retained heap returns to the pre-sharing level.
type standardRuntimePrefix struct {
	machine *vm.VM
	classes []vm.Class
	aliases map[string]bool
}

var standardRuntimeBase struct {
	used   atomic.Bool
	mu     sync.Mutex
	failed bool
	prefix weak.Pointer[standardRuntimePrefix]
}

func sharedStandardRuntimePrefix() *standardRuntimePrefix {
	standardRuntimeBase.mu.Lock()
	defer standardRuntimeBase.mu.Unlock()
	if prefix := standardRuntimeBase.prefix.Value(); prefix != nil {
		return prefix
	}
	if standardRuntimeBase.failed {
		return nil
	}
	machine := vm.New(nil)
	classes := passiveStandardRuntimeClasses(nil, nil)
	if err := registerBaseRuntime(machine, nil, classes, nil); err != nil {
		// Keep the legacy error path if the platform prefix cannot be
		// registered. No partially prepared prefix may be published.
		standardRuntimeBase.failed = true
		return nil
	}
	machine.FreezeClassLookup()
	aliases := make(map[string]bool, len(machine.Classes))
	for name := range machine.Classes {
		aliases[strings.ToLower(strings.TrimSpace(name))] = true
	}
	prefix := &standardRuntimePrefix{machine: machine, classes: classes, aliases: aliases}
	standardRuntimeBase.prefix = weak.Make(prefix)
	return prefix
}

type projectClassRuntime struct {
	classes      []vm.Class
	unregistered []vm.Class
	machine      *vm.VM
	// Keeps the shared prefix alive for as long as the caller's cache entry.
	prefix *standardRuntimePrefix
}

func prepareProjectClassRuntime(index typesys.Index, methods map[string]vm.Method, sources *sourceCache) projectClassRuntime {
	// A one-shot server/test invocation cannot amortize another frozen VM.
	// Own its passive declarations directly instead of retaining both pristine
	// templates and runtime copies. A later cold build establishes the cache and
	// prefix for repeated projects or source generations.
	if !standardRuntimeBase.used.Swap(true) {
		classes := compileProjectClassesWhere(index, methods, func(typesys.TypeSymbol) bool { return true }, sources)
		classes = append(classes, ownedPassiveStandardRuntimeClasses(index.Types, classes)...)
		return projectClassRuntime{classes: classes, unregistered: classes}
	}
	prefix := sharedStandardRuntimePrefix()
	canReuse := prefix != nil
	for _, typ := range index.Types {
		if !canReuse {
			// No shared prefix: use the legacy per-project registration.
			break
		}
		// RegisterClass publishes both the raw name and its namespace-qualified
		// alias, including nested names. typeSymbolRuntimeName alone treats a
		// dotted nested name as already qualified and cannot prove disjointness.
		name := strings.ToLower(strings.TrimSpace(typ.Name))
		namespace := strings.ToLower(strings.TrimSpace(typ.Namespace))
		if prefix.aliases[name] ||
			(namespace != "" && prefix.aliases[namespace+"."+name]) {
			canReuse = false
			break
		}
	}
	if !canReuse {
		classes := compileProjectClasses(index, methods, sources)
		return projectClassRuntime{classes: classes, unregistered: classes}
	}
	// A non-nil include predicate suppresses the implicit passive suffix while
	// preserving every indexed declaration (including artifacts and enums).
	private := compileProjectClassesWhere(index, methods, func(typesys.TypeSymbol) bool { return true }, sources)
	classes := make([]vm.Class, 0, len(private)+len(prefix.classes))
	classes = append(classes, private...)
	classes = append(classes, prefix.classes...)
	return projectClassRuntime{classes, private, prefix.machine.CloneRuntimeFrozenShared(nil), prefix}
}

// The first runtime takes ownership of freshly built passive declarations.
// Keep the same name exclusions and symbol-view order as the cached path, but
// do not publish or clone templates that a one-project process cannot reuse.
func ownedPassiveStandardRuntimeClasses(indexTypes []typesys.TypeSymbol, existing []vm.Class) []vm.Class {
	seen := make(map[string]bool, len(indexTypes)+len(existing))
	for _, typ := range indexTypes {
		seen[strings.ToLower(typeSymbolRuntimeName(typ))] = true
	}
	for _, class := range existing {
		seen[strings.ToLower(class.Name)] = true
	}
	var out []vm.Class
	for _, template := range buildPassiveStandardRuntimeClassTemplates(typesys.StandardPlatformSymbolView()) {
		if !seen[template.key] {
			out = append(out, template.class)
		}
	}
	return out
}
