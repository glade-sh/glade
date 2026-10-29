package vm

import (
	"runtime"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native membership: pending_notification_permissions/org.tsv OP_1/2/3 at
// API 62/67; pending_rows_remaining/org.tsv:48 supplies the API 63 boundary.
// Keep the same org schema while changing the executing source API. Neither a
// per-VM describe root nor a shared field map may reuse the other API's answer.
func TestObjectPermissionFieldMapSourceAPICache(t *testing.T) {
	for _, shared := range []bool{false, true} {
		name := "same VM"
		if shared {
			name = "shared runtime clones"
		}
		t.Run(name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.APIVersion = "67.0"
			storage.EnsureStandardObject(&org, "ObjectPermissions")
			template := storage.NewRuntimeTemplate(org)
			PrimeRuntimeTemplateSchema(&template)
			base := New(nil)
			base.SetRuntimeTemplateOrg(&template.Org)
			base.PrimeMetadataSchema(&template.Org)
			base.FreezeClassLookup()
			var machines []*VM // Retain shared entries across each API transition.
			for _, api := range []string{"67.0", "62.0", "63.0", "62.0", "67.0"} {
				machine := base
				if shared {
					machine = base.CloneRuntimeFrozenShared(nil)
					clonedOrg := template.CloneRuntimeOrg()
					machine.SetRuntimeTemplateOrg(&clonedOrg)
					machines = append(machines, machine)
				}
				program, err := CompileAnonymousWithOptions(`System.debug(Schema.getGlobalDescribe().get('ObjectPermissions').getDescribe().fields.getMap().containsKey('PermissionsViewAllFields'));`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatal(err)
				}
				want := "true"
				if api == "62.0" {
					want = "false"
				}
				if len(result.Debug) != 1 || result.Debug[0] != want {
					t.Fatalf("source API %s, org API 67: debug=%v; want %s", api, result.Debug, want)
				}
			}
			runtime.KeepAlive(machines)
		})
	}
}
