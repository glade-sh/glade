package server

import (
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

const bulkV1Namespace = "http://www.force.com/2009/06/asyncapi/dataload"

type bulkV1Job struct {
	ID              string
	Operation       string
	Object          string
	ContentType     string
	ExternalIDField string
	State           string
	Batches         map[string]bulkV1Batch
}

type bulkV1Batch struct {
	ID      string
	JobID   string
	State   string
	Results []bulkV1Result
}

type bulkV1Result struct {
	ID      storage.ID
	Success bool
	Created bool
	Error   string
}

type bulkV1JobInfoXML struct {
	XMLName             xml.Name `xml:"jobInfo"`
	ID                  string   `xml:"id,omitempty"`
	Operation           string   `xml:"operation,omitempty"`
	Object              string   `xml:"object,omitempty"`
	ContentType         string   `xml:"contentType,omitempty"`
	ExternalIDFieldName string   `xml:"externalIdFieldName,omitempty"`
	State               string   `xml:"state,omitempty"`
}

type bulkV1BatchInfoXML struct {
	XMLName                xml.Name `xml:"batchInfo"`
	ID                     string   `xml:"id"`
	JobID                  string   `xml:"jobId"`
	State                  string   `xml:"state"`
	NumberRecordsProcessed int      `xml:"numberRecordsProcessed"`
	NumberRecordsFailed    int      `xml:"numberRecordsFailed"`
}

func (s *Server) handleBulkV1Routes(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) < 2 {
		writeSalesforceError(w, errUnknownEndpoint)
		return
	}
	switch {
	case len(parts) == 2 && parts[1] == "job":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		s.createBulkV1Job(w, r)
	case len(parts) == 3 && parts[1] == "job":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		s.updateBulkV1Job(w, r, parts[2])
	case len(parts) == 4 && parts[1] == "job" && parts[3] == "batch":
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w, http.MethodPost)
			return
		}
		s.createBulkV1Batch(w, r, parts[2])
	case len(parts) == 5 && parts[1] == "job" && parts[3] == "batch":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		s.writeBulkV1BatchStatus(w, parts[2], parts[4])
	case len(parts) == 6 && parts[1] == "job" && parts[3] == "batch" && parts[5] == "result":
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w, http.MethodGet)
			return
		}
		s.writeBulkV1BatchResult(w, parts[2], parts[4])
	default:
		writeSalesforceError(w, errUnknownEndpoint)
	}
}

func (s *Server) createBulkV1Job(w http.ResponseWriter, r *http.Request) {
	info, ok := decodeBulkV1JobInfo(w, r.Body)
	if !ok {
		return
	}
	operation := strings.TrimSpace(info.Operation)
	switch operation {
	case "insert", "upsert":
	default:
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v1 "+operation+" jobs are not implemented in the local server")
		return
	}
	if !strings.EqualFold(strings.TrimSpace(info.ContentType), "CSV") {
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v1 ingest supports CSV contentType only")
		return
	}
	objectName, ok := storage.ResolveObjectName(*s.Org, strings.TrimSpace(info.Object))
	if !ok {
		writeSalesforceError(w, errUnknownObject, "unknown object "+strings.TrimSpace(info.Object))
		return
	}
	externalIDField := strings.TrimSpace(info.ExternalIDFieldName)
	if operation == "upsert" {
		if externalIDField == "" {
			externalIDField = bulkV1DefaultExternalIDField(*s.Org, objectName)
		}
		if !bulkV1ExternalIDFieldOK(*s.Org, objectName, externalIDField) {
			writeSalesforceError(w, errInvalidField, "Bulk API v1 upsert requires a valid externalIdFieldName")
			return
		}
	}
	if s.bulkV1Jobs == nil {
		s.bulkV1Jobs = make(map[string]bulkV1Job)
	}
	s.nextBulkV1JobID++
	id := fmt.Sprintf("750%012d", s.nextBulkV1JobID)
	job := bulkV1Job{
		ID:              id,
		Operation:       operation,
		Object:          objectName,
		ContentType:     "CSV",
		ExternalIDField: externalIDField,
		State:           "Open",
		Batches:         make(map[string]bulkV1Batch),
	}
	s.bulkV1Jobs[id] = job
	writeBulkV1XML(w, http.StatusCreated, bulkV1JobXML(job))
}

