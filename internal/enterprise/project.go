package enterprise

import (
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

type Context struct {
	Project project.Project
	Schema  gladeschema.Schema
	Index   typesys.Index
	Sema    sema.Result
}

func LoadContext(root string) (Context, error) {
	p, err := project.Load(root)
	if err != nil {
		return Context{}, err
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		return Context{}, err
	}
	index := typesys.Build(p, s)
	return Context{Project: p, Schema: s, Index: index, Sema: sema.Analyze(index)}, nil
}

func (c Context) Summary() ProjectSummary {
	tests := 0
	classes := 0
	for _, typ := range c.Index.Types {
		if typ.Dependency {
			continue
		}
		if string(typ.Kind) == "class" {
			classes++
		}
		if typ.IsTest {
			tests++
		}
	}
	return ProjectSummary{
		Root:             c.Project.Root,
		Namespace:        c.Project.Namespace,
		SourceAPIVersion: c.Project.SourceAPIVersion,
		ApexClasses:      classes,
		Triggers:         len(c.Index.Triggers),
		Tests:            tests,
		MetadataFiles:    CountMetadataFiles(c.Project),
	}
}

func CountMetadataFiles(p project.Project) int {
	return len(p.ObjectFiles) + len(p.FieldFiles) + len(p.FieldSetFiles) +
		len(p.RecordTypeFiles) + len(p.ValidationRuleFiles) + len(p.LabelFiles) +
		len(p.TranslationFiles) + len(p.StaticResourceFiles) + len(p.StaticResourceMetas) +
		len(p.DataWeaveFiles) + len(p.DataWeaveMetas) + len(p.ContentAssetFiles) +
		len(p.ContentAssetMetas) + len(p.EmailTemplateFiles) + len(p.FolderFiles) +
		len(p.NamedCredentialFiles) + len(p.RemoteSiteFiles) + len(p.CustomMetadataFiles) +
		len(p.WorkflowFiles) + len(p.FlowFiles) + len(p.ProfileFiles) +
		len(p.PermissionSetFiles) + len(p.PermissionSetGroupFiles) +
		len(p.PermissionAssignmentFiles) + len(p.ListViewFiles) + len(p.LayoutFiles) +
		len(p.CompactLayoutFiles) + len(p.TabFiles) + len(p.WebLinkFiles) +
		len(p.QuickActionFiles) + len(p.GlobalValueSetFiles) + len(p.StandardValueSetFiles) +
		len(p.FlexiPageFiles) + len(p.ApplicationFiles) + len(p.VisualforcePageFiles) +
		len(p.VisualforceComponentFiles) + len(p.AuraFiles) + len(p.LWCFiles) +
		len(p.LWCHTMLFiles) + len(p.LWCCSSFiles) + len(p.LWCMetaFiles)
}
