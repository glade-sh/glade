package visualforce

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

type RemoteObjectSchema map[string][]string

type RemoteObjectsDescriptor struct {
	Namespace string
	Models    []RemoteObjectModelDescriptor
}

type RemoteObjectModelDescriptor struct {
	Name     string
	JSName   string
	Fields   []RemoteObjectFieldDescriptor
	idPrefix string
}

type RemoteObjectFieldDescriptor struct {
	Name   string
	JSName string
}

type RemoteObjectCRUDRequest struct {
	Operation  string
	ObjectName string
	Fields     map[string]any
	Criteria   map[string]any
	IDs        []string
	ViewState  string
	CSRF       string
}

type RemoteObjectCRUDResult struct {
	Success  bool                        `json:"success"`
	IDs      []string                    `json:"ids,omitempty"`
	Records  []map[string]any            `json:"records,omitempty"`
	Describe *RemoteObjectDescribeResult `json:"describe,omitempty"`
	Errors   []RemoteObjectCRUDError     `json:"errors,omitempty"`
}

type RemoteObjectDescribeResult struct {
	Name   string                        `json:"name"`
	JSName string                        `json:"jsName,omitempty"`
	Fields []RemoteObjectFieldDescriptor `json:"fields"`
}

type RemoteObjectCRUDError struct {
	Message    string   `json:"message"`
	StatusCode string   `json:"statusCode,omitempty"`
	Fields     []string `json:"fields,omitempty"`
}

func BuildRemoteObjectsDescriptor(root *MarkupNode, schema RemoteObjectSchema) (RemoteObjectsDescriptor, error) {
	node := findVisualforceComponent(root, "apex", "remoteobjects")
	if node == nil {
		return RemoteObjectsDescriptor{}, fmt.Errorf("apex:remoteObjects not found")
	}
	descriptor := RemoteObjectsDescriptor{Namespace: firstNonEmpty(node.Attribute("jsnamespace"), "RemoteObjectModel")}
	for _, modelNode := range node.Children {
		if !isVisualforceComponent(modelNode, "apex", "remoteobjectmodel") {
			continue
		}
		model, err := buildRemoteObjectModel(modelNode, schema)
		if err != nil {
			return RemoteObjectsDescriptor{}, err
		}
		descriptor.Models = append(descriptor.Models, model)
	}
	if len(descriptor.Models) == 0 {
		return RemoteObjectsDescriptor{}, fmt.Errorf("apex:remoteObjects requires at least one apex:remoteObjectModel")
	}
	sort.Slice(descriptor.Models, func(i, j int) bool {
		return descriptor.Models[i].JSName < descriptor.Models[j].JSName
	})
	return descriptor, nil
}

