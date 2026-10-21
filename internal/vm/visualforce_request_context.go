package vm

import (
	"sort"
	"strings"
)

// PageReference headers have their own runtime marker so their native lookup
// and null rules do not alter ordinary Map<String,String> values.
func pageReferenceHeaders() Value {
	headers := typedMap("Map<String,String>")
	headers.Runtime = "pagereference-headers"
	return headers
}

// Both put and putAll validate each header value before applying that entry.
// Native bulk writes retain earlier entries when a later null value throws.
func visualforceHeaderValueError(receiver, key, value Value) error {
	if receiver.Runtime == "pagereference-headers" && value.Kind == ValueNull {
		return newExceptionError("System.NullPointerException", "Value for header '"+key.String()+"' cannot be null.")
	}
	return nil
}

// SetCurrentPageRequestHeaders seeds the current request before its controller
// is constructed. It does not change the execution user or trust user headers.
func (vm *VM) SetCurrentPageRequestHeaders(input map[string]string) {
	if vm == nil || vm.currentPage.Kind != ValueObject {
		return
	}
	headers := pageReferenceHeaders()
	names := make([]string, 0, len(input))
	for name := range input {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		key := String(name)
		raw := mapKey(key)
		headers.Map[raw] = String(input[name])
		headers.MapKeys[raw] = key
		headers.MapOrder = append(headers.MapOrder, raw)
	}
	vm.currentPage.Fields["headers"] = headers
	vm.currentPageHeaderWrites = nil
	vm.currentPageHeaderWriteRef = headers.Ref
}

// A successful entry write is a response write even when it repeats an incoming value.
// Track only this request's header map; ordinary maps and detached clones do
// not acquire response authority from containing the same header text.
func (vm *VM) recordVisualforceHeaderWrite(receiver, key Value) {
	if vm == nil || receiver.Runtime != "pagereference-headers" || key.Kind != ValueString ||
		vm.currentPage.Kind != ValueObject || receiver.Ref == 0 ||
		receiver.Ref != vm.currentPage.Fields["headers"].Ref {
		return
	}
	if vm.currentPageHeaderWriteRef != receiver.Ref {
		vm.currentPageHeaderWrites = nil
		vm.currentPageHeaderWriteRef = receiver.Ref
	}
	if vm.currentPageHeaderWrites == nil {
		vm.currentPageHeaderWrites = make(map[string]bool)
	}
	vm.currentPageHeaderWrites[strings.ToLower(key.Text)] = true
}

func (vm *VM) visualforceHeaderWrites() map[string]bool {
	if vm == nil || vm.currentPage.Kind != ValueObject || vm.currentPageHeaderWriteRef == 0 ||
		vm.currentPageHeaderWriteRef != vm.currentPage.Fields["headers"].Ref {
		return nil
	}
	return vm.currentPageHeaderWrites
}

// CurrentPageHeaderWritten distinguishes an explicit response write from an
// unchanged incoming header. Header names follow the page map's case rules.
func (vm *VM) CurrentPageHeaderWritten(name string) bool {
	return vm.visualforceHeaderWrites()[strings.ToLower(name)]
}

func (vm *VM) CurrentPageHeaders() map[string]string {
	result := map[string]string{}
	if vm == nil || vm.currentPage.Kind != ValueObject {
		return result
	}
	headers := vm.currentPage.Fields["headers"]
	for _, raw := range orderedValueMapKeys(headers) {
		key, value := mapStoredKey(headers, raw), headers.Map[raw]
		if key.Kind == ValueString && value.Kind == ValueString {
			result[key.Text] = value.Text
		}
	}
	return result
}

// VisualforceResponseCookie carries the controller's actual Cookie fields to
// the HTTP boundary. Only cookies written by setCookies enter this collection.
type VisualforceResponseCookie struct {
	Name     string
	Value    string
	Path     string
	MaxAge   int
	Secure   bool
	HTTPOnly bool
	SameSite string
}

func (vm *VM) CurrentPageResponseCookies() []VisualforceResponseCookie {
	var result []VisualforceResponseCookie
	if vm == nil || vm.currentPage.Kind != ValueObject {
		return result
	}
	cookies := vm.currentPage.Fields["responseCookies"]
	for _, raw := range orderedValueMapKeys(cookies) {
		cookie := cookies.Map[raw]
		if cookie.Kind != ValueObject || !strings.EqualFold(cookie.Type, "Cookie") {
			continue
		}
		value := cookie.Fields["value"]
		text := ""
		if value.Kind == ValueString {
			text = value.Text
		}
		result = append(result, VisualforceResponseCookie{
			Name:     cookie.Fields["name"].Text,
			Value:    text,
			Path:     cookie.Fields["path"].Text,
			MaxAge:   int(cookie.Fields["maxAge"].Int),
			Secure:   cookie.Fields["secure"].Bool,
			HTTPOnly: cookie.Fields["httpOnly"].Bool,
			SameSite: cookie.Fields["sameSite"].Text,
		})
	}
	return result
}