func (s *Server) updateBulkV1Job(w http.ResponseWriter, r *http.Request, jobID string) {
	job, ok := s.bulkV1Jobs[jobID]
	if !ok {
		writeSalesforceError(w, errUnknownRecord, "Bulk API v1 job not found")
		return
	}
	info, ok := decodeBulkV1JobInfo(w, r.Body)
	if !ok {
		return
	}
	if strings.EqualFold(strings.TrimSpace(info.State), "Closed") {
		job.State = "Closed"
		s.bulkV1Jobs[jobID] = job
		writeBulkV1XML(w, http.StatusOK, bulkV1JobXML(job))
		return
	}
	writeSalesforceError(w, errUnsupportedFeature, "Bulk API v1 job updates support Closed state only")
}

func (s *Server) createBulkV1Batch(w http.ResponseWriter, r *http.Request, jobID string) {
	job, ok := s.bulkV1Jobs[jobID]
	if !ok {
		writeSalesforceError(w, errUnknownRecord, "Bulk API v1 job not found")
		return
	}
	if job.State == "Closed" {
		writeSalesforceError(w, errDMLFailure, "Bulk API v1 job is closed")
		return
	}
	results, ok := s.runBulkV1CSVBatch(w, r, job)
	if !ok {
		return
	}
	batchID := fmt.Sprintf("751%012d", len(job.Batches)+1)
	batch := bulkV1Batch{ID: batchID, JobID: job.ID, State: "Completed", Results: results}
	job.Batches[batchID] = batch
	s.bulkV1Jobs[jobID] = job
	writeBulkV1XML(w, http.StatusCreated, bulkV1BatchXML(batch))
}

func (s *Server) writeBulkV1BatchStatus(w http.ResponseWriter, jobID, batchID string) {
	batch, ok := s.lookupBulkV1Batch(jobID, batchID)
	if !ok {
		writeSalesforceError(w, errUnknownRecord, "Bulk API v1 batch not found")
		return
	}
	writeBulkV1XML(w, http.StatusOK, bulkV1BatchXML(batch))
}

func (s *Server) writeBulkV1BatchResult(w http.ResponseWriter, jobID, batchID string) {
	batch, ok := s.lookupBulkV1Batch(jobID, batchID)
	if !ok {
		writeSalesforceError(w, errUnknownRecord, "Bulk API v1 batch not found")
		return
	}
	var body bytes.Buffer
	writer := csv.NewWriter(&body)
	_ = writer.Write([]string{"Id", "Success", "Created", "Error"})
	for _, result := range batch.Results {
		_ = writer.Write([]string{string(result.ID), strconv.FormatBool(result.Success), strconv.FormatBool(result.Created), result.Error})
	}
	writer.Flush()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body.Bytes())
}

func (s *Server) runBulkV1CSVBatch(w http.ResponseWriter, r *http.Request, job bulkV1Job) ([]bulkV1Result, bool) {
	reader := csv.NewReader(r.Body)
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return nil, false
	}
	if len(rows) == 0 || len(rows[0]) == 0 {
		writeSalesforceError(w, errRequiredFieldMissing, "Bulk API v1 CSV header is required")
		return nil, false
	}
	results := make([]bulkV1Result, len(rows)-1)
	records := make([]storage.Record, 0, len(rows)-1)
	recordRows := make([]int, 0, len(rows)-1)
	for i, row := range rows[1:] {
		record, err := s.bulkV1RecordFromCSVRow(job.Object, rows[0], row)
		if err != nil {
			results[i] = bulkV1Result{Success: false, Error: err.Error()}
			continue
		}
		records = append(records, record)
		recordRows = append(recordRows, i)
	}
	if len(records) == 0 {
		return results, true
	}
	next := s.Org.Clone()
	engine := s.newDMLEngine(r, &next)
	var dmlResults []dml.Result
	switch job.Operation {
	case "insert":
		dmlResults = engine.Insert(records)
	case "upsert":
		dmlResults = engine.UpsertWithExternalID(records, job.ExternalIDField)
	default:
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v1 "+job.Operation+" jobs are not implemented in the local server")
		return nil, false
	}
	hasSuccess := false
	for i, result := range dmlResults {
		rowIndex := recordRows[i]
		created := result.Created
		if job.Operation == "insert" && result.Success {
			created = true
		}
		results[rowIndex] = bulkV1Result{ID: result.ID, Success: result.Success, Created: created, Error: bulkV1DMLError(result)}
		if result.Success {
			hasSuccess = true
		}
	}
	if hasSuccess {
		if err := s.commitOrg(next); err != nil {
			writeSalesforceError(w, errStoreFailure, err.Error())
			return nil, false
		}
	}
	return results, true
}

