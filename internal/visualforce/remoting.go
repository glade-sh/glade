package visualforce

import (
	_ "embed"
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
	ClassName      string
	MethodName     string
	Annotations    []string
	Modifiers      []string
	ParameterCount int
	ParameterTypes []string
}

type RemotingMetadata struct {
	Actions []RemoteActionDescriptor
}

type RemoteActionDescriptor struct {
	ClassName      string
	MethodName     string
	Action         string
	ParameterCount int
	ParameterTypes []string `json:"-"`
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
			parameterTypes := make([]string, len(member.Parameters))
			for i, parameter := range member.Parameters {
				parameterTypes[i] = parameter.Type
			}
			methods = append(methods, RemoteActionMethod{
				ClassName:      className,
				MethodName:     member.Name,
				Annotations:    member.Modifiers,
				Modifiers:      member.Modifiers,
				ParameterCount: len(member.Parameters),
				ParameterTypes: parameterTypes,
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
			ClassName:      className,
			MethodName:     methodName,
			Action:         className + "." + methodName,
			ParameterCount: method.ParameterCount,
			ParameterTypes: append([]string(nil), method.ParameterTypes...),
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

// Rendering and dispatch discover the same exposed actions. Reuse the page's
// registered runtime so rendering does not parse or compile the project again.
func renderPageRemotingScript(node *MarkupNode, ctx *RenderContext) string {
	if ctx.VM == nil {
		return ""
	}
	page := ctx.PageMeta
	if page.Controller == "" {
		page.Controller = node.Attribute("controller")
	}
	if len(page.Extensions) == 0 {
		page.Extensions = splitCSV(node.Attribute("extensions"))
	}
	methods := make([]RemoteActionMethod, 0)
	seen := map[string]bool{}
	for _, method := range ctx.VM.Methods {
		dot := strings.LastIndex(method.Name, ".")
		if dot < 0 || !hasRemoteActionAnnotation(method.Modifiers) {
			continue
		}
		candidate := RemoteActionMethod{
			ClassName:      method.Name[:dot],
			MethodName:     method.Name[dot+1:],
			Annotations:    method.Modifiers,
			Modifiers:      method.Modifiers,
			ParameterCount: len(method.Params),
		}
		key := strings.ToLower(method.Name)
		if seen[key] || ValidateRemoteActionExposure(candidate) != nil {
			continue
		}
		seen[key] = true
		methods = append(methods, candidate)
	}
	metadata, err := BuildRemotingMetadata(page, methods)
	if err != nil || len(metadata.Actions) == 0 {
		return ""
	}
	return RenderRemotingMetadataScript(metadata)
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

//go:embed remoting_runtime.js
var remotingManagerScript string

func RenderRemotingMetadataScript(metadata RemotingMetadata) string {
	builder := strings.Builder{}
	builder.WriteString(`<script>(function(window){`)
	actions, _ := json.Marshal(metadata.Actions)
	builder.WriteString(`var actions=`)
	builder.Write(actions)
	builder.WriteString(`;`)
	builder.WriteString(strings.ReplaceAll(remotingManagerScript, "__GLADE_VIEW_STATE_FIELD__", ViewStateFormFieldName()))
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