func DispatchRemoteObjectCRUD(org *storage.OrgState, descriptor RemoteObjectsDescriptor, req RemoteObjectCRUDRequest) RemoteObjectCRUDResult {
	operation := strings.ToLower(strings.TrimSpace(req.Operation))
	model, ok := remoteObjectModel(descriptor, req.ObjectName)
	if !ok {
		return remoteObjectFailure(fmt.Sprintf("undeclared remote object %s", strings.TrimSpace(req.ObjectName)), "INVALID_TYPE", nil)
	}
	if operation == "describe" {
		return RemoteObjectCRUDResult{Success: true, Describe: remoteObjectDescribe(model)}
	}
	if org == nil {
		return remoteObjectFailure("local org state is required", "UNSUPPORTED_FEATURE", nil)
	}
	objectName, ok := storage.ResolveObjectName(*org, model.Name)
	if !ok {
		return remoteObjectFailure(fmt.Sprintf("unknown remote object %s", model.Name), "INVALID_TYPE", nil)
	}
	allowed := remoteObjectFieldAliases(model)
	fields, explicitNulls, err := remoteObjectStorageFields(allowed, req.Fields)
	if err != nil {
		return remoteObjectFailure(err.Error(), "INVALID_FIELD", nil)
	}

	switch operation {
	case "create":
		record := storage.Record{Object: objectName, Fields: fields, ExplicitNulls: explicitNulls}
		engine := dml.NewEngine(org)
		result := engine.Insert([]storage.Record{record})[0]
		if !result.Success {
			return remoteObjectDMLFailure(result)
		}
		return RemoteObjectCRUDResult{Success: true, IDs: []string{string(result.ID)}}
	case "retrieve":
		records, err := remoteObjectRetrieve(*org, objectName, model, req.IDs)
		if err != nil {
			return remoteObjectFailure(err.Error(), "NOT_FOUND", nil)
		}
		return RemoteObjectCRUDResult{Success: true, Records: records}
	case "query":
		if unsupported := unsupportedRemoteObjectQueryCriteria(req.Criteria); len(unsupported) > 0 {
			return remoteObjectFailure(fmt.Sprintf("remote object query criteria only supports Id or ids locally; unsupported criteria: %s", strings.Join(unsupported, ", ")), "UNSUPPORTED_FEATURE", unsupported)
		}
		records, err := remoteObjectQuery(*org, objectName, model, remoteObjectQueryIDs(req))
		if err != nil {
			return remoteObjectFailure(err.Error(), "NOT_FOUND", nil)
		}
		return RemoteObjectCRUDResult{Success: true, Records: records}
	case "update":
		ids := remoteObjectRequestIDs(req)
		if len(ids) == 0 {
			return remoteObjectFailure("remote object update requires an Id", "MISSING_ARGUMENT", []string{"Id"})
		}
		record := storage.Record{ID: storage.ID(ids[0]), Object: objectName, Fields: fields, ExplicitNulls: explicitNulls}
		engine := dml.NewEngine(org)
		result := engine.Update([]storage.Record{record})[0]
		if !result.Success {
			return remoteObjectDMLFailure(result)
		}
		return RemoteObjectCRUDResult{Success: true, IDs: []string{string(result.ID)}}
	case "delete", "del":
		ids := remoteObjectRequestIDs(req)
		if len(ids) == 0 {
			return remoteObjectFailure("remote object delete requires an Id", "MISSING_ARGUMENT", []string{"Id"})
		}
		record := storage.Record{ID: storage.ID(ids[0]), Object: objectName}
		engine := dml.NewEngine(org)
		result := engine.Delete([]storage.Record{record})[0]
		if !result.Success {
			return remoteObjectDMLFailure(result)
		}
		return RemoteObjectCRUDResult{Success: true, IDs: []string{string(result.ID)}}
	default:
		return remoteObjectFailure(fmt.Sprintf("unsupported remote object operation %s", strings.TrimSpace(req.Operation)), "UNSUPPORTED_FEATURE", nil)
	}
}

