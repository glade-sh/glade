// Package stablemath holds fused-multiply-add-free copies of the Go standard
// library's pure-Go math routines.
//
// Go fuses x*y+z into one instruction on arm64 and not on amd64, and the
// standard library has assembly for Exp and Log on some architectures. The
// same call can then return different last bits on different machines. Apex
// runs on Java, which never fuses unless code calls Math.fma, so the unfused
// result is also the faithful one.
//
// The files other than doc.go are the standard library sources (BSD license,
// see the headers) with the architecture dispatch removed and every float64
// product wrapped in an explicit float64 conversion, which the Go spec says
// forbids fusing. Exported names carry a Stable suffix. Regenerate by
// repeating that transformation on a newer Go release, then run the
// cross-architecture tests in internal/vm.
package stablemath

const (
	mask  = 0x7FF
	shift = 64 - 11 - 1
	bias  = 1023
)
