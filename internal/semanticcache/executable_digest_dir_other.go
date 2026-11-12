//go:build !unix

package semanticcache

// Without ownership checks the executable digest is never cached.
func privateDigestDir(string) bool { return false }
