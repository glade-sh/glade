package gladecli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPersonAccountProfileDefaultsProjectLoaders(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			testPersonAccountProfileDefaultsProjectLoaders(t, api)
		})
	}
}

func testPersonAccountProfileDefaultsProjectLoaders(t *testing.T, api string) {
	const fixture = "../../testdata/conformance/person_account_profile_defaults"
	root := t.TempDir()
	err := filepath.WalkDir(fixture, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		if rel != "sfdx-project.json" && !strings.HasPrefix(rel, "force-app/") && !strings.HasPrefix(rel, "config/") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		writeTestFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "67.0", api))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	loadDAP := func() (storage.OrgState, error) {
		org, _, _, err := loadDAPStartupState(root)
		return org, err
	}
	for _, route := range []struct {
		name string
		load func() (storage.OrgState, error)
	}{
		{"exec", func() (storage.OrgState, error) {
			load := loadExecProject(root, true)
			if load.err != nil {
				return storage.OrgState{}, load.err
			}
			return load.org, load.orgErr
		}},
		{"server_and_database", func() (storage.OrgState, error) { return orgForProject(root) }},
		{"dap", loadDAP},
		{"dap_restore", func() (storage.OrgState, error) {
			entry, err := startupcache.Read(root, startupcache.SubdirDAP)
			if err != nil {
				return storage.OrgState{}, err
			}
			if entry == nil {
				return storage.OrgState{}, errors.New("DAP cache was not populated")
			}
			const marker = "person-account-profile-cache-restore"
			entry.BuiltAt = marker
			if err := startupcache.Write(entry, startupcache.SubdirDAP); err != nil {
				return storage.OrgState{}, err
			}
			org, err := loadDAP()
			if err != nil {
				return storage.OrgState{}, err
			}
			restored, err := startupcache.Read(root, startupcache.SubdirDAP)
			if err != nil {
				return storage.OrgState{}, err
			}
			if restored == nil || restored.BuiltAt != marker {
				return storage.OrgState{}, errors.New("DAP cache was rebuilt instead of restored")
			}
			return org, nil
		}},
		{"dap_legacy_cache", func() (storage.OrgState, error) {
			entry, err := startupcache.Read(root, startupcache.SubdirDAP)
			if err != nil {
				return storage.OrgState{}, err
			}
			if entry == nil {
				return storage.OrgState{}, errors.New("DAP cache was not populated")
			}
			entry.Version = dapCacheVersion - 1
			entry.Org.Metadata.AccountRecordTypeDefaults = nil
			if err := startupcache.Write(entry, startupcache.SubdirDAP); err != nil {
				return storage.OrgState{}, err
			}
			return loadDAP()
		}},
	} {
		t.Run(route.name, func(t *testing.T) {
			org, err := route.load()
			if err != nil {
				t.Fatal(err)
			}
			if _, exists := org.Objects["PersonAccount"]; exists {
				t.Fatal("PersonAccount metadata was not folded into Account")
			}
			engine := dml.NewEngine(&org)
			results := engine.Insert([]storage.Record{
				{Object: "Account", Fields: map[string]storage.Value{"LastName": storage.StringValue("Probe")}},
				{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}},
			})
			// Verified anonymous rows admin_person_omitted and
			// admin_business_omitted apply to every project org constructor.
			for i, want := range []string{"Provider", "Business_Default"} {
				if !results[i].Success {
					t.Fatalf("insert %d: %+v", i, results[i])
				}
				account := org.Objects["Account"]
				id := storage.ID(account.Records[results[i].ID].Fields["RecordTypeId"].ID)
				got := ""
				for _, rt := range account.Definition.RecordTypes {
					if storage.IDsEqual(rt.ID, id) {
						got = rt.DeveloperName
					}
				}
				if got != want {
					t.Fatalf("insert %d record type: got %s, want %s", i, got, want)
				}
			}
		})
	}
}
