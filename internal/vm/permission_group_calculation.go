package vm

import "github.com/glade-sh/glade/internal/storage"

func (vm *VM) calculatePermissionSetGroup(value Value) (Value, error) {
	if vm.Org == nil {
		return Null, nil
	}
	values := []Value{value}
	if value.Kind == ValueList {
		values = value.List
	}
	ids := make([]storage.ID, 0, len(values))
	for _, item := range values {
		text, ok := idTextFromValue(item)
		if !isApexIDLikeValue(item) || !ok || text == "" {
			return Null, nil
		}
		ids = append(ids, storage.ID(text))
	}
	engine := vm.newDMLEngine(nil)
	err := engine.RecalculatePermissionSetGroups(ids, func(rows int) error {
		if err := vm.incrementLimit("dmlStatements", 1); err != nil {
			return err
		}
		return vm.incrementLimit("dmlRows", rows)
	})
	return Null, err
}
