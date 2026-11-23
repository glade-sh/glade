package server

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/storage"
)

// These fixed-message validation failures are captured as fetchResponse values
// by the object metadata wires at APIs 59 and 67. The record and Apex adapters
// keep their separate error contracts.
func writeObjectMetadataWireError(w http.ResponseWriter, status int, code, id, message string) {
	statusText := http.StatusText(status)
	if status == http.StatusForbidden {
		statusText = "Unexpected HTTP Status Code: 403"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	body := map[string]any{
		"errorCode":  code,
		"message":    message,
		"statusCode": status,
	}
	if id != "" {
		body["id"] = id
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"body":       body,
			"errorType":  "fetchResponse",
			"headers":    map[string]any{},
			"ok":         false,
			"status":     status,
			"statusText": statusText,
		},
	})
}

// UI API presents display types and field metadata rather than REST field types.
// Keep the conversion here so REST describe and storage semantics are unchanged.
func objectInfoFieldPayload(described map[string]any, field storage.Field) map[string]any {
	out := make(map[string]any, len(described)+1)
	for key, value := range described {
		out[key] = value
	}
	delete(out, "type")
	out["highScaleNumber"] = false
	out["compound"] = field.CompoundFieldName != "" && field.CompoundFieldName == field.APIName
	switch strings.ToUpper(field.DisplayType) {
	case "CURRENCY":
		out["dataType"] = "Currency"
	case "PERCENT":
		out["dataType"] = "Percent"
	case "PHONE":
		out["dataType"] = "Phone"
	case "EMAIL":
		out["dataType"] = "Email"
	}
	if field.Length > 0 {
		out["length"] = field.Length
	} else if field.Type == storage.FieldReference || field.Type == storage.FieldID {
		out["length"] = 18
	}
	out["searchPrefilterable"] = field.Type == storage.FieldReference && len(field.ReferenceTo) == 1 && storage.FieldFlagValue(field.Filterable, true)
	references, _ := described["referenceToInfos"].([]map[string]any)
	uiReferences := make([]map[string]any, 0, len(references))
	for _, reference := range references {
		apiName, _ := reference["apiName"].(string)
		nameFields := reference["nameFields"]
		if standard, ok := standardObjectInfoUI[apiName]; ok {
			nameFields = append([]string(nil), standard.nameFields...)
		}
		uiReferences = append(uiReferences, map[string]any{
			"apiName":    apiName,
			"nameFields": nameFields,
		})
	}
	out["referenceToInfos"] = uiReferences
	return out
}

func objectInfoChildRelationships(raw any) []map[string]any {
	relationships, _ := raw.([]map[string]any)
	out := make([]map[string]any, 0, len(relationships))
	for _, relationship := range relationships {
		relationshipName, _ := relationship["relationshipName"].(string)
		// UI API exposes named child relationships. Unnamed references such as
		// merge links remain in REST describe, but are absent from object info.
		if relationshipName == "" {
			continue
		}
		out = append(out, map[string]any{
			"childObjectApiName":  relationship["childSObject"],
			"fieldName":           relationship["field"],
			"relationshipName":    relationshipName,
			"junctionIdListNames": []string{},
			"junctionReferenceTo": []string{},
		})
	}
	return out
}

func objectInfoRelationships(org *storage.OrgState, objectName string, raw any) []map[string]any {
	items, _ := raw.([]map[string]any)
	items = append([]map[string]any(nil), items...)
	storage.VisitStandardChildRelationships(objectName, func(child string, relation storage.Relationship) {
		// Loaded definitions override the catalog, including custom relations and
		// feature-specific removals. Unloaded children need no runtime allocation.
		if _, loaded := org.Objects[child]; loaded {
			return
		}
		items = append(items, map[string]any{"childSObject": child, "field": relation.Field, "relationshipName": relation.ChildRelationship})
	})
	out := objectInfoChildRelationships(items)
	sort.Slice(out, func(i, j int) bool {
		a := out[i]["childObjectApiName"].(string) + "." + out[i]["relationshipName"].(string) + "." + out[i]["fieldName"].(string)
		b := out[j]["childObjectApiName"].(string) + "." + out[j]["relationshipName"].(string) + "." + out[j]["fieldName"].(string)
		return a < b
	})
	return out
}

// Only these request conditions have captured envelopes (r_pick_not_picklist
// and r_defaults_optional_unqualified). Do not claim the same contract for
// other fields or objects without native rows.
func capturedPicklistErrorEnvelope(req lwcbrowser.WireGetPicklistValuesRequest, errCode string) bool {
	return errCode == "INVALID_FIELD" && req.FieldAPIName == "Account.Name"
}

func capturedCreateDefaultsErrorEnvelope(req lwcbrowser.WireGetRecordCreateDefaultsRequest, errCode string) bool {
	return errCode == "ILLEGAL_QUERY_PARAMETER_VALUE" && req.ObjectAPIName == "Account" && len(req.OptionalFields) == 1 && req.OptionalFields[0] == "Name"
}

func writeObjectMetadataDataError(w http.ResponseWriter, capturedEnvelope bool, errCode, message string) bool {
	if !capturedEnvelope {
		return false
	}
	// Opaque IDs are known only for these exact captured texts. Their lookup
	// never selects the HTTP status or envelope; an unknown ID stays absent.
	id := map[string]string{"Field Name is not a picklist.": "-1797869752", "Expected '.' in all qualified names: Name is invalid": "-116886043"}[message]
	writeObjectMetadataWireError(w, http.StatusBadRequest, errCode, id, message)
	return true
}

func defaultPicklistStatusAttributes(org *storage.OrgState, objectName, fieldName string, value map[string]any) {
	if value == nil || fieldName != "Status" {
		return
	}
	setupObject, flag, outputFlag := "", "", ""
	switch objectName {
	case "Case":
		setupObject, flag, outputFlag = "CaseStatus", "IsClosed", "closed"
	case "Lead":
		setupObject, flag, outputFlag = "LeadStatus", "IsConverted", "converted"
	default:
		return
	}
	for _, record := range org.Objects[setupObject].Records {
		label := storageValueJSON(record.Fields["MasterLabel"])
		if label != value["value"] {
			continue
		}
		if state, ok := record.Fields[flag]; ok {
			value["attributes"] = map[string]any{outputFlag: storageValueJSON(state), "picklistAtrributesValueType": setupObject}
		}
		return
	}
}

// These presentation defaults are captured for the five CRM objects at APIs
// 59.0 and 67.0 in object-info cases. Other objects retain their defaults.
var standardObjectInfoUI = map[string]struct {
	nameFields []string
	themeColor string
}{
	"Account":     {[]string{"Name"}, "5867E8"},
	"Contact":     {[]string{"FirstName", "LastName", "Name"}, "9602C7"},
	"Opportunity": {[]string{"Name"}, "FF5D2D"},
	"Case":        {[]string{"CaseNumber"}, "FF538A"},
	"Lead":        {[]string{"FirstName", "LastName", "Name"}, "06A59A"},
}

func addObjectInfoUIProperties(payload map[string]any, objectAPIName string) {
	color := "747474"
	if standard, ok := standardObjectInfoUI[objectAPIName]; ok {
		payload["nameFields"] = append([]string(nil), standard.nameFields...)
		payload["layoutable"] = true
		payload["mruEnabled"] = true
		color = standard.themeColor
	}
	payload["themeInfo"] = map[string]any{
		"color":   color,
		"iconUrl": "",
	}
}
