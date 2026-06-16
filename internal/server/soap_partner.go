package server

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

const (
	partnerSOAPNamespace  = "urn:partner.soap.sforce.com"
	partnerSObjectXMLNS   = "urn:sobject.partner.soap.sforce.com"
	partnerXSITypeAttrNS  = "http://www.w3.org/2001/XMLSchema-instance"
	partnerSOAPFaultCode  = "sf:UNKNOWN_SOAP_METHOD"
	partnerMalformedFault = "sf:MALFORMED_XML"
)

func (s *Server) handlePartnerSOAP(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) < 1 || len(parts) > 2 || strings.TrimSpace(parts[0]) == "" {
		writeSOAPFault(w, http.StatusInternalServerError, "sf:UNKNOWN_SOAP_ENDPOINT", "unknown SOAP endpoint")
		return
	}
	method, payload, err := parsePartnerSOAPBody(r.Body)
	if err != nil {
		writeSOAPFault(w, http.StatusInternalServerError, partnerMalformedFault, err.Error())
		return
	}
	switch method {
	case "describeSObjects":
		s.handlePartnerDescribeSObjects(w, payload)
	case "upsert":
		s.handlePartnerUpsert(w, r, payload)
	default:
		writeSOAPFault(w, http.StatusInternalServerError, partnerSOAPFaultCode, "unknown SOAP method: "+method)
	}
}

func (s *Server) handlePartnerDescribeSObjects(w http.ResponseWriter, payload soapPartnerNode) {
	names := payload.childrenByLocal("sObjectType")
	results := make([]partnerDescribeSObjectResult, 0, len(names))
	for _, nameNode := range names {
		rawName := strings.TrimSpace(nameNode.Text)
		objectName, ok := storage.ResolveObjectName(*s.Org, rawName)
		if !ok {
			writeSOAPFault(w, http.StatusInternalServerError, "sf:INVALID_TYPE", "unknown object: "+rawName)
			return
		}
		results = append(results, partnerDescribeResultFromObject(s.Org.Objects[objectName].Definition))
	}
	writeSOAPXML(w, http.StatusOK, partnerDescribeEnvelope{
		SoapEnv: soapEnvelopeNamespace,
		Body: partnerDescribeBody{
			Response: partnerDescribeResponse{
				Xmlns:   partnerSOAPNamespace,
				Results: results,
			},
		},
	})
}

func (s *Server) handlePartnerUpsert(w http.ResponseWriter, r *http.Request, payload soapPartnerNode) {
	externalField := strings.TrimSpace(payload.firstChildText("externalIDFieldName"))
	if externalField == "" {
		writeSOAPFault(w, http.StatusInternalServerError, partnerMalformedFault, "upsert externalIDFieldName is required")
		return
	}
	nodes := payload.childrenByLocal("sObjects")
	rows := make([]partnerUpsertRow, len(nodes))
	records := make([]storage.Record, 0, len(nodes))
	recordRows := make([]int, 0, len(nodes))
	for i, node := range nodes {
		record, result := s.partnerRecordFromSOAPNode(node)
		if result != nil {
			rows[i].Result = result
			continue
		}
		rows[i].Record = record
		records = append(records, record)
		recordRows = append(recordRows, i)
	}

	var engineResults []dml.Result
	if len(records) > 0 {
		next := s.Org.Clone()
		engine := s.newDMLEngine(r, &next)
		engineResults = engine.UpsertWithExternalID(records, externalField)
		commit := false
		for _, result := range engineResults {
			if result.Success {
				commit = true
				break
			}
		}
		if commit {
			if err := s.commitOrg(next); err != nil {
				writeSOAPFault(w, http.StatusInternalServerError, "sf:SERVER", err.Error())
				return
			}
		}
	}
	for i, result := range engineResults {
		rows[recordRows[i]].Result = &result
	}

	results := make([]partnerUpsertResult, 0, len(rows))
	for _, row := range rows {
		if row.Result == nil {
			results = append(results, partnerUpsertResult{Success: false, Errors: []partnerUpsertError{{Message: "upsert row was not processed", StatusCode: "UNKNOWN_EXCEPTION"}}})
			continue
		}
		results = append(results, partnerUpsertResultFromDML(*row.Result))
	}
	writeSOAPXML(w, http.StatusOK, partnerUpsertEnvelope{
		SoapEnv: soapEnvelopeNamespace,
		Body: partnerUpsertBody{
			Response: partnerUpsertResponse{
				Xmlns:   partnerSOAPNamespace,
				Results: results,
			},
		},
	})
}

