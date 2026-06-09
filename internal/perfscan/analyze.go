package perfscan

import (
	"os"
	"path/filepath"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

type Options struct {
	ProjectRoot string
	TracePath   string
	TopN        int
}

func AnalyzeProject(options Options) (Report, error) {
	root := options.ProjectRoot
	if root == "" {
		root = "."
	}
	absRoot, _ := filepath.Abs(root)
	report := Report{SchemaVersion: SchemaVersion, Project: absRoot}

	p, err := project.Load(absRoot)
	if err != nil {
		return report, err
	}

	schema, err := gladeschema.LoadProject(p)
	if err != nil {
		return report, err
	}

	parser := apexast.NewParser()
	parsed := apexast.Result{Files: make([]apexast.File, 0, len(p.ApexFiles))}
	for _, path := range p.ApexFiles {
		file, err := parser.ParseFile(path)
		if err != nil {
			continue
		}
		parsed.Files = append(parsed.Files, file)
	}

	index := typesys.Build(p, schema)
	scanApex(&report, p, parsed, index)
	scanMetadata(&report, p, index)
	if options.TracePath != "" {
		data, err := os.ReadFile(options.TracePath)
		if err != nil {
			return report, err
		}
		if err := scanTraceBytes(&report, data); err != nil {
			return report, err
		}
	}
	report.Finalize()
	return report, nil
}
