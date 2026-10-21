package apextest

import "testing"

func TestPlatformConstructorSchemaNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "platform_constructor_schema", "PlatformConstructorSchemaTest", []string{"62.0", "67.0"}, 3)
}
