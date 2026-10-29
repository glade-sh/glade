package vm

import "github.com/glade-sh/glade/internal/storage"

// Metadata caches are one graph with one installation lifetime. Individual
// lookup keys (names, counts, stamps or container addresses) are not authority
// to retain that graph after a host changes its schema. The storage generation
// is explicit even when a host retains equal stamp text or edits in place.
type metadataCacheLifetime struct {
	org               *storage.OrgState
	schemaGeneration  uint64
	runtimeGeneration uint64
	objectCount       int
	namespace         string
}

func metadataLifetimeForOrg(org *storage.OrgState) metadataCacheLifetime {
	if org == nil {
		return metadataCacheLifetime{}
	}
	return metadataCacheLifetime{
		org: org, schemaGeneration: org.SchemaGeneration(),
		runtimeGeneration: org.RuntimeGenerationID(),
		objectCount:       len(org.Objects), namespace: org.Namespace,
	}
}

func (vm *VM) ensureMetadataCacheGeneration() {
	// An uninstalled template may resolve built-in names while cloning. It
	// owns preparation for a future installation, not an active org lifetime.
	// SetOrg(nil) explicitly discards installed caches.
	if vm == nil || vm.Org == nil {
		return
	}
	current := metadataLifetimeForOrg(vm.Org)
	if vm.metadataCacheLifetime != current {
		// Do not invalidate the org here: the host already advanced it. This
		// only detaches this VM's old graph; siblings retain their generation.
		vm.discardMetadataCaches()
		vm.metadataCacheLifetime = current
	}
}
