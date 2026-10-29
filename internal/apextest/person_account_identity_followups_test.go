package apextest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// The owned oracle is testdata/conformance/person_record_type_identity/org.tsv.
// The captured unqualified labels do not establish developer identity.
// Qualified metadata retains the pre-existing projection until captured.
func TestPersonAccountIdentityCollisionsFollowups(t *testing.T) {
	type identityCase struct {
		name, incoming string
		recordTypes    []storage.RecordTypeInfo
	}
	var cases []identityCase
	for _, name := range []string{"Care", "Member", "Provider"} {
		business := personAccountFlagFollowupRecordType("Business_"+name, name)
		exact := personAccountFlagFollowupRecordType(name, name+" Label")
		cases = append(cases,
			identityCase{name + "/labelWithoutExact", name, []storage.RecordTypeInfo{business}},
			identityCase{name + "/labelWithExact", name, []storage.RecordTypeInfo{business, exact}},
		)
	}
	for _, route := range []string{"fold", "profile", "permissionSet", "profilePermissions"} {
		for _, tc := range cases {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/reverse=%v", route, tc.name, reverse), func(t *testing.T) {
					originals := append([]storage.RecordTypeInfo(nil), tc.recordTypes...)
					for i := range originals {
						originals[i].ID = storage.ID(fmt.Sprintf("012000000000%03d", 101+i))
					}
					if reverse {
						for i, j := 0, len(originals)-1; i < j; i, j = i+1, j-1 {
							originals[i], originals[j] = originals[j], originals[i]
						}
					}
					org := personAccountFlagFollowupOrg(originals[0])
					account := org.Objects["Account"]
					account.Definition.RecordTypes = append(account.Definition.RecordTypes, originals[1:]...)
					org.Objects["Account"] = account
					count := len(account.Definition.RecordTypes)
					foundExact := false
					for _, original := range originals {
						foundExact = foundExact || original.DeveloperName == tc.incoming
					}
					if !foundExact {
						count++
					}
					personAccountIdentityFollowupApply(t, &org, route, tc.incoming)
					personAccountIdentityFollowupCheck(t, org, originals, tc.incoming, count)
					personAccountIdentityFollowupApply(t, &org, route, tc.incoming)
					personAccountIdentityFollowupCheck(t, org, originals, tc.incoming, count)

					data, err := json.Marshal(org)
					if err != nil {
						t.Fatal(err)
					}
					var restored storage.OrgState
					if err := json.Unmarshal(data, &restored); err != nil {
						t.Fatal(err)
					}
					personAccountIdentityFollowupCheck(t, restored, originals, tc.incoming, count)
					personAccountIdentityFollowupApply(t, &restored, route, tc.incoming)
					personAccountIdentityFollowupCheck(t, restored, originals, tc.incoming, count)
				})
			}
		}
	}
}

// Preserve base a recorded revision behavior for qualified inputs and candidates.
// The native oracle at org.tsv:20,40 rejects absent qualified references and
// contains no installed package, so it cannot justify changing these cases.
func TestPersonAccountQualifiedFoldLegacyFollowups(t *testing.T) {
	for _, name := range []string{"Care", "Member", "Provider"} {
		for _, scenario := range []string{"qualifiedIncoming", "qualifiedCandidate", "qualifiedLabel", "exactDuplicates", "noMatch"} {
			t.Run(name+"/"+scenario, func(t *testing.T) {
				incoming := "pkg__" + name
				originals := []storage.RecordTypeInfo{personAccountFlagFollowupRecordType(name, "Plain")}
				switch scenario {
				case "qualifiedCandidate":
					incoming = name
					originals[0] = personAccountFlagFollowupRecordType("pkg__"+name, "Qualified")
				case "qualifiedLabel":
					originals[0] = personAccountFlagFollowupRecordType("other__Business", name)
				case "exactDuplicates":
					originals = []storage.RecordTypeInfo{personAccountFlagFollowupRecordType(incoming, "First"), personAccountFlagFollowupRecordType(incoming, "Second")}
				case "noMatch":
					originals[0] = personAccountFlagFollowupRecordType("other__Other", "Other")
				}
				want := append([]storage.RecordTypeInfo(nil), originals...)
				if scenario == "exactDuplicates" {
					for i := range want {
						want[i].PersonAccount = true
					}
				}
				if scenario == "noMatch" {
					want = append(want, storage.RecordTypeInfo{DeveloperName: incoming, Name: incoming + " Label", Active: true, Available: true, PersonAccount: true})
				}
				org := personAccountFlagFollowupOrg(originals[0])
				state := org.Objects["Account"]
				state.Definition.RecordTypes = append([]storage.RecordTypeInfo(nil), originals...)
				org.Objects["Account"] = state
				personAccountIdentityFollowupApply(t, &org, "fold", incoming)
				if got := org.Objects["Account"].Definition.RecordTypes; !reflect.DeepEqual(got, want) {
					t.Fatalf("fold = %#v; want legacy %#v", got, want)
				}
			})
		}
	}
}

