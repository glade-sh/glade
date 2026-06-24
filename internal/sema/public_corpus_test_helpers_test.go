package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func analyzePublicCorpusFiles(t *testing.T, root string, files ...string) Result {
	t.Helper()
	apexFiles := make([]string, 0, len(files))
	for _, file := range files {
		apexFiles = append(apexFiles, filepath.Join(root, file))
	}
	return Analyze(typesys.Build(project.Project{Root: root, ApexFiles: apexFiles}, schema.Schema{}))
}

func analyzePublicCorpusWithSchema(t *testing.T, root string, schema schema.Schema, files ...string) Result {
	t.Helper()
	apexFiles := make([]string, 0, len(files))
	for _, file := range files {
		apexFiles = append(apexFiles, filepath.Join(root, file))
	}
	return Analyze(typesys.Build(project.Project{Root: root, ApexFiles: apexFiles}, schema))
}
