package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRuntimeKeyTracksVisualforceInputs(t *testing.T) {
	for _, relative := range []string{"pages/Home.page", "pages/Home.page-meta.xml", "components/Part.component", "components/Part.component-meta.xml"} {
		t.Run(relative, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
			page := filepath.Join(root, "src", "pages", "Home.page")
			component := filepath.Join(root, "src", "components", "Part.component")
			writeFile(t, page, `<apex:page/>`)
			writeFile(t, page+"-meta.xml", `<ApexPage><apiVersion>62.0</apiVersion></ApexPage>`)
			writeFile(t, component, `<apex:component/>`)
			writeFile(t, component+"-meta.xml", `<ApexComponent><apiVersion>62.0</apiVersion></ApexComponent>`)
			index := loadTestIndex(t, root)
			before := RuntimeContentKey(index, nil)
			path := filepath.Join(root, "src", relative)
			writeFile(t, path, "changed")
			after := RuntimeContentKey(index, nil)
			if before == after {
				t.Fatalf("runtime key ignored %s", relative)
			}
		})
	}
}

func TestVisualforceRuntimeKeyCoversValidationInputs(t *testing.T) {
	cases := []struct {
		name string
		set  func(*project.Project, string)
	}{
		{"email", func(p *project.Project, f string) { p.EmailTemplateFiles = []string{f} }},
		{"flow", func(p *project.Project, f string) { p.FlowFiles = []string{f} }},
		{"labels", func(p *project.Project, f string) { p.LabelFiles = []string{f} }},
		{"translations", func(p *project.Project, f string) { p.TranslationFiles = []string{f} }},
		{"static resource", func(p *project.Project, f string) { p.StaticResourceFiles = []string{f} }},
		{"static resource metadata", func(p *project.Project, f string) { p.StaticResourceMetas = []string{f} }},
		{"message channel", func(p *project.Project, f string) { p.MessageChannelFiles = []string{f} }},
		{"tab", func(p *project.Project, f string) { p.TabFiles = []string{f} }},
		{"profile", func(p *project.Project, f string) { p.ProfileFiles = []string{f} }},
		{"permission set", func(p *project.Project, f string) { p.PermissionSetFiles = []string{f} }},
		{"permission set group", func(p *project.Project, f string) { p.PermissionSetGroupFiles = []string{f} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
			page := filepath.Join(root, "src", "pages", "Home.page")
			input := filepath.Join(root, "src", "validation-input.xml")
			writeFile(t, page, `<apex:page/>`)
			writeFile(t, input, `before`)
			p := project.Project{Root: root, VisualforcePageFiles: []string{page}}
			tc.set(&p, input)
			index := loadTestIndex(t, root)
			key := func() runtimeCacheKey {
				sources := newSourceCache()
				sources.seedProjectSnapshot(&p)
				gen := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
				if err := sources.captureProjectRuntimeMetadata(index, &gen); err != nil {
					t.Fatal(err)
				}
				return runtimeKeyWithMetadataGeneration("base", gen)
			}
			before := key()
			writeFile(t, input, `after`)
			if after := key(); before == after {
				t.Fatalf("runtime key ignored %s", tc.name)
			}
		})
	}
}

func TestRuntimeKeyTracksVisualforceMembership(t *testing.T) {
	for _, relative := range []string{"pages/Second.page", "components/Second.component"} {
		t.Run(relative, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}],"sourceApiVersion":"62.0"}`)
			writeFile(t, filepath.Join(root, "src", "pages", "Home.page"), `<apex:page/>`)
			before := RuntimeContentKey(loadTestIndex(t, root), nil)
			writeFile(t, filepath.Join(root, "src", relative), `<apex:page/>`)
			after := RuntimeContentKey(loadTestIndex(t, root), nil)
			if before == after {
				t.Fatalf("runtime key ignored addition of %s", relative)
			}
		})
	}
}