func (s *Server) bulkV1RecordFromCSVRow(objectName string, headers, row []string) (storage.Record, error) {
	object := s.Org.Objects[objectName]
	record := storage.Record{Object: objectName, Fields: make(map[string]storage.Value), ExplicitNulls: make(map[string]bool)}
	for i, header := range headers {
		fieldName := strings.TrimSpace(header)
		if fieldName == "" {
			continue
		}
		value := ""
		if i < len(row) {
			value = row[i]
		}
		if strings.EqualFold(fieldName, "Id") {
			record.ID = storage.ID(value)
			continue
		}
		if strings.Contains(fieldName, ".") {
			if err := s.applyBulkV1RelationshipCSVField(&record, object.Definition, fieldName, value); err != nil {
				return storage.Record{}, err
			}
			continue
		}
		canonical, ok := storage.ResolveFieldName(object.Definition, s.Org.Namespace, fieldName)
		if !ok {
			canonical = fieldName
		}
		field := object.Definition.Fields[canonical]
		record.Fields[canonical] = bulkV1StorageValue(field, value)
		delete(record.ExplicitNulls, canonical)
	}
	return record, nil
}

func (s *Server) applyBulkV1RelationshipCSVField(record *storage.Record, definition storage.ObjectDefinition, header, raw string) error {
	parts := strings.SplitN(header, ".", 2)
	if len(parts) != 2 {
		return nil
	}
	lookupFieldName, lookupField, ok := bulkV1RelationshipLookupField(definition, s.Org.Namespace, parts[0])
	if !ok {
		return fmt.Errorf("unknown relationship field %s", header)
	}
	if len(lookupField.ReferenceTo) == 0 {
		return fmt.Errorf("relationship field %s has no reference target", header)
	}
	parentObjectName, ok := storage.ResolveObjectName(*s.Org, lookupField.ReferenceTo[0])
	if !ok {
		return fmt.Errorf("unknown relationship target %s", lookupField.ReferenceTo[0])
	}
	parentObject := s.Org.Objects[parentObjectName]
	externalFieldName, ok := storage.ResolveFieldName(parentObject.Definition, s.Org.Namespace, parts[1])
	if !ok {
		return fmt.Errorf("unknown external id field %s.%s", parentObjectName, parts[1])
	}
	externalField := parentObject.Definition.Fields[externalFieldName]
	parent, id, matches := findExternalIDRecord(parentObject, externalFieldName, externalField, bulkV1StorageValue(externalField, raw))
	if matches != 1 {
		return fmt.Errorf("external id %s.%s matched %d records", parentObjectName, externalFieldName, matches)
	}
	if parent.ID != "" {
		id = parent.ID
	}
	record.Fields[lookupFieldName] = storage.IDValue(id)
	delete(record.ExplicitNulls, lookupFieldName)
	return nil
}

func bulkV1RelationshipLookupField(definition storage.ObjectDefinition, namespace, relationshipName string) (string, storage.Field, bool) {
	for name, field := range definition.Fields {
		if field.RelationshipName != "" && strings.EqualFold(field.RelationshipName, relationshipName) {
			if field.APIName == "" {
				field.APIName = name
			}
			return name, field, true
		}
		apiName := field.APIName
		if apiName == "" {
			apiName = name
		}
		if strings.HasSuffix(apiName, "Id") && strings.EqualFold(strings.TrimSuffix(apiName, "Id"), relationshipName) {
			return name, field, true
		}
		if strings.HasSuffix(apiName, "__c") {
			customRelationship := strings.TrimSuffix(apiName, "__c") + "__r"
			if storage.StripNamespaceToken(namespace, customRelationship) == relationshipName || strings.EqualFold(customRelationship, relationshipName) {
				return name, field, true
			}
		}
	}
	return "", storage.Field{}, false
}