// Source-reference discovery still accepts labels and namespace aliases. It
// does not establish person provenance and keeps its pre-existing matcher.
func TestPersonAccountLegacyExistenceMatcherFollowups(t *testing.T) {
	recordTypes := []storage.RecordTypeInfo{{DeveloperName: "pkg__Care", Name: "Care Label"}}
	for _, name := range []string{"pkg__Care", "Care", "Care Label"} {
		if !profileRecordTypeExists(recordTypes, name) {
			t.Fatalf("legacy source reference %q no longer matches", name)
		}
	}
	if profileRecordTypeExists(recordTypes, "Other") {
		t.Fatal("unrelated source reference unexpectedly matched")
	}
}

func personAccountIdentityFollowupApply(t *testing.T, org *storage.OrgState, route, incoming string) {
	t.Helper()
	if route == "fold" {
		org.Objects["PersonAccount"] = storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName: "PersonAccount", RecordTypes: []storage.RecordTypeInfo{{DeveloperName: incoming, Name: incoming + " Label", Active: true, Available: true}},
		}}
		foldPersonAccountObjectIntoAccount(org)
		if _, exists := org.Objects["PersonAccount"]; exists {
			t.Fatal("PersonAccount metadata object survived folding")
		}
		return
	}
	root := "Profile"
	suffix := ".profile-meta.xml"
	if route == "permissionSet" {
		root = "PermissionSet"
		suffix = ".permissionset-meta.xml"
	}
	file := filepath.Join(t.TempDir(), "Identity"+suffix)
	writeFile(t, file, personAccountFlagFollowupVisibility(root, "PersonAccount."+incoming, true))
	switch route {
	case "profile":
		applyProjectProfileRecordTypes(org, project.Project{ProfileFiles: []string{file}})
	case "permissionSet":
		applyProjectPermissionSetRecords(org, project.Project{PermissionSetFiles: []string{file}}, nil)
	case "profilePermissions":
		storage.EnsureStandardObject(org, "Profile")
		applyProjectProfileRecords(org, project.Project{ProfileFiles: []string{file}}, nil)
	default:
		t.Fatalf("unknown route %q", route)
	}
}

func personAccountIdentityFollowupCheck(t *testing.T, org storage.OrgState, originals []storage.RecordTypeInfo, incoming string, count int) {
	t.Helper()
	recordTypes := org.Objects["Account"].Definition.RecordTypes
	if len(recordTypes) != count {
		t.Fatalf("record types = %#v; want %d", recordTypes, count)
	}
	for _, original := range originals {
		want := original
		if want.DeveloperName == incoming {
			want.PersonAccount = true
		}
		found := false
		for _, got := range recordTypes {
			if got.DeveloperName == original.DeveloperName {
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("identity %s changed: got %#v, want %#v", original.DeveloperName, got, want)
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("lost original identity %s: %#v", original.DeveloperName, recordTypes)
		}
	}
	matches := 0
	for _, recordType := range recordTypes {
		if recordType.DeveloperName == incoming {
			matches++
			if !recordType.PersonAccount {
				t.Fatalf("incoming identity %s is not a person type: %#v", incoming, recordType)
			}
		}
	}
	if matches != 1 {
		t.Fatalf("incoming identity %s has %d matches: %#v", incoming, matches, recordTypes)
	}
}

// Native metadata accepts unqualified developer references (org.tsv:3,24).
// Qualified ingestion remains the pre-existing projection until captured.
func TestPersonAccountQualifiedLegacyProjectionFollowups(t *testing.T) {
	for _, objectName := range []string{"Account", "PersonAccount"} {
		for _, name := range []string{"Care", "Member", "Provider"} {
			for _, route := range []string{"profile", "permissionSet", "profilePermissions"} {
				for _, existing := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%s/existing=%v", objectName, name, route, existing), func(t *testing.T) {
						original := personAccountFlagFollowupRecordType("pkg__"+name, name+" Label")
						org := personAccountFlagFollowupOrg(original)
						state := org.Objects["Account"]
						state.Definition.RecordTypes = nil
						if existing {
							state.Definition.RecordTypes = []storage.RecordTypeInfo{original}
						}
						org.Objects["Account"] = state
						root, suffix := "Profile", ".profile-meta.xml"
						if route == "permissionSet" {
							root, suffix = "PermissionSet", ".permissionset-meta.xml"
						}
						file := filepath.Join(t.TempDir(), "Legacy"+suffix)
						writeFile(t, file, personAccountFlagFollowupVisibility(root, objectName+".pkg__"+name, true))
						switch route {
						case "profile":
							applyProjectProfileRecordTypes(&org, project.Project{ProfileFiles: []string{file}})
						case "permissionSet":
							applyProjectPermissionSetRecords(&org, project.Project{PermissionSetFiles: []string{file}}, nil)
						case "profilePermissions":
							storage.EnsureStandardObject(&org, "Profile")
							applyProjectProfileRecords(&org, project.Project{ProfileFiles: []string{file}}, nil)
						}
						rts := org.Objects["Account"].Definition.RecordTypes
						if len(rts) != 1 {
							t.Fatalf("record types = %#v", rts)
						}
						if existing {
							if !reflect.DeepEqual(rts[0], original) {
								t.Fatalf("legacy identity changed: %#v", rts[0])
							}
						} else if rts[0].DeveloperName != name {
							t.Fatalf("legacy identity = %#v; want %s", rts[0], name)
						}
					})
				}
			}
		}
	}
}

