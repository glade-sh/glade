package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// org.tsv q3share:2-5: both named standard shares expose Manual.
func TestQ3StandardShareReasonLookup(t *testing.T) {
	for _, name := range []string{"AccountShare", "OpportunityShare"} {
		t.Run(name, func(t *testing.T) {
			vm := New(nil)
			d, ok := storage.StandardObjectDefinition(name)
			if !ok {
				t.Fatal("missing standard object")
			}
			f, present := d.Fields["RowCause"]
			got, ok := vm.lookupSObjectSharingReasonToken([]string{"Schema", name, "RowCause", "Manual"})
			if !ok || got.Text != "Manual" {
				t.Fatalf("token=%#v ok=%v; field present=%v type=%s values=%#v", got, ok, present, f.Type, f.PicklistValues)
			}
		})
	}
}

func TestQ3SharingReasonLocalRootsShadowTokens(t *testing.T) {
	for _, name := range []string{"AccountShare", "Schema"} {
		t.Run(name, func(t *testing.T) {
			machine := New(nil)
			reason := Object("Q3Reason")
			reason.Fields["Manual"] = String("local")
			share := Object("Q3Share")
			share.Fields["RowCause"] = reason
			path := "AccountShare.RowCause.Manual"
			root := share
			if name == "Schema" {
				root = Object("Q3Root")
				root.Fields["AccountShare"] = share
				path = "Schema." + path
			}
			machine.Globals[name] = root
			got, err := machine.lookup(path)
			if err != nil || got.Kind != ValueString || got.Text != "local" {
				t.Fatalf("local root: %#v err=%v", got, err)
			}
		})
	}
}
