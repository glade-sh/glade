package scripts

import (
 "fmt"
 "strings"
 "testing"
)

// The CI workflow must schedule every browser shard on both paths.
func TestCIBrowserShardCountsAndRoutes(t *testing.T) {
 _, jobs := readCIWorkflow(t)
 matrix, fast := jobs["lwc-vf"], jobs["pr-tests"]
 for _, tc := range []struct { lane, label string; count int }{{"lwc-compile", "LWC compile", 12}, {"visualforce", "Visualforce", 8}} {
  for i := 0; i < tc.count; i++ {
   row := fmt.Sprintf("          - lane: %s-%d\n            name: %s (%d)\n            shard: %d\n", tc.lane, i, tc.label, i, i)
   if strings.Count(matrix, row) != 1 { t.Errorf("%s shard %d missing or duplicated", tc.lane, i) }
  }
 }
 for _, marker := range []string{"lwc-compile:lwc/compile:24:12", "visualforce:visualforce:36:8", "SHARD == 44"} {
  if !strings.Contains(fast, marker) { t.Errorf("fast lane missing %q", marker) }
 }
}
