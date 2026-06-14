package visualforce

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

type RemotingRequest struct {
	Action string            `json:"action"`
	Method string            `json:"method"`
	Data   []json.RawMessage `json:"data"`
	Type   string            `json:"type,omitempty"`
	TID    int               `json:"tid,omitempty"`
	CTX    map[string]any    `json:"ctx,omitempty"`
}

type RemoteActionMethod struct {
	ClassName   string
	MethodName  string
	Annotations []string
	Modifiers   []string
}

type RemotingMetadata struct {
	Actions []RemoteActionDescriptor
}

type RemoteActionDescriptor struct {
	ClassName  string
	MethodName string
	Action     string
}

type RemotingInvocation struct {
	Request   RemotingRequest
	Action    RemoteActionDescriptor
	Arguments []json.RawMessage
}

type RemotingResponse struct {
	Action  string          `json:"action"`
	Method  string          `json:"method"`
	Type    string          `json:"type,omitempty"`
	TID     int             `json:"tid,omitempty"`
	Status  bool            `json:"status"`
	Result  any             `json:"result,omitempty"`
	Message string          `json:"message,omitempty"`
	Where   string          `json:"where,omitempty"`
	Errors  []RemotingError `json:"errors,omitempty"`
}

type RemotingError struct {
	Message string `json:"message"`
	Where   string `json:"where,omitempty"`
}

type RemotingInvoker func(RemotingInvocation) (any, error)

func ValidateRemoteActionExposure(method RemoteActionMethod) error {
	className := strings.TrimSpace(method.ClassName)
	methodName := strings.TrimSpace(method.MethodName)
	if className == "" || methodName == "" {
		return fmt.Errorf("remote action method requires class and method names")
	}
	if !hasRemoteActionAnnotation(method.Annotations) {
		return fmt.Errorf("%s.%s is not annotated @RemoteAction", className, methodName)
	}
	if !hasFold(method.Modifiers, "static") {
		return fmt.Errorf("@RemoteAction method %s.%s must be static", className, methodName)
	}
	if !hasFold(method.Modifiers, "public") && !hasFold(method.Modifiers, "global") {
		return fmt.Errorf("@RemoteAction method %s.%s must be public or global", className, methodName)
	}
	return nil
}

func BuildRemotingMetadataFromIndex(page Page, index typesys.Index) (RemotingMetadata, error) {
	methods := make([]RemoteActionMethod, 0)
	for _, typ := range index.Types {
		className := strings.TrimSpace(typ.Name)
		if className == "" {
			continue
		}
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationMethod || !hasRemoteActionAnnotation(member.Modifiers) {
				continue
			}
			methods = append(methods, RemoteActionMethod{
				ClassName:   className,
				MethodName:  member.Name,
				Annotations: member.Modifiers,
				Modifiers:   member.Modifiers,
			})
		}
	}
	return BuildRemotingMetadata(page, methods)
}

func BuildRemotingMetadata(page Page, methods []RemoteActionMethod) (RemotingMetadata, error) {
	classes := map[string]bool{}
	if controller := strings.TrimSpace(page.Controller); controller != "" {
		classes[strings.ToLower(controller)] = true
	}
	for _, extension := range page.Extensions {
		if extension = strings.TrimSpace(extension); extension != "" {
			classes[strings.ToLower(extension)] = true
		}
	}
	metadata := RemotingMetadata{}
	for _, method := range methods {
		if !classes[strings.ToLower(strings.TrimSpace(method.ClassName))] || !hasRemoteActionAnnotation(method.Annotations) {
			continue
		}
		if err := ValidateRemoteActionExposure(method); err != nil {
			return RemotingMetadata{}, err
		}
		className := strings.TrimSpace(method.ClassName)
		methodName := strings.TrimSpace(method.MethodName)
		metadata.Actions = append(metadata.Actions, RemoteActionDescriptor{
			ClassName:  className,
			MethodName: methodName,
			Action:     className + "." + methodName,
		})
	}
	sort.Slice(metadata.Actions, func(i, j int) bool {
		if metadata.Actions[i].ClassName == metadata.Actions[j].ClassName {
			return metadata.Actions[i].MethodName < metadata.Actions[j].MethodName
		}
		return metadata.Actions[i].ClassName < metadata.Actions[j].ClassName
	})
	return metadata, nil
}

func DispatchRemotingRequests(metadata RemotingMetadata, requests []RemotingRequest, invoker RemotingInvoker) []RemotingResponse {
	actions := remotingActionLookup(metadata)
	responses := make([]RemotingResponse, 0, len(requests))
	for _, request := range requests {
		action, ok := actions[remotingRequestActionKey(request)]
		if !ok {
			responses = append(responses, remotingFailureResponse(request, "Visualforce remoting action not found", ""))
			continue
		}
		response := RemotingResponse{
			Action: action.ClassName,
			Method: action.MethodName,
			Type:   request.Type,
			TID:    request.TID,
		}
		if invoker == nil {
			response.Status = false
			response.Message = "Visualforce remoting dispatch is not bound"
			response.Errors = []RemotingError{{Message: response.Message}}
			responses = append(responses, response)
			continue
		}
		result, err := invoker(RemotingInvocation{Request: request, Action: action, Arguments: append([]json.RawMessage(nil), request.Data...)})
		if err != nil {
			response.Status = false
			response.Message = err.Error()
			response.Errors = []RemotingError{{Message: err.Error()}}
			responses = append(responses, response)
			continue
		}
		response.Status = true
		response.Result = result
		responses = append(responses, response)
	}
	return responses
}