func RenderRemoteObjectsScript(descriptor RemoteObjectsDescriptor) string {
	namespace := firstNonEmpty(descriptor.Namespace, "RemoteObjectModel")
	namespaceID := jsIdentifier(namespace)
	builder := strings.Builder{}
	builder.WriteString(`<script>(function(window){`)
	builder.WriteString(`window.__gladeRemoteObjects=window.__gladeRemoteObjects||function(operation,objectName,fields,callback,criteria){
fields=fields||{};criteria=criteria||{};
var ids=[];
if(fields.Id){ids=[String(fields.Id)];}else if(fields.id){ids=[String(fields.id)];}
else if(criteria.Id){ids=[String(criteria.Id)];}else if(criteria.id){ids=[String(criteria.id)];}
else if(Array.isArray(criteria.ids)){ids=criteria.ids.map(String);}
var read=function(name){var el=document.querySelector('input[name="'+name+'"]');return el?el.value:"";};
var transportStatus=false;
return fetch(window.location.pathname.replace(/\/$/,"")+"/remoteObjects",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({operation:operation,objectName:objectName,fields:fields,criteria:criteria,ids:ids,viewState:read("` + ViewStateFormFieldName() + `"),csrf:read("__vf_csrf")})})
.then(function(response){transportStatus=response.ok;return response.json();})
.then(function(result){
var event={status:!!(result&&result.success),type:"remoteObjects"};
Object.defineProperty(event,"transportStatus",{value:transportStatus});
if(callback){callback(result,event);}
return result;
}).catch(function(err){
var result={success:false,errors:[{message:String(err)}]};
var event={status:false,type:"remoteObjects",message:String(err)};
Object.defineProperty(event,"transportStatus",{value:false});
if(callback){callback(result,event);}
return result;
});
};
var remoteObjectTID=1;
var retrieve=function(objectName,criteria,callback){
if(typeof criteria==="function"){criteria=criteria();}
var tid=remoteObjectTID++;
return window.__gladeRemoteObjects("query",objectName,{},function(result,transport){
var success=!!(result&&result.success);
var failure=result&&result.errors&&result.errors[0];
var error=success?null:new Error(failure&&failure.message||"Remote object retrieve failed");
var records=success?(result.records||[]):null;
var event={status:!!(transport&&transport.transportStatus),type:"rpc",tid:tid,action:"JavaScriptSObjectBaseController",method:"retrieve"};
if(!event.status){event.type="exception";event.message=transport&&transport.message||error&&error.message;}
if(typeof callback==="function"){callback(error,records,event);}
},criteria||{});
};`)
	builder.WriteString(`var `)
	builder.WriteString(namespaceID)
	builder.WriteString(` = window.`)
	builder.WriteString(namespaceID)
	builder.WriteString(` || {};`)
	builder.WriteString(`window.`)
	builder.WriteString(namespaceID)
	builder.WriteString(` = `)
	builder.WriteString(namespaceID)
	builder.WriteString(`;`)
	for _, model := range descriptor.Models {
		jsName := jsIdentifier(model.JSName)
		fieldNames := make([]string, 0, len(model.Fields))
		aliases := map[string]string{"Id": "Id"}
		for _, field := range model.Fields {
			fieldNames = append(fieldNames, field.Name)
			aliases[field.Name] = field.Name
			if field.JSName != "" {
				aliases[field.JSName] = field.Name
			}
		}
		aliasJSON, _ := json.Marshal(aliases)
		builder.WriteString(`(function(){var aliases=`)
		builder.Write(aliasJSON)
		builder.WriteString(`;var idPrefix=`)
		builder.WriteString(jsString(model.idPrefix))
		builder.WriteString(`;var canonical=function(name){return Object.prototype.hasOwnProperty.call(aliases,name)?aliases[name]:name;};`)
		builder.WriteString(namespaceID)
		builder.WriteString(`.`)
		builder.WriteString(jsName)
		builder.WriteString(` = function(fields){this.fields=Object.create(null);var self=this;Object.keys(fields||{}).forEach(function(name){self.set(name,fields[name]);});};`)
		builder.WriteString(namespaceID + `.` + jsName + `.prototype.get=function(name){var value=this.fields[canonical(name)];return value===null?undefined:value;};`)
		builder.WriteString(namespaceID + `.` + jsName + `.prototype.set=function(name,value){var field=canonical(name);if(field==="Id"&&value!=null&&idPrefix&&(typeof value!=="string"||!(/^[a-zA-Z0-9]{15}([a-zA-Z0-9]{3})?$/.test(value))||value.slice(0,3)!==idPrefix)){throw new Error("Invalid id provided: "+value+" Expected value with prefix: "+idPrefix+" and length 15 or 18");}this.fields[field]=value;};`)
		builder.WriteString(namespaceID)
		builder.WriteString(`.`)
		builder.WriteString(jsName)
		builder.WriteString(`.objectName = `)
		builder.WriteString(jsString(model.Name))
		builder.WriteString(`;`)
		builder.WriteString(namespaceID)
		builder.WriteString(`.`)
		builder.WriteString(jsName)
		builder.WriteString(`.fields = `)
		builder.WriteString(jsStringArray(fieldNames))
		builder.WriteString(`;`)
		builder.WriteString(namespaceID)
		builder.WriteString(`.`)
		builder.WriteString(jsName)
		builder.WriteString(`.describe = function(callback){return window.__gladeRemoteObjects("describe",`)
		builder.WriteString(jsString(model.Name))
		builder.WriteString(`,{},callback);};`)
		builder.WriteString(namespaceID)
		builder.WriteString(`.`)
		builder.WriteString(jsName)
		builder.WriteString(`.query = function(criteria,callback){if(typeof criteria=="function"){callback=criteria;criteria={};}return window.__gladeRemoteObjects("query",`)
		builder.WriteString(jsString(model.Name))
		builder.WriteString(`,{},callback,criteria||{});};`)
		// Keep the existing one-callback local retrieval path while adding the
		// captured criteria/callback signature and its three-argument callback.
		builder.WriteString(namespaceID + `.` + jsName + `.prototype.retrieve=function(criteria,callback){if(typeof callback!=="function"){return window.__gladeRemoteObjects("retrieve",`)
		builder.WriteString(jsString(model.Name))
		builder.WriteString(`,this.fields,criteria);}return retrieve(`)
		builder.WriteString(jsString(model.Name))
		builder.WriteString(`,criteria,callback);};`)
		for _, method := range []string{"create", "update", "del"} {
			builder.WriteString(namespaceID)
			builder.WriteString(`.`)
			builder.WriteString(jsName)
			builder.WriteString(`.prototype.`)
			builder.WriteString(method)
			builder.WriteString(` = function(callback){return window.__gladeRemoteObjects(`)
			builder.WriteString(jsString(method))
			builder.WriteString(`,`)
			builder.WriteString(jsString(model.Name))
			builder.WriteString(`,this.fields,callback);};`)
		}
		builder.WriteString(`})();`)
	}
	builder.WriteString(`})(window);</script>`)
	return builder.String()
}

