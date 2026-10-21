package vm

// The native capture/R03/R11/R17-R19/R44-R49: a foreign package's short-name
// alias is not an override declared by the local receiver class. Keep
// namespaced overlays and selected dependency declarations on the existing path.
// The original registered dependency body can still carry a bare ClassName;
// its dependency provenance, rather than that alias spelling, decides identity.
func (vm *VM) receiverDependencyOverrideAlias(class Class, candidate, surface Method) bool {
	return !vm.currentMethod.Dependency && !class.Dependency && class.Namespace == "" &&
		!surface.Dependency && candidate.Dependency
}
