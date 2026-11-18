//go:build !unix

package semanticcache

// Without ownership checks the executable digest is never cached.
func privateDigestDir(string) bool { return false }

func cacheBaseUsable(string) bool         { return true }
func cacheBaseUsableFor(int, string) bool { return true }
