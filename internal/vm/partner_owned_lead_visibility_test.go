package vm

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func partnerOwnedLeads(t testing.TB, manual bool) *VM {
	t.Helper()
	machine, _ := partnerLeadChildVisibilityMachine(t)
	machine.currentMethod = Method{APIVersion: "67.0"}
	user := storage.ID(machine.currentUserID())
	leads := machine.Org.Objects["Lead"]
	for id, row := range leads.Records {
		row.System.OwnerID, row.System.CreatedByID = user, user
		leads.PutRecord(id, row)
	}
	if manual {
		shares := machine.Org.Objects["LeadShare"]
		id := storage.ID("00T000000000201")
		shares.PutRecord(id, storage.Record{ID: id, Object: "LeadShare", Fields: map[string]storage.Value{
			"RowCause": storage.StringValue("Manual"), "LeadId": storage.IDValue("00Q000000000001"),
			"UserOrGroupId": storage.IDValue(user), "LeadAccessLevel": storage.StringValue("Read"),
		}})
		machine.Org.Objects["LeadShare"] = shares
	}
	machine.Org.Objects["Lead"] = leads
	return machine
}

func TestPartnerOwnedLeadsAvoidShareScan(t *testing.T) {
	for _, manual := range []bool{false, true} {
		t.Run(fmt.Sprintf("manual=%t", manual), func(t *testing.T) {
			machine := partnerOwnedLeads(t, manual)
			defer machine.beginPartnerLeadShareScan()()
			user := machine.currentUserID()
			for _, row := range machine.Org.Objects["Lead"].Records {
				if !machine.userModeRecordVisible("Lead", row, user) {
					t.Fatalf("owner cannot see %s", row.ID)
				}
			}
			if machine.partnerLeadShares.ready {
				t.Fatal("owner read walked the share table")
			}
		})
	}
}

func BenchmarkPartnerOwnedLeadVisibility(b *testing.B) {
	for _, manual := range []bool{false, true} {
		for _, scope := range []string{"direct", "scoped"} {
			b.Run(fmt.Sprintf("manual=%t/%s", manual, scope), func(b *testing.B) {
				machine := partnerOwnedLeads(b, manual)
				user := machine.currentUserID()
				rows := machine.Org.Objects["Lead"].Records
				read := func(ids *[]string) int {
					if scope == "scoped" {
						defer machine.beginPartnerLeadShareScan()()
					}
					count := 0
					for _, row := range rows {
						if machine.userModeRecordVisible("Lead", row, user) {
							count++
							if ids != nil {
								*ids = append(*ids, string(row.ID))
							}
						}
					}
					return count
				}
				var ids []string
				read(&ids)
				slices.Sort(ids)
				digest := fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(ids, ","))))
				if len(ids) != 200 || digest != "2f3002fc2ca62089b999e5711eb494ab1b321aad2967800716bf6cf614b29acc" {
					b.Fatalf("visible IDs = %d, digest %s", len(ids), digest)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if n := read(nil); n != len(ids) {
						b.Fatalf("visible = %d", n)
					}
				}
			})
		}
	}
}