func parsePartnerSOAPBody(body io.Reader) (string, soapPartnerNode, error) {
	decoder := xml.NewDecoder(body)
	inBody := false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", soapPartnerNode{}, soapParseError{kind: soapParseMalformed, message: "malformed SOAP XML: " + err.Error()}
		}
		switch token := token.(type) {
		case xml.StartElement:
			if token.Name.Local == "Body" {
				inBody = true
				continue
			}
			if !inBody {
				continue
			}
			node, err := decodePartnerSOAPNode(decoder, token)
			if err != nil {
				return "", soapPartnerNode{}, err
			}
			return token.Name.Local, node, nil
		case xml.EndElement:
			if token.Name.Local == "Body" {
				inBody = false
			}
		}
	}
	return "", soapPartnerNode{}, soapParseError{kind: soapParseUnknownMethod, message: "unknown SOAP method"}
}

func decodePartnerSOAPNode(decoder *xml.Decoder, start xml.StartElement) (soapPartnerNode, error) {
	node := soapPartnerNode{Name: start.Name, Attrs: append([]xml.Attr(nil), start.Attr...)}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return soapPartnerNode{}, soapParseError{kind: soapParseMalformed, message: "malformed SOAP XML: unexpected end of " + start.Name.Local}
		}
		if err != nil {
			return soapPartnerNode{}, soapParseError{kind: soapParseMalformed, message: "malformed SOAP XML: " + err.Error()}
		}
		switch token := token.(type) {
		case xml.CharData:
			node.Text += string(token)
		case xml.StartElement:
			child, err := decodePartnerSOAPNode(decoder, token)
			if err != nil {
				return soapPartnerNode{}, err
			}
			node.Children = append(node.Children, child)
		case xml.EndElement:
			if token.Name.Local == start.Name.Local {
				return node, nil
			}
		}
	}
}

func (s *Server) partnerRecordFromSOAPNode(node soapPartnerNode) (storage.Record, *dml.Result) {
	objectName := partnerSOAPTypeName(node.xsiType())
	source := node
	if objectName == "" {
		if typeText := strings.TrimSpace(node.firstChildText("type")); typeText != "" {
			objectName = typeText
		}
	}
	if objectName == "" {
		for _, child := range node.Children {
			if resolved, ok := storage.ResolveObjectName(*s.Org, child.Name.Local); ok {
				objectName = resolved
				source = child
				break
			}
		}
	}
	objectName, ok := storage.ResolveObjectName(*s.Org, objectName)
	if !ok {
		result := partnerFailedDMLResult("", "unknown object: "+objectName, "INVALID_TYPE", nil)
		return storage.Record{}, &result
	}
	object := s.Org.Objects[objectName]
	record := storage.Record{
		Object:        objectName,
		Fields:        make(map[string]storage.Value),
		ExplicitNulls: make(map[string]bool),
	}
	for _, child := range source.Children {
		name := child.Name.Local
		if strings.EqualFold(name, "type") {
			continue
		}
		fieldName, field, isField := partnerResolveField(object.Definition, s.Org.Namespace, name)
		if !isField {
			relationshipField, relationship, ok := partnerRelationshipField(object.Definition, name)
			if !ok {
				record.Fields[name] = storage.StringValue(strings.TrimSpace(child.Text))
				continue
			}
			if child.xsiNil() {
				record.ExplicitNulls[relationshipField] = true
				continue
			}
			id, result := s.partnerResolveRelationshipReference(relationship, child)
			if result != nil {
				return storage.Record{}, result
			}
			record.Fields[relationshipField] = storage.IDValue(id)
			continue
		}
		if child.xsiNil() {
			record.ExplicitNulls[fieldName] = true
			delete(record.Fields, fieldName)
			continue
		}
		value, err := partnerSOAPValueForField(field, strings.TrimSpace(child.Text))
		if err != nil {
			result := partnerFailedDMLResult("", err.Error(), "INVALID_FIELD", []string{fieldName})
			return storage.Record{}, &result
		}
		if strings.EqualFold(fieldName, "Id") {
			record.ID = value.ID
			continue
		}
		record.Fields[fieldName] = value
	}
	if len(record.ExplicitNulls) == 0 {
		record.ExplicitNulls = nil
	}
	return record, nil
}

