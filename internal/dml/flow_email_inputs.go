package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// flowEmailActionVerb marks a Send Email action that Salesforce rejects inside
// a record-triggered flow; it fails the triggering save like an Apex action.
const flowEmailActionVerb = "email action"

const flowEmailOrgWideInvalid = "Org-Wide Email provided is not valid. Please fix and try again."

// flowEmailActionInputs resolves Send Email inputs. An input that names a text
// template of the interview is rendered from the frame (plain and rich subject
// and body templates with $Record merge fields: flow_criteria_shapes/org.tsv
// ea, eb and ec rows, 62.0 and 67.0, both routes). Every other input keeps
// flowActionInputValues.
func (e *Engine) flowEmailActionInputs(action storage.FlowAction, source storage.Record, sourceDefinition storage.ObjectDefinition, lookupOutputs map[string]flowLookupOutput, frame *flowFrame) (map[string]storage.Value, bool) {
	if frame == nil || len(frame.textTemplates) == 0 {
		return e.flowActionInputValues(action, source, sourceDefinition, lookupOutputs, frame)
	}
	rest := action
	rest.Inputs = nil
	rendered := make(map[string]storage.Value)
	for _, input := range action.Inputs {
		reference := strings.TrimSpace(input.SourceField)
		if _, template := frame.textTemplates[flowFrameKey(reference)]; reference != "" && template {
			value, ok := e.flowFrameReferenceValue(reference, frame)
			if !ok {
				return nil, false
			}
			rendered[strings.ToLower(strings.TrimSpace(input.Name))] = value
			continue
		}
		rest.Inputs = append(rest.Inputs, input)
	}
	inputs, ok := e.flowActionInputValues(rest, source, sourceDefinition, lookupOutputs, frame)
	if !ok {
		return nil, false
	}
	for name, value := range rendered {
		inputs[name] = value
	}
	return inputs, true
}

// flowEmailOrgWideSenderFault rejects a Send Email whose senderType is
// OrgWideEmailAddress and whose senderAddress names no org-wide address of
// the org: INVALID_ARGUMENT_TYPE with the captured text (flow_criteria_shapes
// ea rows, 62.0 and 67.0, both routes).
func (e *Engine) flowEmailOrgWideSenderFault(action storage.FlowAction, inputs map[string]storage.Value) error {
	if !strings.EqualFold(strings.TrimSpace(workflowValueString(inputs["sendertype"])), "OrgWideEmailAddress") {
		return nil
	}
	address := strings.TrimSpace(workflowValueString(inputs["senderaddress"]))
	if address == "" || e.Org == nil {
		return nil
	}
	if name, ok := storage.ResolveObjectName(*e.Org, "OrgWideEmailAddress"); ok {
		for _, record := range e.Org.Objects[name].Records {
			if value, ok := record.GetField("Address"); ok && strings.EqualFold(strings.TrimSpace(workflowValueString(value)), address) {
				return nil
			}
		}
	}
	failure := dmlErrorf("INVALID_ARGUMENT_TYPE", nil, "dml: flow action %s: %s", action.Name, flowEmailOrgWideInvalid)
	return &flowElementFault{err: failure, verb: flowEmailActionVerb, code: "INVALID_ARGUMENT_TYPE", text: flowEmailOrgWideInvalid, inTest: e.flowFaultInTest()}
}