func renderApexRemoteObjects(node *MarkupNode, ctx *RenderContext) (string, error) {
	schema := remoteObjectRenderSchema(ctx)
	descriptor, err := BuildRemoteObjectsDescriptor(node, schema)
	if err != nil && schema != nil {
		descriptor, err = BuildRemoteObjectsDescriptor(node, nil)
	}
	if err != nil {
		return "", err
	}
	if ctx != nil && ctx.VM != nil && ctx.VM.Org != nil {
		for i := range descriptor.Models {
			if name, ok := storage.ResolveObjectName(*ctx.VM.Org, descriptor.Models[i].Name); ok {
				descriptor.Models[i].idPrefix = strings.TrimSpace(ctx.VM.Org.Objects[name].Definition.KeyPrefix)
			}
		}
	}
	children, err := renderRemoteObjectsVisibleChildren(node, ctx)
	if err != nil {
		return "", err
	}
	return RenderRemoteObjectsScript(descriptor) + children, nil
}

func renderRemoteObjectsVisibleChildren(node *MarkupNode, ctx *RenderContext) (string, error) {
	builder := strings.Builder{}
	for _, child := range node.Children {
		if isRemoteObjectDeclaration(child) {
			continue
		}
		rendered, err := renderMarkupNode(child, ctx)
		if err != nil {
			return "", err
		}
		builder.WriteString(rendered)
	}
	return builder.String(), nil
}

func isRemoteObjectDeclaration(node *MarkupNode) bool {
	return isVisualforceComponent(node, "apex", "remoteobjectmodel") ||
		isVisualforceComponent(node, "apex", "remoteobjectfield")
}

func remoteObjectRenderSchema(ctx *RenderContext) RemoteObjectSchema {
	if ctx == nil || ctx.VM == nil || ctx.VM.Org == nil {
		return nil
	}
	return RemoteObjectSchemaFromOrg(ctx.VM.Org)
}