func (s *Server) partnerResolveRelationshipReference(field storage.Field, node soapPartnerNode) (storage.ID, *dml.Result) {
	targetName := partnerSOAPTypeName(node.xsiType())
	if targetName == "" {
		targetName = strings.TrimSpace(node.firstChildText("type"))
	}
	if targetName == "" && len(field.ReferenceTo) > 0 {
		targetName = field.ReferenceTo[0]
	}
	targetName, ok := storage.ResolveObjectName(*s.Org, targetName)
	if !ok {
		result := partnerFailedDMLResult("", "unknown relationship target: "+targetName, "INVALID_FIELD", []string{field.APIName})
		return "", &result
	}
	target := s.Org.Objects[targetName]
	for _, child := range node.Children {
		if strings.EqualFold(child.Name.Local, "type") {
			continue
		}
		fieldName, externalField, ok := partnerResolveField(target.Definition, s.Org.Namespace, child.Name.Local)
		if !ok {
			continue
		}
		if strings.EqualFold(fieldName, "Id") {
			return storage.ID(strings.TrimSpace(child.Text)), nil
		}
		if !externalField.ExternalID && !externalField.IDLookup {
			continue
		}
		value, err := partnerSOAPValueForField(externalField, strings.TrimSpace(child.Text))
		if err != nil {
			result := partnerFailedDMLResult("", err.Error(), "INVALID_FIELD", []string{field.APIName})
			return "", &result
		}
		_, id, matches := findExternalIDRecord(target, fieldName, externalField, value)
		switch matches {
		case 1:
			return id, nil
		case 0:
			result := partnerFailedDMLResult("", fmt.Sprintf("external id %s.%s did not match a record", targetName, fieldName), "INVALID_FIELD", []string{field.APIName})
			return "", &result
		default:
			result := partnerFailedDMLResult("", fmt.Sprintf("external id %s.%s matched multiple records", targetName, fieldName), "DUPLICATE_VALUE", []string{field.APIName})
			return "", &result
		}
	}
	result := partnerFailedDMLResult("", "relationship external id is required", "INVALID_FIELD", []string{field.APIName})
	return "", &result
}

func partnerDescribeResultFromObject(def storage.ObjectDefinition) partnerDescribeSObjectResult {
	fieldNames := make([]string, 0, len(def.Fields))
	for name := range def.Fields {
		fieldNames = append(fieldNames, name)
	}
	sort.Strings(fieldNames)
	fields := make([]partnerDescribeField, 0, len(fieldNames))
	for _, name := range fieldNames {
		field := def.Fields[name]
		apiName := field.APIName
		if apiName == "" {
			apiName = name
		}
		fields = append(fields, partnerDescribeField{
			Name:             apiName,
			Label:            partnerSOAPLabel(field.Label, apiName),
			Type:             partnerSOAPFieldType(field),
			Createable:       storage.FieldFlagValue(field.Createable, true),
			Updateable:       storage.FieldFlagValue(field.Updateable, true),
			Nillable:         storage.FieldFlagValue(field.Nillable, !field.Required),
			ExternalID:       field.ExternalID,
			Unique:           field.Unique,
			RelationshipName: field.RelationshipName,
			ReferenceTo:      append([]string(nil), field.ReferenceTo...),
		})
	}
	recordTypes := make([]partnerDescribeRecordTypeInfo, 0, len(def.RecordTypes))
	for _, recordType := range def.RecordTypes {
		recordTypes = append(recordTypes, partnerDescribeRecordTypeInfo{
			RecordTypeID:             recordType.ID,
			Name:                     recordType.Name,
			DeveloperName:            recordType.DeveloperName,
			Active:                   recordType.Active,
			Available:                recordType.Available,
			DefaultRecordTypeMapping: recordType.Default,
		})
	}
	return partnerDescribeSObjectResult{
		Name:            def.APIName,
		Label:           partnerSOAPLabel(def.Label, def.APIName),
		Custom:          strings.HasSuffix(def.APIName, "__c"),
		CustomSetting:   storage.IsCustomSettingDefinition(def),
		KeyPrefix:       def.KeyPrefix,
		Fields:          fields,
		RecordTypeInfos: recordTypes,
	}
}

