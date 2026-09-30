package vm

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// AuthorizeVisualforceRecordFields checks a bounded USER_MODE field projection
// without consulting a row. A missing or unshared row cannot serve as an FLS
// decision, and this check never returns record data.
func (vm *VM) AuthorizeVisualforceRecordFields(objectName string, requested []string) ([]string, error) {
	if vm == nil || vm.Org == nil {
		return nil, fmt.Errorf("Visualforce record field authorization requires an org")
	}
	user, err := vm.visualforceExecutionUserRecord()
	if err != nil {
		return nil, err
	}
	vm.SetCurrentUser(user)
	resolvedObject, ok := vm.resolveObjectName(strings.TrimSpace(objectName))
	if !ok {
		return nil, fmt.Errorf("Visualforce record field object is unavailable")
	}
	object, ok := vm.Org.Objects[resolvedObject]
	if !ok {
		return nil, fmt.Errorf("Visualforce record field object is unavailable")
	}
	fields, err := visualforceRecordFieldNames(object.Definition, vm.Org.Namespace, requested)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("Visualforce record field authorization requires a field")
	}
	query := soql.Query{Object: resolvedObject, Fields: append([]string{"Id"}, fields...), SecurityMode: "USER_MODE"}
	if err := vm.enforceSOQLSecurity(query, ""); err != nil {
		return nil, err
	}
	if err := vm.enforceOrgShapeObjectAvailability(query); err != nil {
		return nil, err
	}
	return fields, nil
}

// ReadVisualforceRecordFields returns a fresh USER_MODE projection containing
// only the requested root fields and the record ID.
func (vm *VM) ReadVisualforceRecordFields(objectName string, id storage.ID, requested []string) (storage.Record, bool, error) {
	if vm == nil || vm.Org == nil {
		return storage.Record{}, false, fmt.Errorf("Visualforce record field read requires an org")
	}
	user, err := vm.visualforceExecutionUserRecord()
	if err != nil {
		return storage.Record{}, false, err
	}
	vm.SetCurrentUser(user)

	objectName = strings.TrimSpace(objectName)
	if objectName == "" || id == "" {
		return storage.Record{}, false, nil
	}
	resolvedObject, ok := vm.resolveObjectName(objectName)
	if !ok {
		return storage.Record{}, false, fmt.Errorf("Visualforce record field object is unavailable")
	}
	object, ok := vm.Org.Objects[resolvedObject]
	if !ok {
		return storage.Record{}, false, fmt.Errorf("Visualforce record field object is unavailable")
	}
	fields, err := visualforceRecordFieldNames(object.Definition, vm.Org.Namespace, requested)
	if err != nil {
		return storage.Record{}, false, err
	}

	projectionFields := make([]string, 0, len(fields))
	for _, field := range fields {
		if !strings.EqualFold(field, "Id") {
			projectionFields = append(projectionFields, field)
		}
	}
	if len(projectionFields) == 0 {
		projectionFields = []string{"Id"}
	}
	partial := Object(resolvedObject)
	partial.Fields["Id"] = platformScalar("Id", string(id))
	selectedFields := make([]Value, 0, len(projectionFields))
	for _, field := range projectionFields {
		selectedFields = append(selectedFields, String(field))
	}
	projection, err := vm.standardControllerFieldProjection(partial, List(selectedFields...))
	if err != nil {
		var thrown *apexThrowError
		if errors.As(err, &thrown) && strings.EqualFold(thrown.value.Type, "QueryException") &&
			stringField(thrown.value, "message") == "List has no rows for assignment to SObject" {
			return storage.Record{}, false, nil
		}
		return storage.Record{}, false, err
	}
	if projection.Kind != ValueObject {
		return storage.Record{}, false, nil
	}
	projectedID := sObjectIDFromFields(projection.Fields)
	if projectedID == "" {
		return storage.Record{}, false, nil
	}

	selected := storage.Record{
		ID:     projectedID,
		Object: resolvedObject,
		Fields: make(map[string]storage.Value, len(fields)),
	}
	for _, field := range fields {
		value, present := standardControllerFieldPathValue(projection, field)
		if !present {
			selected.Fields[field] = storage.NullValue()
			continue
		}
		storedValue := storage.Value{}
		var err error
		if metadata, ok := object.Definition.Fields[field]; ok {
			storedValue, err = storageValueFromVMForField(value, metadata)
		} else {
			storedValue, err = storageValueFromVM(value)
		}
		if err != nil {
			return storage.Record{}, false, err
		}
		selected.Fields[field] = storedValue
	}
	return selected, true, nil
}

func visualforceRecordFieldNames(definition storage.ObjectDefinition, namespace string, requested []string) ([]string, error) {
	fields := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, requestedField := range requested {
		name := strings.TrimSpace(requestedField)
		if name == "" || strings.Contains(name, ".") {
			return nil, fmt.Errorf("Visualforce record field must be a root field")
		}
		canonical, ok := storage.ResolveFieldName(definition, namespace, name)
		if !ok {
			return nil, fmt.Errorf("Visualforce record field %q is unavailable", name)
		}
		key := strings.ToLower(canonical)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		fields = append(fields, canonical)
	}
	return fields, nil
}
