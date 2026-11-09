package resource

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
)

// ProjectApexClassSourceUnit identifies the project that seeded a class row.
func ProjectApexClassSourceUnit(root string) string {
	identity := projectDirectoryIdentity(root)
	if identity == "" {
		identity = "path:" + filepath.Clean(root)
	}
	digest := sha256.Sum256([]byte(identity))
	return fmt.Sprintf("glade:project-apex-class:%x", digest)
}