func partnerSOAPValueForField(field storage.Field, raw string) (storage.Value, error) {
	switch field.Type {
	case storage.FieldID, storage.FieldReference:
		return storage.IDValue(storage.ID(raw)), nil
	case storage.FieldBoolean:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return storage.Value{}, fmt.Errorf("invalid boolean value %q", raw)
		}
		return storage.BooleanValue(value), nil
	case storage.FieldInteger:
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return storage.Value{}, fmt.Errorf("invalid integer value %q", raw)
		}
		return storage.IntegerValue(value), nil
	case storage.FieldDecimal:
		return storage.DecimalValue(raw), nil
	case storage.FieldDate:
		return storage.DateValue(raw), nil
	case storage.FieldDateTime:
		return storage.DateTimeValue(raw), nil
	case storage.FieldBlob:
		return storage.BlobValue(raw), nil
	default:
		return storage.StringValue(raw), nil
	}
}

func partnerResolveField(def storage.ObjectDefinition, namespace, name string) (string, storage.Field, bool) {
	if strings.EqualFold(name, "Id") {
		return "Id", storage.Field{APIName: "Id", Type: storage.FieldID, Nillable: storage.BoolFlag(false)}, true
	}
	canonical, ok := storage.ResolveFieldName(def, namespace, name)
	if !ok {
		return "", storage.Field{}, false
	}
	field := def.Fields[canonical]
	if field.APIName == "" {
		field.APIName = canonical
	}
	return canonical, field, true
}

func partnerRelationshipField(def storage.ObjectDefinition, relationshipName string) (string, storage.Field, bool) {
	for name, field := range def.Fields {
		if field.RelationshipName != "" && strings.EqualFold(field.RelationshipName, relationshipName) {
			if field.APIName == "" {
				field.APIName = name
			}
			return name, field, true
		}
	}
	return "", storage.Field{}, false
}

func partnerSOAPFieldType(field storage.Field) string {
	switch field.Type {
	case storage.FieldID:
		return "id"
	case storage.FieldString, storage.FieldPicklist, storage.FieldMultiPicklist:
		return "string"
	case storage.FieldBoolean:
		return "boolean"
	case storage.FieldInteger:
		return "int"
	case storage.FieldDecimal:
		return "double"
	case storage.FieldDate:
		return "date"
	case storage.FieldDateTime:
		return "datetime"
	case storage.FieldReference:
		return "reference"
	case storage.FieldBlob:
		return "base64"
	default:
		return strings.ToLower(string(field.Type))
	}
}

func partnerSOAPLabel(label, fallback string) string {
	if strings.TrimSpace(label) != "" {
		return label
	}
	return fallback
}

func partnerSOAPTypeName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if i := strings.LastIndex(raw, ":"); i >= 0 {
		return raw[i+1:]
	}
	return raw
}

func partnerUpsertResultFromDML(result dml.Result) partnerUpsertResult {
	out := partnerUpsertResult{
		Created: result.Created,
		Success: result.Success,
		ID:      result.ID,
	}
	if !result.Success {
		errors := result.Errors
		if len(errors) == 0 && result.Error != "" {
			errors = []dml.Error{{Message: result.Error, StatusCode: result.StatusCode, Fields: result.Fields}}
		}
		for _, err := range errors {
			out.Errors = append(out.Errors, partnerUpsertError{
				Message:    err.Message,
				StatusCode: err.StatusCode,
				Fields:     append([]string(nil), err.Fields...),
			})
		}
	}
	return out
}

func partnerFailedDMLResult(id storage.ID, message, statusCode string, fields []string) dml.Result {
	copiedFields := append([]string(nil), fields...)
	return dml.Result{
		ID:         id,
		Success:    false,
		Error:      message,
		StatusCode: statusCode,
		Fields:     copiedFields,
		Errors: []dml.Error{{
			Message:    message,
			StatusCode: statusCode,
			Fields:     append([]string(nil), copiedFields...),
		}},
	}
}

type partnerUpsertRow struct {
	Record storage.Record
	Result *dml.Result
}

type soapPartnerNode struct {
	Name     xml.Name          `xml:""`
	Attrs    []xml.Attr        `xml:",any,attr"`
	Text     string            `xml:",chardata"`
	Children []soapPartnerNode `xml:",any"`
}