func RemoteObjectSchemaFromOrg(org *storage.OrgState) RemoteObjectSchema {
	if org == nil {
		return nil
	}
	schema := RemoteObjectSchema{}
	for objectName, object := range org.Objects {
		apiName := firstNonEmpty(object.Definition.APIName, objectName)
		if strings.TrimSpace(apiName) == "" {
			continue
		}
		fields := make([]string, 0, len(object.Definition.Fields))
		for fieldName, field := range object.Definition.Fields {
			apiName := firstNonEmpty(field.APIName, fieldName)
			if strings.TrimSpace(apiName) != "" {
				fields = append(fields, apiName)
			}
		}
		sort.Strings(fields)
		schema[apiName] = fields
	}
	if len(schema) == 0 {
		return nil
	}
	return schema
}

func remoteObjectModel(descriptor RemoteObjectsDescriptor, name string) (RemoteObjectModelDescriptor, bool) {
	name = strings.TrimSpace(name)
	for _, model := range descriptor.Models {
		if strings.EqualFold(model.Name, name) || strings.EqualFold(model.JSName, name) {
			return model, true
		}
	}
	return RemoteObjectModelDescriptor{}, false
}

func remoteObjectFieldAliases(model RemoteObjectModelDescriptor) map[string]string {
	allowed := map[string]string{"id": "Id"}
	for _, field := range model.Fields {
		if field.Name == "" {
			continue
		}
		allowed[strings.ToLower(field.Name)] = field.Name
		if strings.TrimSpace(field.JSName) != "" {
			allowed[strings.ToLower(field.JSName)] = field.Name
		}
	}
	return allowed
}

func remoteObjectStorageFields(allowed map[string]string, raw map[string]any) (map[string]storage.Value, map[string]bool, error) {
	fields := map[string]storage.Value{}
	explicitNulls := map[string]bool{}
	for name, rawValue := range raw {
		canonical, ok := allowed[strings.ToLower(strings.TrimSpace(name))]
		if !ok {
			return nil, nil, fmt.Errorf("undeclared remote field %s", name)
		}
		if strings.EqualFold(canonical, "Id") {
			continue
		}
		value := remoteObjectStorageValue(rawValue)
		if value.Kind == storage.ValueNull {
			explicitNulls[canonical] = true
			continue
		}
		fields[canonical] = value
	}
	return fields, explicitNulls, nil
}

func remoteObjectStorageValue(value any) storage.Value {
	switch typed := value.(type) {
	case nil:
		return storage.NullValue()
	case string:
		return storage.StringValue(typed)
	case bool:
		return storage.BooleanValue(typed)
	case int:
		return storage.IntegerValue(int64(typed))
	case int64:
		return storage.IntegerValue(typed)
	case float64:
		if math.Trunc(typed) != typed {
			return storage.DecimalValue(strconv.FormatFloat(typed, 'f', -1, 64))
		}
		return storage.IntegerValue(int64(typed))
	case json.Number:
		text := typed.String()
		if strings.ContainsAny(text, ".eE") {
			return storage.DecimalValue(text)
		}
		parsed, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return storage.DecimalValue(text)
		}
		return storage.IntegerValue(parsed)
	case []any:
		values := make([]storage.Value, 0, len(typed))
		for _, item := range typed {
			values = append(values, remoteObjectStorageValue(item))
		}
		return storage.ListValue(values...)
	default:
		return storage.StringValue(fmt.Sprintf("%v", typed))
	}
}

func remoteObjectRequestIDs(req RemoteObjectCRUDRequest) []string {
	if len(req.IDs) > 0 {
		return req.IDs
	}
	if raw, ok := req.Fields["Id"]; ok {
		return []string{fmt.Sprintf("%v", raw)}
	}
	if raw, ok := req.Fields["id"]; ok {
		return []string{fmt.Sprintf("%v", raw)}
	}
	return nil
}

func remoteObjectQueryIDs(req RemoteObjectCRUDRequest) []string {
	if ids := remoteObjectRequestIDs(req); len(ids) > 0 {
		return ids
	}
	for _, key := range []string{"Id", "id"} {
		if raw, ok := req.Criteria[key]; ok {
			return []string{fmt.Sprintf("%v", raw)}
		}
	}
	if raw, ok := req.Criteria["ids"]; ok {
		switch typed := raw.(type) {
		case []any:
			ids := make([]string, 0, len(typed))
			for _, item := range typed {
				ids = append(ids, fmt.Sprintf("%v", item))
			}
			return ids
		case []string:
			return append([]string(nil), typed...)
		}
	}
	return nil
}

