//go:build windows

package gladecli

// peakRSSBytes is not measured on Windows.
func peakRSSBytes() int64 { return 0 }