func (n soapPartnerNode) firstDescendant(local string) (soapPartnerNode, bool) {
	if n.Name.Local == local {
		return n, true
	}
	for _, child := range n.Children {
		if found, ok := child.firstDescendant(local); ok {
			return found, true
		}
	}
	return soapPartnerNode{}, false
}

func (n soapPartnerNode) childrenByLocal(local string) []soapPartnerNode {
	var out []soapPartnerNode
	for _, child := range n.Children {
		if child.Name.Local == local {
			out = append(out, child)
		}
	}
	return out
}

func (n soapPartnerNode) firstChildText(local string) string {
	for _, child := range n.Children {
		if child.Name.Local == local {
			return child.Text
		}
	}
	return ""
}

func (n soapPartnerNode) xsiType() string {
	for _, attr := range n.Attrs {
		if attr.Name.Local == "type" && (attr.Name.Space == partnerXSITypeAttrNS || attr.Name.Space == "xsi" || attr.Name.Space == "") {
			return attr.Value
		}
	}
	return ""
}

func (n soapPartnerNode) xsiNil() bool {
	for _, attr := range n.Attrs {
		if attr.Name.Local != "nil" || (attr.Name.Space != partnerXSITypeAttrNS && attr.Name.Space != "xsi" && attr.Name.Space != "") {
			continue
		}
		value, err := strconv.ParseBool(strings.TrimSpace(attr.Value))
		return err == nil && value
	}
	return false
}

type partnerDescribeEnvelope struct {
	XMLName xml.Name            `xml:"soapenv:Envelope"`
	SoapEnv string              `xml:"xmlns:soapenv,attr"`
	Body    partnerDescribeBody `xml:"soapenv:Body"`
}

type partnerDescribeBody struct {
	Response partnerDescribeResponse `xml:"describeSObjectsResponse"`
}

type partnerDescribeResponse struct {
	Xmlns   string                         `xml:"xmlns,attr"`
	Results []partnerDescribeSObjectResult `xml:"result"`
}

type partnerDescribeSObjectResult struct {
	Name            string                          `xml:"name"`
	Label           string                          `xml:"label"`
	Custom          bool                            `xml:"custom"`
	CustomSetting   bool                            `xml:"customSetting"`
	KeyPrefix       string                          `xml:"keyPrefix"`
	Fields          []partnerDescribeField          `xml:"fields"`
	RecordTypeInfos []partnerDescribeRecordTypeInfo `xml:"recordTypeInfos"`
}

type partnerDescribeField struct {
	Name             string   `xml:"name"`
	Label            string   `xml:"label"`
	Type             string   `xml:"type"`
	Createable       bool     `xml:"createable"`
	Updateable       bool     `xml:"updateable"`
	Nillable         bool     `xml:"nillable"`
	ExternalID       bool     `xml:"externalId"`
	Unique           bool     `xml:"unique"`
	RelationshipName string   `xml:"relationshipName"`
	ReferenceTo      []string `xml:"referenceTo"`
}

type partnerDescribeRecordTypeInfo struct {
	RecordTypeID             storage.ID `xml:"recordTypeId"`
	Name                     string     `xml:"name"`
	DeveloperName            string     `xml:"developerName"`
	Active                   bool       `xml:"active"`
	Available                bool       `xml:"available"`
	DefaultRecordTypeMapping bool       `xml:"defaultRecordTypeMapping"`
}

type partnerUpsertEnvelope struct {
	XMLName xml.Name          `xml:"soapenv:Envelope"`
	SoapEnv string            `xml:"xmlns:soapenv,attr"`
	Body    partnerUpsertBody `xml:"soapenv:Body"`
}

type partnerUpsertBody struct {
	Response partnerUpsertResponse `xml:"upsertResponse"`
}

type partnerUpsertResponse struct {
	Xmlns   string                `xml:"xmlns,attr"`
	Results []partnerUpsertResult `xml:"result"`
}

type partnerUpsertResult struct {
	Created bool                 `xml:"created"`
	Success bool                 `xml:"success"`
	ID      storage.ID           `xml:"id,omitempty"`
	Errors  []partnerUpsertError `xml:"errors,omitempty"`
}

type partnerUpsertError struct {
	Message    string   `xml:"message"`
	StatusCode string   `xml:"statusCode"`
	Fields     []string `xml:"fields"`
}