func RenderRemotingMetadataScript(metadata RemotingMetadata) string {
	builder := strings.Builder{}
	builder.WriteString(`<script>(function(window){`)
	builder.WriteString(`window.Visualforce=window.Visualforce||{};`)
	builder.WriteString(`Visualforce.remoting=Visualforce.remoting||{};`)
	builder.WriteString(`Visualforce.remoting.Manager=Visualforce.remoting.Manager||{};`)
	builder.WriteString(`Visualforce.remoting.Manager._tid=Visualforce.remoting.Manager._tid||1;`)
	builder.WriteString(`Visualforce.remoting.Manager.invokeAction=function(remoteAction){var values=Array.prototype.slice.call(arguments,1);var callback=null;if(values.length&&typeof values[values.length-1]=="function"){callback=values.pop();}else if(values.length>1&&typeof values[values.length-2]=="function"){callback=values[values.length-2];values.splice(values.length-2,1);}var isOptions=function(value){return value&&typeof value=="object"&&!Array.isArray(value)&&("escape" in value||"timeout" in value||"buffer" in value||"abortable" in value);};if(callback&&values.length&&isOptions(values[values.length-1])){values.pop();}var read=function(name){var el=document.querySelector('input[name="'+name+'"]');return el?el.value:"";};var actionText=String(remoteAction||"");var actionName=actionText.replace(/^\{!\$RemoteAction\./,"").replace(/\}$/,"");var dot=actionName.lastIndexOf(".");var action=dot>=0?actionName.slice(0,dot):actionName;var method=dot>=0?actionName.slice(dot+1):"";var request={action:action,method:method,data:values,type:"rpc",tid:Visualforce.remoting.Manager._tid++,ctx:{page:window.location.pathname,viewState:read("` + ViewStateFormFieldName() + `"),csrf:read("__vf_csrf")}};return fetch(window.location.pathname.replace(/\/$/,"")+"/remoting",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify([request])}).then(function(response){return response.json();}).then(function(responses){var response=Array.isArray(responses)?responses[0]:responses;var event={status:!!(response&&response.status),type:(response&&response.type)||"rpc",tid:response&&response.tid,action:response&&response.action,method:response&&response.method,message:response&&response.message,where:response&&response.where};if(callback){callback(response?response.result:null,event);}return response;}).catch(function(err){var event={status:false,type:"exception",message:String(err)};if(callback){callback(null,event);}return {status:false,message:String(err),errors:[{message:String(err)}]};});};`)
	for _, action := range metadata.Actions {
		classID := jsIdentifier(action.ClassName)
		methodID := jsIdentifier(action.MethodName)
		remoteActionJSON := jsString("{!$RemoteAction." + action.Action + "}")
		builder.WriteString(`window.`)
		builder.WriteString(classID)
		builder.WriteString(`=window.`)
		builder.WriteString(classID)
		builder.WriteString(`||{};`)
		builder.WriteString(classID)
		builder.WriteString(`.`)
		builder.WriteString(methodID)
		builder.WriteString(`=function(){var args=Array.prototype.slice.call(arguments);args.unshift(`)
		builder.WriteString(remoteActionJSON)
		builder.WriteString(`);return Visualforce.remoting.Manager.invokeAction.apply(Visualforce.remoting.Manager,args);};`)
	}
	builder.WriteString(`})(window);</script>`)
	return builder.String()
}

func remotingActionLookup(metadata RemotingMetadata) map[string]RemoteActionDescriptor {
	out := make(map[string]RemoteActionDescriptor, len(metadata.Actions)*2)
	for _, action := range metadata.Actions {
		key := strings.ToLower(strings.TrimSpace(action.Action))
		if key == "" {
			continue
		}
		out[key] = action
		out[strings.ToLower("{!$RemoteAction."+action.Action+"}")] = action
	}
	return out
}

func remotingRequestActionKey(request RemotingRequest) string {
	action := strings.TrimSpace(request.Action)
	method := strings.TrimSpace(request.Method)
	if action != "" && method != "" && !strings.Contains(action, ".") && !strings.Contains(action, "$RemoteAction") {
		return strings.ToLower(action + "." + method)
	}
	return strings.ToLower(action)
}

func remotingFailureResponse(request RemotingRequest, message, where string) RemotingResponse {
	return RemotingResponse{
		Action:  strings.TrimSpace(request.Action),
		Method:  strings.TrimSpace(request.Method),
		Type:    request.Type,
		TID:     request.TID,
		Status:  false,
		Message: message,
		Where:   where,
		Errors:  []RemotingError{{Message: message, Where: where}},
	}
}

func ValidateRemotingRequest(body []byte) error {
	return CheckVisualforceRemotingRequestSize(len(body))
}

func NormalizeRemotingTimeout(timeout time.Duration) (time.Duration, error) {
	if timeout <= 0 {
		return DefaultVisualforceRemotingTimeout, nil
	}
	if timeout > MaxVisualforceRemotingTimeout {
		return 0, fmt.Errorf("visualforce remoting timeout %s exceeds max %ds", timeout, int(MaxVisualforceRemotingTimeout/time.Second))
	}
	return timeout, nil
}

func hasRemoteActionAnnotation(annotations []string) bool {
	for _, annotation := range annotations {
		annotation = strings.TrimPrefix(strings.TrimSpace(annotation), "@")
		if strings.EqualFold(annotation, "RemoteAction") {
			return true
		}
	}
	return false
}

func hasFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), want) {
			return true
		}
	}
	return false
}

func jsString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
