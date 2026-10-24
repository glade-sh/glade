package vm_test

import "testing"

func TestSystemTypeConstructionNativeAnonymousNamedAndProjectRoutes(t *testing.T) {
	runExecutionCapture(t, "system_type_construction", 2)
}

func TestConcreteFinalizerFieldNativeAnonymousNamedAndProjectRoutes(t *testing.T) {
	runExecutionCapture(t, "finalizer_field_dispatch", 3)
}

func TestPublishingDateArgumentNativeAnonymousNamedAndProjectRoutes(t *testing.T) {
	runExecutionCapture(t, "publishing_date_arguments", 3)
}
