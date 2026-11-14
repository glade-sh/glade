package apextest

import (
	"path/filepath"
	"testing"
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
