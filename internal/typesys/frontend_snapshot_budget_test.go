package typesys

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
)

// A large project's authenticated snapshot must restore. The fixed 256 MiB
// decoded-graph budget charged about 20 bytes per encoded byte, so snapshots
// above roughly 12 MiB (about 500 classes) were rejected on every warm run and
// the frontend was rebuilt from source.
func TestFrontendSnapshotLargeProjectRestoresWithinBudget(t *testing.T) {
	var index Index
	for class := 0; class < 1200; class++ {
		sym := TypeSymbol{Kind: apexast.DeclarationClass, Name: fmt.Sprintf("LargeProjectClass%04d", class), File: fmt.Sprintf("classes/LargeProjectClass%04d.cls", class), SourceBacked: true}
		for member := 0; member < 40; member++ {
			// Declaration records without source chunks charge more per encoded
			// byte than a whole project snapshot (21 to 27); the scaled budget
			// must still admit them.
			sym.Members = append(sym.Members, MemberSymbol{
				Kind:       apexast.DeclarationMethod,
				Name:       fmt.Sprintf("calculateProjectedRevenueForSegment%02d", member),
				Type:       "Map<String, List<LargeProjectClass0001.RevenueProjection>>",
				Modifiers:  []string{"public", "static"},
				Parameters: []apexast.Parameter{{Name: "opportunityRecordsBySegment", Type: "Map<String, List<Opportunity>>"}, {Name: "fiscalQuarterStartDate", Type: "Date"}},
			})
		}
		index.Types = append(index.Types, sym)
	}
	var stream bytes.Buffer
	if err := binary.Write(&stream, binary.LittleEndian, uint32(FrontendSnapshotVersion)); err != nil {
		t.Fatal(err)
	}
	if err := writeFrontendRecord(&stream, frontendStreamHeader{Types: len(index.Types), TypesPresent: true}); err != nil {
		t.Fatal(err)
	}
	cost := int64(0)
	for _, sym := range index.Types {
		var record bytes.Buffer
		if err := writeFrontendRecord(&record, frontendType{TypeSymbol: sym}); err != nil {
			t.Fatal(err)
		}
		probe := frontendDecodeBudget{remaining: 1 << 62}
		if err := probe.validate(record.Bytes()[4:]); err != nil {
			t.Fatal(err)
		}
		cost += (1 << 62) - probe.remaining
		stream.Write(record.Bytes())
	}
	if cost <= frontendDecodeBudgetMinimum {
		t.Fatalf("fixture charges %d bytes; it must exceed the %d byte minimum budget to exercise the scaled bound", cost, frontendDecodeBudgetMinimum)
	}
	digest := sha256.Sum256(stream.Bytes())
	stream.Write(digest[:])
	wire, err := readFrontendPayload(stream.Bytes())
	if err != nil {
		t.Fatalf("large snapshot rejected: %v", err)
	}
	if len(wire.Index.Types) != len(index.Types) {
		t.Fatalf("restored %d types, want %d", len(wire.Index.Types), len(index.Types))
	}
}

func TestFrontendDecodeBudgetKeepsFixedBoundForSmallSnapshots(t *testing.T) {
	if got := frontendDecodeBudgetFor(1 << 20); got != frontendDecodeBudgetMinimum {
		t.Fatalf("1 MiB snapshot budget = %d, want the fixed minimum %d", got, frontendDecodeBudgetMinimum)
	}
	if got := frontendDecodeBudgetFor(64 << 20); got != 64<<20*frontendDecodeBudgetPerEncodedByte {
		t.Fatalf("64 MiB snapshot budget = %d, want %d", got, 64<<20*frontendDecodeBudgetPerEncodedByte)
	}
}