// Unqualified references with an existing qualified candidate also retain the
// legacy existence matcher: the native oracle has no installed package.
func TestPersonAccountUnqualifiedWithQualifiedCandidateFollowups(t *testing.T) {
	for _, name := range []string{"Care", "Member", "Provider"} {
		for _, route := range []string{"profile", "permissionSet", "profilePermissions"} {
			t.Run(name+"/"+route, func(t *testing.T) {
				original := personAccountFlagFollowupRecordType("pkg__"+name, name+" Label")
				org := personAccountFlagFollowupOrg(original)
				state := org.Objects["Account"]
				state.Definition.RecordTypes = []storage.RecordTypeInfo{original}
				org.Objects["Account"] = state
				personAccountIdentityFollowupApply(t, &org, route, name)
				rts := org.Objects["Account"].Definition.RecordTypes
				if len(rts) != 1 || !reflect.DeepEqual(rts[0], original) {
					t.Fatalf("legacy qualified candidate changed: %#v", rts)
				}
			})
		}
	}
}

// Mixed qualified/unqualified metadata remains on the base matcher because
// the native collision capture did not include installed packages.
func TestPersonAccountMixedQualifiedLegacyFollowups(t *testing.T) {
	for _, name := range []string{"Care", "Member", "Provider"} {
		for _, route := range []string{"fold", "profile", "permissionSet", "profilePermissions"} {
			t.Run(name+"/"+route, func(t *testing.T) {
				originals := []storage.RecordTypeInfo{personAccountFlagFollowupRecordType("Business_"+name, name), personAccountFlagFollowupRecordType("pkg__Unrelated", "Unrelated")}
				org := personAccountFlagFollowupOrg(originals[0])
				state := org.Objects["Account"]
				state.Definition.RecordTypes = append([]storage.RecordTypeInfo(nil), originals...)
				org.Objects["Account"] = state
				personAccountIdentityFollowupApply(t, &org, route, name)
				if got := org.Objects["Account"].Definition.RecordTypes; !reflect.DeepEqual(got, originals) {
					t.Fatalf("mixed qualified metadata = %#v; want legacy %#v", got, originals)
				}
			})
		}
	}
}

func TestPersonAccountProfileIdentityDefaultsFollowups(t *testing.T) {
	for _, name := range []string{"Care", "Member", "Provider"} {
		file := filepath.Join(t.TempDir(), "Identity.profile-meta.xml")
		source := personAccountFlagFollowupVisibility("Profile", "Account.pkg__"+name, true)
		writeFile(t, file, strings.Replace(source, "<default>false</default>", "<default>true</default>", 1))
		if got := loadProfileRecordTypeDefaults(file)["Account"]; got != name {
			t.Errorf("default = %q; want %q", got, name)
		}
		got := loadProfileRecordTypeVisibilities(file)["account\x00"+strings.ToLower(name)]
		if got.DeveloperName != name {
			t.Errorf("visibility = %#v; want %q", got, name)
		}
	}
}
