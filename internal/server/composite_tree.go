package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

type compositeGraphEnvelope struct {
	Graphs []compositeGraphRequest `json:"graphs"`
}

type compositeGraphRequest struct {
	GraphID          string                        `json:"graphId"`
	CompositeRequest []compositeSubrequestEnvelope `json:"compositeRequest"`
}

func (s *Server) handleCompositeBreadth(w http.ResponseWriter, r *http.Request, version string, parts []string) {
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			base := "/services/data/" + version + "/composite"
			writeJSON(w, http.StatusOK, map[string]any{
				"resources": map[string]string{
					"composite": base,
					"batch":     base + "/batch",
					"graph":     base + "/graph",
					"sobjects":  base + "/sobjects",
					"tree":      base + "/tree/{object}",
				},
				"unsupported": []string{},
			})
		case http.MethodPost:
			body, ok := decodeCompositeRequestEnvelope(w, r)
			if !ok {
				return
			}
			s.handleGenericComposite(w, r, version, body)
		default:
			writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
		}
		return
	}
	switch parts[0] {
	case "tree":
		s.handleCompositeTree(w, r, parts[1:])
	case "graph":
		s.handleCompositeGraph(w, r, version)
	default:
		s.handleComposite(w, r, version, parts)
	}
}

func (s *Server) handleCompositeTree(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	if len(parts) < 1 || strings.TrimSpace(parts[0]) == "" {
		writeSalesforceError(w, errRequiredFieldMissing, "object name is required for Composite tree")
		return
	}
	objectName, ok := storage.ResolveObjectName(*s.Org, parts[0])
	if !ok {
		writeSalesforceError(w, errUnknownObject, "unknown object "+parts[0])
		return
	}
	var body struct {
		Records *[]map[string]json.RawMessage `json:"records"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return
	}
	if body.Records == nil || len(*body.Records) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "records is required and must contain at least one tree record")
		return
	}

	next := s.Org.Clone()
	engine := s.newDMLEngine(r, &next)
	results := make([]dml.Result, 0)
	referenceIDs := make([]string, 0)
	hasFailure := false
	for i, raw := range *body.Records {
		parent, children, referenceID, ok := s.decodeCompositeTreeRecord(w, objectName, raw, i, false)
		if !ok {
			return
		}
		parentResult := engine.Insert([]storage.Record{parent})[0]
		results = append(results, parentResult)
		referenceIDs = append(referenceIDs, referenceID)
		if !parentResult.Success {
			hasFailure = true
			for _, child := range children {
				results = append(results, dml.Result{Success: false, StatusCode: "INVALID_FIELD", Error: "dml: parent reference could not be created", Errors: []dml.Error{{StatusCode: "INVALID_FIELD", Message: "dml: parent reference could not be created", Fields: []string{child.ParentField}}}})
				referenceIDs = append(referenceIDs, child.ReferenceID)
			}
			continue
		}
		for _, child := range children {
			child.Record.Fields[child.ParentField] = storage.IDValue(parentResult.ID)
			childResult := engine.Insert([]storage.Record{child.Record})[0]
			if !childResult.Success {
				hasFailure = true
			}
			results = append(results, childResult)
			referenceIDs = append(referenceIDs, child.ReferenceID)
		}
	}
	if hasFailure {
		writeJSON(w, http.StatusBadRequest, compositeTreePayload(true, compositeAllOrNoneRollbackResults(results), referenceIDs))
		return
	}
	if err := s.commitOrg(next); err != nil {
		writeSalesforceError(w, errStoreFailure, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, compositeTreePayload(false, results, referenceIDs))
}

type compositeTreeChildRecord struct {
	Record      storage.Record
	ParentField string
	ReferenceID string
}

func (s *Server) decodeCompositeTreeRecord(w http.ResponseWriter, objectName string, raw map[string]json.RawMessage, index int, child bool) (storage.Record, []compositeTreeChildRecord, string, bool) {
	attrsRaw, ok := raw["attributes"]
	if !ok {
		writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("records[%d].attributes.referenceId is required", index))
		return storage.Record{}, nil, "", false
	}
	var attrs struct {
		ReferenceID string `json:"referenceId"`
	}
	if err := json.Unmarshal(attrsRaw, &attrs); err != nil {
		writeSalesforceError(w, errMalformedJSON, "attributes must be a JSON object")
		return storage.Record{}, nil, "", false
	}
	referenceID := strings.TrimSpace(attrs.ReferenceID)
	if referenceID == "" {
		writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("records[%d].attributes.referenceId is required", index))
		return storage.Record{}, nil, "", false
	}

	fields := make(map[string]json.RawMessage, len(raw))
	children := make([]compositeTreeChildRecord, 0)
	for name, value := range raw {
		if name == "attributes" || name == "referenceId" {
			continue
		}
		childRecords, isChild, ok := decodeCompositeTreeChildCollection(value)
		if !ok {
			writeSalesforceError(w, errMalformedJSON, "Composite tree child relationship "+name+" must contain a records array")
			return storage.Record{}, nil, "", false
		}
		if !isChild {
			fields[name] = value
			continue
		}
		if child {
			writeSalesforceError(w, errUnsupportedFeature, "nested Composite tree child relationships beyond one parent-child level are not modeled in the local server")
			return storage.Record{}, nil, "", false
		}
		childObject, parentField, ok := s.resolveCompositeTreeChildRelationship(objectName, name)
		if !ok {
			writeSalesforceError(w, errUnsupportedFeature, "Composite tree child relationship "+name+" is not modeled in local metadata")
			return storage.Record{}, nil, "", false
		}
		for childIndex, childRaw := range childRecords {
			record, nested, childReferenceID, ok := s.decodeCompositeTreeRecord(w, childObject, childRaw, childIndex, true)
			if !ok {
				return storage.Record{}, nil, "", false
			}
			if len(nested) != 0 {
				writeSalesforceError(w, errUnsupportedFeature, "nested Composite tree child relationships beyond one parent-child level are not modeled in the local server")
				return storage.Record{}, nil, "", false
			}
			children = append(children, compositeTreeChildRecord{Record: record, ParentField: parentField, ReferenceID: childReferenceID})
		}
	}
	record, err := recordFromRawFields(objectName, "", fields)
	if err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return storage.Record{}, nil, "", false
	}
	return record, children, referenceID, true
}

func decodeCompositeTreeChildCollection(raw json.RawMessage) ([]map[string]json.RawMessage, bool, bool) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, false, true
	}
	recordsRaw, ok := probe["records"]
	if !ok {
		return nil, false, true
	}
	var records []map[string]json.RawMessage
	if err := json.Unmarshal(recordsRaw, &records); err != nil {
		return nil, true, false
	}
	return records, true, true
}

func (s *Server) resolveCompositeTreeChildRelationship(parentObjectName, relationshipName string) (string, string, bool) {
	parent := s.Org.Objects[parentObjectName]
	for _, relation := range parent.Definition.Relations {
		if !strings.EqualFold(relation.ChildRelationship, relationshipName) || relation.Field == "" {
			continue
		}
		for objectName, object := range s.Org.Objects {
			field, ok := object.Definition.Fields[relation.Field]
			if !ok || field.Type != storage.FieldReference || !containsStringFold(field.ReferenceTo, parentObjectName) {
				continue
			}
			return objectName, relation.Field, true
		}
	}
	return "", "", false
}

func compositeTreePayload(hasErrors bool, results []dml.Result, referenceIDs []string) map[string]any {
	return map[string]any{
		"hasErrors": hasErrors,
		"results":   compositeResults(results, referenceIDs),
	}
}

func (s *Server) handleCompositeGraph(w http.ResponseWriter, r *http.Request, version string) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	body, ok := decodeCompositeGraphEnvelope(w, r)
	if !ok {
		return
	}
	responses := make([]map[string]any, 0, len(body.Graphs))
	for _, graph := range body.Graphs {
		next := s.Org.Clone()
		child := *s
		child.Org = &next
		child.Store = nil
		references := make(map[string]any, len(graph.CompositeRequest))
		subresponses := make([]compositeSubresponse, 0, len(graph.CompositeRequest))
		hasFailure := false
		hasMutation := false
		for _, subrequest := range graph.CompositeRequest {
			resolved, err := resolveCompositeSubrequest(subrequest, references)
			if err != nil {
				hasFailure = true
				subresponses = append(subresponses, compositeReferenceErrorResponse(subrequest.ReferenceID, err))
				continue
			}
			if isCompositeMutationMethod(resolved.Method) {
				hasMutation = true
			}
			response := child.executeCompositeSubrequest(r, version, resolved)
			if response.HTTPStatusCode >= 400 {
				hasFailure = true
			}
			references[resolved.ReferenceID] = response.Body
			subresponses = append(subresponses, response)
		}
		if hasMutation && !hasFailure {
			if err := s.commitOrg(next); err != nil {
				writeSalesforceError(w, errStoreFailure, err.Error())
				return
			}
			s.queryLocators = child.queryLocators
			s.queryOrder = child.queryOrder
			s.nextQueryID = child.nextQueryID
			s.bulkQueryJobs = child.bulkQueryJobs
			s.nextBulkJobID = child.nextBulkJobID
		}
		responses = append(responses, map[string]any{
			"graphId":      graph.GraphID,
			"isSuccessful": !hasFailure,
			"graphResponse": map[string]any{
				"compositeResponse": subresponses,
			},
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"graphs": responses})
}

func decodeCompositeGraphEnvelope(w http.ResponseWriter, r *http.Request) (compositeGraphEnvelope, bool) {
	var body struct {
		Graphs *[]compositeGraphRequest `json:"graphs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return compositeGraphEnvelope{}, false
	}
	if body.Graphs == nil || len(*body.Graphs) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "graphs is required and must contain at least one graph")
		return compositeGraphEnvelope{}, false
	}
	for i, graph := range *body.Graphs {
		if strings.TrimSpace(graph.GraphID) == "" {
			writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("graphs[%d].graphId is required", i))
			return compositeGraphEnvelope{}, false
		}
		if len(graph.CompositeRequest) == 0 {
			writeSalesforceError(w, errRequiredFieldMissing, fmt.Sprintf("graphs[%d].compositeRequest is required and must contain at least one subrequest", i))
			return compositeGraphEnvelope{}, false
		}
		if !validateCompositeSubrequests(w, graph.CompositeRequest, fmt.Sprintf("graphs[%d].compositeRequest", i), true) {
			return compositeGraphEnvelope{}, false
		}
	}
	return compositeGraphEnvelope{Graphs: *body.Graphs}, true
}

func containsStringFold(values []string, expected string) bool {
	for _, value := range values {
		if strings.EqualFold(value, expected) {
			return true
		}
	}
	return false
}
