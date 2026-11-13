//go:build !linux && !darwin

package gladecli

// hostMemoryBudget is unknown on this platform; callers use the conservative
// fallback floor.
func hostMemoryBudget() uint64 { return 0 }
