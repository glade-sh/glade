package visualforce

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/project"
)

// rejectionRecord is the left-out files of the latest PageNamesWithDiagnostics
// call for one project root. It is reused only while the files that call read
// (pages, components, profiles, permission sets and their groups) keep the same
// size and modification time, so a long-lived process never reports stale files.
type rejectionRecord struct {
	signature string
	rejected  []PageRejection
}

var lastPageRejections sync.Map // project root -> rejectionRecord

func pageInputsSignature(p project.Project) string {
	var b strings.Builder
	for _, group := range [][]string{p.VisualforcePageFiles, p.VisualforceComponentFiles, p.ProfileFiles, p.PermissionSetFiles, p.PermissionSetGroupFiles} {
		for _, path := range group {
			info, err := os.Stat(path)
			if err != nil {
				fmt.Fprintf(&b, "%s|missing;", path)
				continue
			}
			fmt.Fprintf(&b, "%s|%d|%d;", path, info.Size(), info.ModTime().UnixNano())
		}
		b.WriteString("#")
	}
	return b.String()
}

func recordPageRejections(p project.Project, rejected []PageRejection) {
	if p.Root == "" {
		return
	}
	lastPageRejections.Store(p.Root, rejectionRecord{signature: pageInputsSignature(p), rejected: append([]PageRejection(nil), rejected...)})
}

// PageRejections returns the pages, components and metadata files that page
// registration leaves out for p. A runtime that already registered the pages
// has recorded the answer for unchanged inputs; otherwise the pages are loaded
// once here. Callers report the result and never use it to decide a test
// outcome.
func PageRejections(p project.Project) []PageRejection {
	if len(p.VisualforcePageFiles) == 0 {
		return nil
	}
	if cached, ok := lastPageRejections.Load(p.Root); ok {
		if record := cached.(rejectionRecord); record.signature == pageInputsSignature(p) {
			return append([]PageRejection(nil), record.rejected...)
		}
	}
	_, rejected := PageNamesWithDiagnostics(p, nil)
	return rejected
}

// PageRejectionMessages formats the rejections of p and of its loaded
// dependencies as one line per file: "visualforce: <file>: <message>". Files
// are relative to root when they sit under it.
func PageRejectionMessages(root string, p project.Project) []string {
	var messages []string
	seen := map[string]bool{}
	var visit func(project.Project)
	visit = func(p project.Project) {
		for _, rejection := range PageRejections(p) {
			file := rejection.File
			if root != "" && file != "" {
				if rel, err := filepath.Rel(root, file); err == nil && !strings.HasPrefix(rel, "..") {
					file = rel
				}
			}
			message := strings.Join(strings.Fields(rejection.Message), " ")
			line := "visualforce: " + file + ": " + message
			if file == "" {
				line = "visualforce: " + message
			}
			if !seen[line] {
				seen[line] = true
				messages = append(messages, line)
			}
		}
		for _, dep := range p.ManagedPackageDependencies {
			if dep.Status == "loaded" && dep.Project != nil {
				visit(*dep.Project)
			}
		}
	}
	visit(p)
	return messages
}
