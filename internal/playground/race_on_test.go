//go:build race

package playground

import "time"

// case339RaceRunBudget widens the public run deadline for tests that must
// see a completed run. Under the race detector the anonymous compile alone
// can outrun the 5s product limit (14.5s observed in the scheduled race
// run), so the race build raises the budget for those tests only; the
// product default itself is unchanged.
const case339RaceRunBudget = 30 * time.Second
