package lwcbrowser

type WireApexRequest struct {
	ClassName string `json:"className"`
	Method    string `json:"method"`
	Params    any    `json:"params"`
}

type WireGetRecordRequest struct {
	RecordID       string   `json:"recordId"`
	Fields         []string `json:"fields"`
	OptionalFields []string `json:"optionalFields"`
}

type WireGetRecordsRequest struct {
	Records []WireGetRecordsItemRequest `json:"records"`
}

type WireGetRecordsItemRequest struct {
	RecordIDs      []string `json:"recordIds"`
	Fields         []string `json:"fields"`
	OptionalFields []string `json:"optionalFields"`
}

type WireGetRecordUIRequest struct {
	RecordIDs      []string `json:"recordIds"`
	Fields         []string `json:"fields"`
	OptionalFields []string `json:"optionalFields"`
	LayoutTypes    []string `json:"layoutTypes"`
	Modes          []string `json:"modes"`
	RecordTypeID   string   `json:"recordTypeId"`
	FormFactor     string   `json:"formFactor"`
}

type WireGetObjectInfoRequest struct {
	ObjectAPIName string `json:"objectApiName"`
}

type WireGetObjectInfosRequest struct {
	ObjectAPINames []string `json:"objectApiNames"`
}

type WireGetRecordCreateDefaultsRequest struct {
	ObjectAPIName  string   `json:"objectApiName"`
	RecordTypeID   string   `json:"recordTypeId"`
	OptionalFields []string `json:"optionalFields"`
	FormFactor     string   `json:"formFactor"`
}

type WireGetLayoutRequest struct {
	ObjectAPIName string `json:"objectApiName"`
	RecordTypeID  string `json:"recordTypeId"`
	LayoutType    string `json:"layoutType"`
	Mode          string `json:"mode"`
	FormFactor    string `json:"formFactor"`
}

type WireGetPicklistValuesRequest struct {
	ObjectAPIName string `json:"objectApiName"`
	FieldAPIName  string `json:"fieldApiName"`
	RecordTypeID  string `json:"recordTypeId"`
}

type WireGetPicklistValuesByRecordTypeRequest struct {
	ObjectAPIName string `json:"objectApiName"`
	RecordTypeID  string `json:"recordTypeId"`
}

type WireGetRelatedListRecordsRequest struct {
	ParentRecordID string   `json:"parentRecordId"`
	RelatedListID  string   `json:"relatedListId"`
	Fields         []string `json:"fields"`
	OptionalFields []string `json:"optionalFields"`
	SortBy         []string `json:"sortBy"`
	PageSize       int      `json:"pageSize"`
	PageToken      string   `json:"pageToken"`
}

type WireRecordPickerSearchRequest struct {
	ObjectAPIName  string   `json:"objectApiName"`
	SearchTerm     string   `json:"searchTerm"`
	Fields         []string `json:"fields"`
	MatchingFields []string `json:"matchingFields"`
	PageSize       int      `json:"pageSize"`
}

type WireCreateRecordRequest struct {
	APIName string         `json:"apiName"`
	Fields  map[string]any `json:"fields"`
}

type WireUpdateRecordRequest struct {
	Fields map[string]any `json:"fields"`
}

type WireDeleteRecordRequest struct {
	RecordID string `json:"recordId"`
}

type WireError struct {
	Code    string         `json:"code,omitempty"`
	Type    string         `json:"type,omitempty"`
	Message string         `json:"message"`
	Body    *WireErrorBody `json:"body,omitempty"`
	Status  int            `json:"status,omitempty"`
}

type WireErrorBody struct {
	Code          string `json:"code,omitempty"`
	Message       string `json:"message"`
	ExceptionType string `json:"exceptionType,omitempty"`
	StackTrace    string `json:"stackTrace,omitempty"`
}

type WireResponse struct {
	Data  any        `json:"data"`
	Error *WireError `json:"error"`
}
