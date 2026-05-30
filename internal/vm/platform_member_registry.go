package vm

import "sync"

type platformObjectMemberHandler func(vm *VM, receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error)

type platformObjectMemberSurface struct {
	name  string
	phase string
	call  platformObjectMemberHandler
}

// platformObjectMemberSurfaces returns the registry of platform object member
// handlers. The slice and its closures are built once on first use and cached
// (the handlers capture no per-call state) so dispatch does not rebuild them on
// every call. The cache lives behind a function rather than a package var
// because the handlers transitively reference this function, which would create
// a package-initialization cycle. To add a Salesforce surface, register a
// handler here and add its dispatch phase; see docs/ADDING_A_PLATFORM_API.md.
var (
	platformObjectMemberSurfacesOnce  sync.Once
	platformObjectMemberSurfacesCache []platformObjectMemberSurface
)

func platformObjectMemberSurfaces() []platformObjectMemberSurface {
	platformObjectMemberSurfacesOnce.Do(func() {
		platformObjectMemberSurfacesCache = []platformObjectMemberSurface{
			{name: "sfsqlquery-harness", phase: "early", call: func(vm *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
				return vm.callSfsqlqueryHarnessMember(receiver, method, args)
			}},
			{name: "context-industries", phase: "early", call: func(_ *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
				return callContextIndustriesContextMember(receiver, method, args)
			}},
			{name: "org-instrumentation", phase: "early", call: callOrgInstrumentationMember},
			{name: "commerce-inventory", phase: "commerce", call: func(vm *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
				return vm.callCommerceInventoryServiceMember(receiver, method, args)
			}},
			{name: "user-provisioning-batchable", phase: "user-provisioning", call: func(_ *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
				return callUserProvisioningBatchableMember(receiver, method, args)
			}},
			{name: "platform-callback-default", phase: "controller", call: func(_ *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
				value, handled, err := callPlatformCallbackDefaultMember(receiver, method, args)
				return value, receiver, false, handled, err
			}},
			{name: "packaged-controller", phase: "controller", call: func(vm *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
				return vm.callPackagedControllerMember(receiver, method, args)
			}},
			{name: "industry-controller", phase: "controller", call: func(vm *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
				return vm.callIndustryControllerMember(receiver, method, args)
			}},
			{name: "wave-query", phase: "controller", call: func(_ *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
				return callWaveQueryMember(receiver, method, args)
			}},
			{name: "compression-zip", phase: "controller", call: func(_ *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
				return callCompressionZipMember(receiver, method, args)
			}},
			{name: "generated-optional-wrapper", phase: "controller", call: func(vm *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
				value, handled, err := vm.callGeneratedOptionalWrapperMember(receiver, method, args)
				return value, receiver, false, handled, err
			}},
			{name: "slack-local-harness", phase: "controller", call: func(vm *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
				return vm.callSlackLocalHarnessMember(receiver, method, args)
			}},
		}
	})
	return platformObjectMemberSurfacesCache
}

func platformObjectMemberSurfaceNames() []string {
	surfaces := platformObjectMemberSurfaces()
	names := make([]string, 0, len(surfaces))
	for _, surface := range surfaces {
		names = append(names, surface.name)
	}
	return names
}

func callOrgInstrumentationMember(_ *VM, receiver Value, method string, args []Value, _ *Result) (Value, Value, bool, bool, error) {
	if value, updated, mutated, handled, err := callOrgInstrumentationOperationMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, handled, err
	}
	if value, updated, mutated, handled, err := callOrgInstrumentationContextMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, handled, err
	}
	return callOrgInstrumentationServiceMember(receiver, method, args)
}

func (vm *VM) callRegisteredPlatformObjectMemberPhase(phase string, receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	for _, surface := range platformObjectMemberSurfaces() {
		if surface.phase != phase {
			continue
		}
		value, updated, mutated, handled, err := surface.call(vm, receiver, method, args, result)
		if handled || err != nil {
			return value, updated, mutated, true, err
		}
	}
	return Null, receiver, false, false, nil
}
