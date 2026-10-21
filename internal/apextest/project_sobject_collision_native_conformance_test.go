package apextest

import "testing"

func TestProjectSObjectCollisionNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "project_sobject_collision", "ProjectSObjectCollisionProbeTest", []string{"62.0", "67.0"}, 9)
}
