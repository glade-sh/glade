package vm

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// visualforceSetRecordLimit bounds the current local StandardSetController
// materialization. An oversized set fails explicitly rather than returning a
// partial list with misleading pagination or count values.
const visualforceSetRecordLimit = 10000

// ReadVisualforceRecords returns only the Id/Name projection that the MVP
// Visualforce set controller may render. It performs one USER_MODE query and
// applies the same sharing filter as Apex SOQL before exposing any row.
func (vm *VM) ReadVisualforceRecords(objectName string) ([]storage.Record, error) {
	if vm == nil || vm.Org == nil {
		return nil, fmt.Errorf("Visualforce set record read requires an org")
	}
	user, err := vm.visualforceExecutionUserRecord()
	if err != nil {
		return nil, err
	}
	vm.SetCurrentUser(user)
	objectName = strings.TrimSpace(objectName)
	resolved, ok := vm.resolveObjectName(objectName)
	if !ok {
		return nil, fmt.Errorf("Visualforce set record object is unavailable")
	}
	object, ok := vm.Org.Objects[resolved]
	if !ok {
		return nil, fmt.Errorf("Visualforce set record object is unavailable")
	}
	query := soql.Query{Object: resolved, Fields: []string{"Id", "Name"}, SecurityMode: "USER_MODE"}
	if err := vm.enforceSOQLSecurity(query, ""); err != nil {
		return nil, err
	}
	if err := vm.enforceOrgShapeObjectAvailability(query); err != nil {
		return nil, err
	}
	// Pre-filter in small chunks through the shared sharing path. The bound is
	// on visible, nondeleted rows only; private/deleted population must not
	// change the page outcome or disclose a threshold. This is not a SOQL
	// query per row, and it keeps pre-query memory constant.
	visibleIDs := make([]storage.ID, 0)
	chunk := make([]storage.Record, 0, 256)
	flush := func() error {
		if len(chunk) == 0 {
			return nil
		}
		allowed := vm.applySOQLSharing(query, soql.Result{Records: chunk})
		for _, record := range allowed.Records {
			visibleIDs = append(visibleIDs, record.ID)
			if len(visibleIDs) > visualforceSetRecordLimit {
				return fmt.Errorf("Visualforce set record limit exceeded")
			}
		}
		chunk = chunk[:0]
		return nil
	}
	for _, record := range object.Records {
		if record.System.IsDeleted || record.ID == "" {
			continue
		}
		chunk = append(chunk, record)
		if len(chunk) == cap(chunk) {
			if err := flush(); err != nil {
				return nil, err
			}
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if len(visibleIDs) == 0 {
		return []storage.Record{}, nil
	}
	sort.Slice(visibleIDs, func(i, j int) bool { return visibleIDs[i] < visibleIDs[j] })
	selectedIDs := make([]storage.Value, 0, len(visibleIDs))
	for _, id := range visibleIDs {
		selectedIDs = append(selectedIDs, storage.IDValue(id))
	}
	query.Where = &soql.Condition{Field: "Id", Op: "IN", Values: selectedIDs}
	executionQuery := query
	executionQuery.SecurityMode = ""
	// The SOQL engine can otherwise build a candidate slice for every stored
	// row when this object has no Id index. Give it only the already-authorized
	// bounded candidates; retain the other objects for schema resolution.
	queryOrg := visualforceSetCandidateOrg(vm.Org, resolved, object, visibleIDs)
	rows, err := soql.Execute(queryOrg, executionQuery)
	if err != nil {
		var unsupported *soql.UnsupportedFeatureError
		if errors.As(err, &unsupported) {
			return nil, &RuntimeError{Type: "UnsupportedFeature", Message: unsupported.Message}
		}
		return nil, newExceptionError("QueryException", err.Error())
	}
	rows = vm.applySOQLSharing(query, rows)
	visible := make([]storage.Record, 0, len(rows.Records))
	for _, row := range rows.Records {
		if row.System.IsDeleted || row.ID == "" {
			continue
		}
		selected := storage.Record{ID: row.ID, Object: resolved,
			Fields: map[string]storage.Value{"Name": storage.NullValue()}}
		if name, ok := row.GetField("Name"); ok {
			selected.Fields["Name"] = name
		}
		visible = append(visible, selected)
	}
	sort.Slice(visible, func(i, j int) bool { return visible[i].ID < visible[j].ID })
	return visible, nil
}

func visualforceSetCandidateOrg(org *storage.OrgState, objectName string, object storage.ObjectState, ids []storage.ID) storage.OrgState {
	queryOrg := *org
	queryOrg.Objects = make(map[string]storage.ObjectState, len(org.Objects))
	for name, state := range org.Objects {
		queryOrg.Objects[name] = state
	}
	bounded := object
	bounded.Records = make(map[storage.ID]storage.Record, len(ids))
	bounded.Indexes = nil
	for _, id := range ids {
		bounded.Records[id] = object.Records[id]
	}
	queryOrg.Objects[objectName] = bounded
	return queryOrg
}
