package lwcbrowser

type WireApexRequest struct {
	ClassName string         `json:"className"`
	Method    string         `json:"method"`
	Params    map[string]any `json:"params"`
}

type WireGetRecordRequest struct {
	RecordID string   `json:"recordId"`
	Fields   []string `json:"fields"`
}

type WireGetObjectInfoRequest struct {
	ObjectAPIName string `json:"objectApiName"`
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
	Type    string `json:"type,omitempty"`
	Message string `json:"message"`
}

type WireResponse struct {
	Data  any        `json:"data"`
	Error *WireError `json:"error"`
}
