package visualforce

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestPageNamesPreflight(t *testing.T) {
	for _, api := range []string{"59.0", "67.0"} {
		for _, tc := range []struct {
			name         string
			page         string
			component    string
			metadata     string
			fallback     bool
			fallbackFail bool
		}{
			{name: "missing-page"},
			{name: "invalid-page", page: `<div/>`},
			{name: "invalid-component", page: `<apex:page/>`, component: `<div/>`},
			{name: "invalid-metadata", page: `<apex:page/>`, metadata: `<ApexPage><apiVersion>not-a-version</apiVersion></ApexPage>`},
			{name: "valid-structure", page: `<apex:page/>`, fallback: true},
			{name: "expression-needs-strict-loader", page: `<apex:page controller="Controller">{!missing}</apex:page>`, fallback: true, fallbackFail: true},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				page := filepath.Join(root, "Example.page")
				p := project.Project{SourceAPIVersion: api, VisualforcePageFiles: []string{page}, ApexFiles: []string{filepath.Join(root, "Unavailable.cls")}}
				if tc.page != "" {
					writeFile(t, page, tc.page)
				}
				if tc.metadata != "" {
					writeFile(t, page+"-meta.xml", tc.metadata)
				}
				if tc.component != "" {
					component := filepath.Join(root, "Example.component")
					writeFile(t, component, tc.component)
					p.VisualforceComponentFiles = []string{component}
				}
				called := false
				got := pageNames(p, func(project.Project) (Index, error) {
					called = true
					if !tc.fallback {
						t.Fatal("structural rejection invoked the strict loader")
					}
					if tc.fallbackFail {
						return Index{}, errors.New("expression rejected")
					}
					return Index{Pages: []Page{{Name: "Example"}}}, nil
				})
				if called != tc.fallback {
					t.Fatalf("strict loader called = %v, want %v", called, tc.fallback)
				}
				var want []string
				if tc.fallback && !tc.fallbackFail {
					want = []string{"Example"}
				}
				if !slices.Equal(got, want) {
					t.Fatalf("page names = %v, want %v", got, want)
				}
			})
		}
	}
}