func unsupportedRemoteObjectQueryCriteria(criteria map[string]any) []string {
	if len(criteria) == 0 {
		return nil
	}
	var unsupported []string
	for key := range criteria {
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "id", "ids":
			continue
		default:
			unsupported = append(unsupported, key)
		}
	}
	sort.Strings(unsupported)
	return unsupported
}

func remoteObjectRetrieve(org storage.OrgState, objectName string, model RemoteObjectModelDescriptor, ids []string) ([]map[string]any, error) {
	object, ok := org.Objects[objectName]
	if !ok {
		return nil, fmt.Errorf("unknown remote object %s", objectName)
	}
	out := make([]map[string]any, 0, len(ids))
	for _, rawID := range ids {
		id := storage.ID(strings.TrimSpace(rawID))
		if id == "" {
			continue
		}
		_, record, ok := storage.LookupRecordByID(object.Records, id)
		if !ok || record.System.IsDeleted {
			continue
		}
		row := map[string]any{"Id": string(record.ID)}
		for _, field := range model.Fields {
			if value, ok := record.GetField(field.Name); ok {
				row[field.Name] = remoteObjectJSONValue(value)
			} else if record.HasExplicitNull(field.Name) {
				row[field.Name] = nil
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func remoteObjectQuery(org storage.OrgState, objectName string, model RemoteObjectModelDescriptor, ids []string) ([]map[string]any, error) {
	if len(ids) > 0 {
		return remoteObjectRetrieve(org, objectName, model, ids)
	}
	object, ok := org.Objects[objectName]
	if !ok {
		return nil, fmt.Errorf("unknown remote object %s", objectName)
	}
	recordIDs := make([]string, 0, len(object.Records))
	for id, record := range object.Records {
		if record.System.IsDeleted {
			continue
		}
		recordIDs = append(recordIDs, string(id))
	}
	sort.Strings(recordIDs)
	return remoteObjectRetrieve(org, objectName, model, recordIDs)
}

func remoteObjectDescribe(model RemoteObjectModelDescriptor) *RemoteObjectDescribeResult {
	fields := make([]RemoteObjectFieldDescriptor, len(model.Fields))
	copy(fields, model.Fields)
	return &RemoteObjectDescribeResult{
		Name:   model.Name,
		JSName: model.JSName,
		Fields: fields,
	}
}

func remoteObjectJSONValue(value storage.Value) any {
	switch value.Kind {
	case storage.ValueNull:
		return nil
	case storage.ValueString, storage.ValueDate, storage.ValueDateTime, storage.ValueBlob:
		return value.String
	case storage.ValueInteger:
		return value.Integer
	case storage.ValueBoolean:
		return value.Boolean
	case storage.ValueDecimal:
		return value.Decimal
	case storage.ValueID:
		return string(value.ID)
	case storage.ValueList:
		out := make([]any, 0, len(value.List))
		for _, item := range value.List {
			out = append(out, remoteObjectJSONValue(item))
		}
		return out
	default:
		return value.String
	}
}

func remoteObjectDMLFailure(result dml.Result) RemoteObjectCRUDResult {
	if len(result.Errors) == 0 {
		return remoteObjectFailure(result.Error, result.StatusCode, result.Fields)
	}
	errors := make([]RemoteObjectCRUDError, 0, len(result.Errors))
	for _, item := range result.Errors {
		errors = append(errors, RemoteObjectCRUDError{Message: item.Message, StatusCode: item.StatusCode, Fields: append([]string(nil), item.Fields...)})
	}
	return RemoteObjectCRUDResult{Success: false, Errors: errors}
}

func remoteObjectFailure(message, statusCode string, fields []string) RemoteObjectCRUDResult {
	return RemoteObjectCRUDResult{Success: false, Errors: []RemoteObjectCRUDError{{
		Message:    message,
		StatusCode: statusCode,
		Fields:     append([]string(nil), fields...),
	}}}
}

func buildRemoteObjectModel(node *MarkupNode, schema RemoteObjectSchema) (RemoteObjectModelDescriptor, error) {
	name := strings.TrimSpace(node.Attribute("name"))
	if name == "" {
		return RemoteObjectModelDescriptor{}, fmt.Errorf("apex:remoteObjectModel requires name")
	}
	validateSchema := schema != nil
	declaredFields := map[string]bool{}
	if validateSchema {
		var ok bool
		declaredFields, ok = remoteObjectSchemaFields(schema, name)
		if !ok {
			return RemoteObjectModelDescriptor{}, fmt.Errorf(`Wrong type for attribute <apex:remoteObjectModel name="">. Expected valid entity name, found %s`, name)
		}
	}
	model := RemoteObjectModelDescriptor{Name: name, JSName: firstNonEmpty(node.Attribute("jsshorthand"), name)}
	seen := map[string]bool{}
	addField := func(fieldName, jsName string) error {
		fieldName = strings.TrimSpace(fieldName)
		if fieldName == "" {
			return nil
		}
		key := strings.ToLower(fieldName)
		if validateSchema && !declaredFields[key] {
			return fmt.Errorf("Invalid field '%s.%s' specified. Ensure that you use the full API name for any custom fields.", name, fieldName)
		}
		if seen[key] {
			return fmt.Errorf("Duplicate field(s) '%s' specified for sObject '%s'.", fieldName, name)
		}
		seen[key] = true
		model.Fields = append(model.Fields, RemoteObjectFieldDescriptor{Name: fieldName, JSName: firstNonEmpty(jsName, fieldName)})
		return nil
	}
	for _, fieldName := range splitCSV(node.Attribute("fields")) {
		if err := addField(fieldName, fieldName); err != nil {
			return RemoteObjectModelDescriptor{}, err
		}
	}
	for _, child := range node.Children {
		if !isVisualforceComponent(child, "apex", "remoteobjectfield") {
			continue
		}
		if err := addField(child.Attribute("name"), child.Attribute("jsshorthand")); err != nil {
			return RemoteObjectModelDescriptor{}, err
		}
	}
	return model, nil
}

func remoteObjectSchemaFields(schema RemoteObjectSchema, objectName string) (map[string]bool, bool) {
	for name, fields := range schema {
		if !strings.EqualFold(strings.TrimSpace(name), objectName) {
			continue
		}
		out := map[string]bool{}
		for _, field := range fields {
			if field = strings.TrimSpace(field); field != "" {
				out[strings.ToLower(field)] = true
			}
		}
		return out, true
	}
	return nil, false
}

func findVisualforceComponent(node *MarkupNode, namespace, name string) *MarkupNode {
	if isVisualforceComponent(node, namespace, name) {
		return node
	}
	if node == nil {
		return nil
	}
	for _, child := range node.Children {
		if found := findVisualforceComponent(child, namespace, name); found != nil {
			return found
		}
	}
	return nil
}

func isVisualforceComponent(node *MarkupNode, namespace, name string) bool {
	return node != nil &&
		node.Type == MarkupNodeElement &&
		strings.EqualFold(node.Namespace, namespace) &&
		strings.EqualFold(node.Name, name)
}

func jsIdentifier(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_"
	}
	var builder strings.Builder
	for i, r := range value {
		if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9' {
			builder.WriteRune(r)
			continue
		}
		if i == 0 {
			builder.WriteByte('_')
		}
	}
	if builder.Len() == 0 {
		return "_"
	}
	return builder.String()
}

func jsStringArray(values []string) string {
	builder := strings.Builder{}
	builder.WriteString("[")
	for i, value := range values {
		if i > 0 {
			builder.WriteString(",")
		}
		builder.WriteString(jsString(value))
	}
	builder.WriteString("]")
	return builder.String()
}
