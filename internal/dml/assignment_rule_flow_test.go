package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// AR11 observes the assigned queue through an after-save Flow formula.
// Cover both formula evaluators and all record sources sharing projection.
func TestAssignmentRuleFlowOwnerFormulaRoutes(t *testing.T) {
	for _, source := range []string{"$Record", "$Record__Prior", "NamedRecord", "LookupOutput", "unqualified"} {
		for _, evaluator := range []string{"contextual", "standalone"} {
			for _, tc := range []struct {
				name, owner, explicit string
				lowerCase             bool
				null                  bool
				want                  string
			}{
				{name: "assigned_queue", owner: "00G000000000001", want: "queue"},
				{name: "user_control", owner: string(storage.DefaultUserID), want: "user"},
				{name: "explicit_field", owner: string(storage.DefaultUserID), explicit: "00G000000000001", want: "queue"},
				{name: "lowercase_explicit_field", owner: string(storage.DefaultUserID), explicit: "00G000000000001", lowerCase: true, want: "queue"},
				{name: "explicit_null", owner: "00G000000000001", null: true, want: "user"},
			} {
				t.Run(source+"/"+evaluator+"/"+tc.name, func(t *testing.T) {
					record := storage.Record{Object: "Lead", Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: storage.ID(tc.owner)}}
					if tc.explicit != "" {
						field := "OwnerId"
						if tc.lowerCase {
							field = "ownerid"
						}
						record.Fields[field] = storage.IDValue(storage.ID(tc.explicit))
					}
					if tc.null {
						record.ExplicitNulls = map[string]bool{"OwnerId": true}
					}
					frame := newFlowFrame()
					reference := source + ".OwnerId"
					switch source {
					case "LookupOutput":
						frame.lookupOutputs[flowFrameKey(source)] = flowLookupOutput{record: record}
					case "unqualified":
						frame.records[flowFrameKey("$Record")] = record
						reference = "OwnerId"
					default:
						frame.records[flowFrameKey(source)] = record
					}
					expression := "IF(BEGINS(" + reference + ", \"00G\"), \"queue\", \"user\")"
					var value storage.Value
					var ok bool
					if evaluator == "contextual" {
						value, ok = (&Engine{}).flowContextFormulaValue(expression, frame)
					} else {
						value, ok = flowFrameFormulaValue(expression, frame)
					}
					if !ok || value.String != tc.want {
						t.Fatalf("formula=%s value=%+v resolved=%v; native owner kind=%s", expression, value, ok, tc.want)
					}
				})
			}
		}
	}
}

// AR11's record-create expression follows the non-scalar formula path in a
// record-triggered Flow; interviews and scalar expressions use the frame path.
func TestAssignmentRuleAfterSaveFlowRecordCreateRoutes(t *testing.T) {
	for _, route := range []string{"trigger_graph", "legacy_create", "interview_graph", "scalar_graph"} {
		for _, owner := range []storage.ID{"00G000000000001", storage.DefaultUserID} {
			t.Run(route+"/"+string(owner), func(t *testing.T) {
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Lead")
				storage.EnsureStandardObject(&org, "Account")
				storage.EnsureDeterministicPlatformData(&org)
				engine := NewEngine(&org)
				engine.InvocableFlow = route == "interview_graph"
				source := storage.Record{Object: "Lead", System: storage.SystemFields{OwnerID: owner}}
				frame := newFlowFrame()
				seedFlowFrame(&engine, &source, frame)
				expression := `IF(BEGINS(OwnerId, "00G"), "queue", "user")`
				if route == "scalar_graph" {
					frame.scalars[flowFrameKey("Suffix")] = storage.StringValue("")
					expression += " & Suffix"
				}
				create := storage.FlowRecordCreate{Name: "OwnerObservation", ObjectName: "Account", InputAssignments: []storage.WorkflowFieldUpdate{{Field: "Name", Formula: expression}}}
				var id storage.ID
				var err error
				if route == "legacy_create" {
					id, err = engine.executeFlowRecordCreate(create, source, org.Objects["Lead"].Definition, nil)
				} else {
					id, err = engine.flowCoreRecordCreateWithFrame(create, source, org.Objects["Lead"].Definition, frame)
				}
				if err != nil {
					t.Fatal(err)
				}
				want := "user"
				if owner == "00G000000000001" {
					want = "queue"
				}
				if got := org.Objects["Account"].Records[id].Fields["Name"].String; got != want {
					t.Fatalf("created owner observation=%q; native=%q", got, want)
				}
			})
		}
	}
}

func TestAssignmentRuleFlowCreateFormulaSourceIsolationRoute(t *testing.T) {
	for _, explicitNull := range []bool{false, true} {
		source := storage.Record{Object: "Lead", Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: "00G000000000001"}}
		if explicitNull {
			source.ExplicitNulls = map[string]bool{"OwnerId": true}
		}
		value, null, ok := flowRecordCreateAssignmentValue(storage.Field{Type: storage.FieldString}, source, storage.WorkflowFieldUpdate{Formula: `IF(BEGINS(OwnerId, "00G"), "queue", "user")`}, storage.ObjectDefinition{}, nil, nil)
		want := "queue"
		if explicitNull {
			want = "user"
		}
		if !ok || null || value.String != want {
			t.Fatalf("explicitNull=%v value=%+v null=%v resolved=%v", explicitNull, value, null, ok)
		}
		if len(source.Fields) != 0 {
			t.Fatal("formula projection changed the caller's stored fields")
		}
	}
}
