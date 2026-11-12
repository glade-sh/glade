//go:build !darwin && !linux

package filestamp

import "os"

// Without a trustworthy ctime no stamp is ever produced, so callers hash.
func fromInfo(os.FileInfo) (Stamp, bool) { return Stamp{}, false }
