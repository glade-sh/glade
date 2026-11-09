package apextest

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// Separate processes keep each API-version case cold before the first run.
func TestDynamicPlatformNameProjectColdAndWarm(t *testing.T) {
	if api := os.Getenv("GLADE_DYNAMIC_PROJECT_API"); api != "" {
		project := newSalesforceSurfaceProject(t, fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
		project.writeClass("DynamicShapeTest", `
@IsTest private class DynamicShapeTest {
  @IsTest static void checkShape() {
    Type t = Type.forName('Connect'+'Api'+'.FeedBody');
    String value = String.valueOf(t.newInstance());
    System.assert(value.contains('isRichText=null'), value);
    System.assertEquals(null, Type.forName('Connect'+'Api'+'.DefinitelyMissing'));
  }
}`)
		for i := 0; i < 2; i++ {
			run := project.run()
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
				t.Fatalf("run %d: summary = %#v, problem = %s", i, got, firstRunProblem(run))
			}
		}
		return
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestDynamicPlatformNameProjectColdAndWarm$")
			cmd.Env = append(os.Environ(), "GLADE_DYNAMIC_PROJECT_API="+api)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("API %s: %v\n%s", api, err, output)
			}
		})
	}
}
