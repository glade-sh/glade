package vm

import (
	"errors"
	"strings"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// A lazy org (storage.OrgState.EnableLazyStandardSpread) adds catalog objects
// on first reference. Name resolution installs single objects; the paths that
// enumerate the whole schema complete the spread first so their answers match
// an eagerly built org.

// lazyStandardObject installs the catalog object name refers to, if the lazy
// spread still owes it, and reports its canonical name.
func (vm *VM) lazyStandardObject(name string) (string, bool) {
	if vm == nil || vm.Org == nil || !vm.Org.LazyStandardSpreadEnabled() {
		return "", false
	}
	return storage.EnsureLazyStandardObject(vm.Org, name)
}

// materializeLazyStandardSpread completes the spread of a lazy org. It
// reports whether anything changed so callers can retry a lookup once.
func (vm *VM) materializeLazyStandardSpread() bool {
	if vm == nil || vm.Org == nil || !vm.Org.LazyStandardSpreadEnabled() {
		return false
	}
	if !storage.MaterializeStandardObjectSpread(vm.Org) {
		return false
	}
	vm.ensureMetadataCacheGeneration()
	return true
}

// lazyResolveQueryObjects installs every catalog object a parsed query names
// before the executor sees it: the FROM object, child and semi-join subquery
// objects at any depth, and TYPEOF WHEN types. The soql package receives the
// org by value, so it cannot add objects itself.
func (vm *VM) lazyResolveQueryObjects(query soql.Query) {
	if vm == nil || vm.Org == nil || !vm.Org.LazyStandardSpreadEnabled() {
		return
	}
	vm.lazyResolveQueryObjectsRecursive(&query, 0)
}

func (vm *VM) lazyResolveQueryObjectsRecursive(query *soql.Query, depth int) {
	if query == nil || depth > 8 || !vm.Org.LazyStandardSpreadEnabled() {
		return
	}
	vm.resolveObjectName(query.Object)
	for _, typeof := range query.Typeofs {
		for typeName := range typeof.When {
			vm.resolveObjectName(typeName)
		}
	}
	for i := range query.ChildQueries {
		vm.lazyResolveQueryObjectsRecursive(&query.ChildQueries[i].Query, depth+1)
	}
	vm.lazyResolveConditionObjects(query.Where, depth)
	vm.lazyResolveConditionObjects(query.Having, depth)
}

func (vm *VM) lazyResolveConditionObjects(condition *soql.Condition, depth int) {
	if condition == nil {
		return
	}
	if condition.Subquery != nil {
		vm.lazyResolveQueryObjectsRecursive(condition.Subquery, depth+1)
	}
	for i := range condition.And {
		vm.lazyResolveConditionObjects(&condition.And[i], depth)
	}
	for i := range condition.Or {
		vm.lazyResolveConditionObjects(&condition.Or[i], depth)
	}
}

// lazySpreadCompletesMissingObject reports whether err is the executor's
// unknown-object failure on a lazy org and, if so, completes the spread so the
// caller can retry once. Any other error, or a complete org, returns false.
func (vm *VM) lazySpreadCompletesMissingObject(err error) bool {
	if err == nil || vm == nil || vm.Org == nil || !vm.Org.LazyStandardSpreadEnabled() {
		return false
	}
	var queryErr *soql.QueryError
	if !errors.As(err, &queryErr) {
		return false
	}
	if !strings.Contains(queryErr.Message, "is not supported") && !strings.Contains(err.Error(), "unknown object") {
		return false
	}
	return vm.materializeLazyStandardSpread()
}
