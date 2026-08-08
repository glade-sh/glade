package vm

// The local harness exposes one deterministic registered plugin. It keeps the
// existing callback fixture useful while making absent-plugin probes match the
// platform's NoDataFoundException behavior.
var localRegisteredSparkPlugNames = map[string]struct{}{
	"LocalPlugin": {},
}

func localSparkPlugIsRegistered(name string) bool {
	_, ok := localRegisteredSparkPlugNames[name]
	return ok
}