func bulkV1StorageValue(field storage.Field, raw string) storage.Value {
	switch field.Type {
	case storage.FieldID, storage.FieldReference:
		if raw == "" {
			return storage.StringValue(raw)
		}
		return storage.IDValue(storage.ID(raw))
	case storage.FieldInteger:
		value, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			return storage.IntegerValue(value)
		}
	case storage.FieldBoolean:
		value, err := strconv.ParseBool(raw)
		if err == nil {
			return storage.BooleanValue(value)
		}
	case storage.FieldDecimal:
		return storage.DecimalValue(raw)
	case storage.FieldDate:
		return storage.DateValue(raw)
	case storage.FieldDateTime:
		return storage.DateTimeValue(raw)
	}
	return storage.StringValue(raw)
}

func bulkV1ExternalIDFieldOK(org storage.OrgState, objectName, rawField string) bool {
	object := org.Objects[objectName]
	fieldName, ok := storage.ResolveFieldName(object.Definition, org.Namespace, rawField)
	if !ok {
		return false
	}
	if strings.EqualFold(fieldName, "Id") {
		return true
	}
	field := object.Definition.Fields[fieldName]
	return field.ExternalID
}

func bulkV1DefaultExternalIDField(org storage.OrgState, objectName string) string {
	object := org.Objects[objectName]
	if field, ok := object.Definition.Fields["External_Id__c"]; ok && field.ExternalID {
		return "External_Id__c"
	}
	return ""
}

func decodeBulkV1JobInfo(w http.ResponseWriter, r io.Reader) (bulkV1JobInfoXML, bool) {
	data, err := io.ReadAll(r)
	if err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return bulkV1JobInfoXML{}, false
	}
	if strings.TrimSpace(string(data)) == "" {
		writeSalesforceError(w, errUnsupportedFeature, "Bulk API v1 routes are not implemented in the local server")
		return bulkV1JobInfoXML{}, false
	}
	var info bulkV1JobInfoXML
	if err := xml.NewDecoder(bytes.NewReader(data)).Decode(&info); err != nil {
		writeSalesforceError(w, errMalformedJSON, err.Error())
		return bulkV1JobInfoXML{}, false
	}
	return info, true
}

func bulkV1JobXML(job bulkV1Job) bulkV1JobInfoXML {
	return bulkV1JobInfoXML{
		XMLName:             xml.Name{Local: "jobInfo"},
		ID:                  job.ID,
		Operation:           job.Operation,
		Object:              job.Object,
		ContentType:         job.ContentType,
		ExternalIDFieldName: job.ExternalIDField,
		State:               job.State,
	}
}

func bulkV1BatchXML(batch bulkV1Batch) bulkV1BatchInfoXML {
	failed := 0
	for _, result := range batch.Results {
		if !result.Success {
			failed++
		}
	}
	return bulkV1BatchInfoXML{
		XMLName:                xml.Name{Local: "batchInfo"},
		ID:                     batch.ID,
		JobID:                  batch.JobID,
		State:                  batch.State,
		NumberRecordsProcessed: len(batch.Results),
		NumberRecordsFailed:    failed,
	}
}

func (s *Server) lookupBulkV1Batch(jobID, batchID string) (bulkV1Batch, bool) {
	job, ok := s.bulkV1Jobs[jobID]
	if !ok {
		return bulkV1Batch{}, false
	}
	batch, ok := job.Batches[batchID]
	return batch, ok
}

func bulkV1DMLError(result dml.Result) string {
	if result.Success {
		return ""
	}
	if result.Error != "" {
		return result.Error
	}
	if len(result.Errors) == 0 {
		return "DML operation failed"
	}
	parts := make([]string, 0, len(result.Errors))
	for _, item := range result.Errors {
		parts = append(parts, strings.TrimSpace(item.Message))
	}
	return strings.Join(parts, "; ")
}

func writeBulkV1XML(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(value)
}
