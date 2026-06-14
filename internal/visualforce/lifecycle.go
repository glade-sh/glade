package visualforce

import "github.com/glade-sh/glade/internal/vm"

type RequestKind string

const (
	RequestGET  RequestKind = "GET"
	RequestPOST RequestKind = "POST"
	RequestAjax RequestKind = "AJAX"
)

type LifecycleRequest struct {
	Page           Page
	PageURL        string
	Kind           RequestKind
	ViewState      *ViewStatePayload
	FormValues     map[string]string
	Action         string
	PartialTargets []string
}

type LifecycleState struct {
	Controller         vm.Value
	Extensions         []vm.Value
	StandardController vm.Value
	PageMessages       []vm.Value
	Redirect           *vm.Value
}
