package visualforce

// V09ReleaseRenderEnvironmentsForTest releases request-scoped page-content
// environments after the V09 HTTP/DOM observer has finished a row. Server
// rendering registers each VM in this package-level map; the product currently
// has no release hook. Limit test cleanup to the owned fixture root so unrelated
// render environments remain intact. This helper is absent from product builds.
func V09ReleaseRenderEnvironmentsForTest(projectRoot string) {
	vmRenderEnvironmentMu.Lock()
	defer vmRenderEnvironmentMu.Unlock()
	for machine, environment := range vmRenderEnvironments {
		if environment.Project.Root == projectRoot {
			delete(vmRenderEnvironments, machine)
		}
	}
}
