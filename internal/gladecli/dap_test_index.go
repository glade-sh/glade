package gladecli

import (
	"fmt"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/frontendcache"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func loadDAPTestIndex(root string) (typesys.Index, error) {
	return loadDAPTestIndexWithBuilder(root, typesys.BuildWithArtifacts)
}

func loadDAPTestIndexWithBuilder(root string, builder func(project.Project, gladeschema.Schema) (typesys.Index, typesys.BuildArtifacts)) (typesys.Index, error) {
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, err
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		index, _ := builder(p, gladeschema.Schema{})
		index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{Severity: diagnostic.Error, Code: "GLADESCHEMA001", Message: fmt.Sprintf("metadata schema load failed: %v", err)})
		return index, nil
	}
	build := func() (typesys.Index, typesys.BuildArtifacts) { return builder(p, s) }
	if apextest.ResolveDiskRuntimeCachePolicy(apextest.Options{}).Enabled {
		return frontendcache.LoadOrBuild(p, s, build).Index, nil
	}
	index, _ := build()
	return index, nil
}
