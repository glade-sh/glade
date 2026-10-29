package gladecli

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPersonAccountProfileRoutesProjectLoaders(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	fixtures := map[bool]string{
		true:  "../../testdata/conformance/person_account_profile_routes",
		false: "../../testdata/conformance/person_account_profile_disabled",
	}
	native := map[bool]map[string]string{}
	for personAccounts, fixture := range fixtures {
		body, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(strings.NewReader(string(body)))
		reader.Comma, reader.FieldsPerRecord = '\t', 5
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 || strings.Join(rows[0], "\t") != "api\troute\tcase\tvalue\tdetail" {
			t.Fatal("unexpected native oracle header")
		}
		native[personAccounts] = map[string]string{}
		for _, row := range rows[1:] {
			key := row[0] + "/" + row[1] + "/" + row[2]
			if _, exists := native[personAccounts][key]; exists {
				t.Fatalf("duplicate native row %v", row)
			}
			native[personAccounts][key] = row[3] + "|" + row[4]
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, personAccounts := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/person_accounts_%t", api, personAccounts), func(t *testing.T) {
				root := copyProfileRoutesLoaderProject(t, fixtures[personAccounts], api, personAccounts)
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
						const marker = "profile-business-flags-restored"
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
				} {
					t.Run(route.name, func(t *testing.T) {
						org, err := route.load()
						if err != nil {
							t.Fatal(err)
						}
						if _, exists := org.Objects["PersonAccount"]; exists {
							t.Fatal("PersonAccount object was not folded into Account")
						}
						account := org.Objects["Account"]
						_, enabled := storage.ResolveFieldName(account.Definition, "", "IsPersonAccount")
						if enabled != personAccounts {
							t.Fatalf("Person Accounts enabled=%v, want %v", enabled, personAccounts)
						}
						if !personAccounts {
							for _, rt := range account.Definition.RecordTypes {
								if rt.PersonAccount {
									t.Fatalf("person record type in disabled topology: %+v", rt)
								}
							}
						}
						for _, profile := range []struct{ key, name string }{{"admin", "System Administrator"}, {"false", "Route False"}, {"omitted", "Route Omitted"}, {"misplaced", "Route Misplaced"}} {
							t.Run(profile.key, func(t *testing.T) {
								profileID := storage.ID(storage.DefaultProfileID)
								if profile.key != "admin" {
									profileID = ""
									for id, record := range org.Objects["Profile"].Records {
										if record.Fields["Name"].String == profile.name {
											profileID = id
											break
										}
									}
									if profileID == "" {
										t.Fatalf("profile %q missing", profile.name)
									}
								}
								users := org.Objects["User"]
								user := users.Records[storage.DefaultUserID]
								user.Fields["ProfileId"] = storage.IDValue(profileID)
								users.Records[storage.DefaultUserID] = user
								org.Objects["User"] = users
								caseName, detail := "business_omitted", ""
								if personAccounts {
									caseName, detail = "scalar_business_omitted", "false"
								}
								key := api + "/istest/" + profile.key + "_" + caseName
								captured, ok := native[personAccounts][key]
								if !ok || !strings.HasSuffix(captured, "|"+detail) {
									t.Fatalf("missing successful native business row %s: %q", key, captured)
								}
								want := strings.TrimSuffix(captured, "|"+detail)
								engine := dml.NewEngine(&org)
								results := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}}})
								if len(results) != 1 || !results[0].Success {
									t.Fatalf("business insert: %+v", results)
								}
								account := org.Objects["Account"]
								id := storage.ID(account.Records[results[0].ID].Fields["RecordTypeId"].ID)
								got := ""
								for _, rt := range account.Definition.RecordTypes {
									if storage.IDsEqual(rt.ID, id) {
										got = rt.DeveloperName
									}
								}
								if got != want {
									t.Fatalf("business default: got %s, want %s", got, want)
								}
							})
						}
					})
				}
			})
		}
	}
}

func copyProfileRoutesLoaderProject(t *testing.T, fixture, api string, personAccounts bool) string {
	t.Helper()
	root := t.TempDir()
	err := filepath.WalkDir(fixture, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, err := filepath.Rel(fixture, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if !strings.HasPrefix(rel, "force-app/") && !strings.HasPrefix(rel, "config/") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		source := string(body)
		if strings.HasSuffix(rel, ".cls-meta.xml") {
			source = strings.ReplaceAll(strings.ReplaceAll(source, "62.0", api), "67.0", api)
		}
		if rel == "config/project-scratch-def.json" {
			var config struct {
				Features []string `json:"features"`
			}
			if err := json.Unmarshal(body, &config); err != nil {
				return err
			}
			enabled := false
			for _, feature := range config.Features {
				enabled = enabled || strings.EqualFold(feature, "PersonAccounts")
			}
			if enabled != personAccounts {
				return fmt.Errorf("captured project PersonAccounts=%v, want %v", enabled, personAccounts)
			}
		}
		writeTestFile(t, filepath.Join(root, rel), source)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
	return root
}
