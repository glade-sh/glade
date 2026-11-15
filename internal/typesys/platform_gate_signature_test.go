package typesys

import (
	"os"
	"os/exec"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
)

func TestGatedNamespaceIndexSignatures(t *testing.T) {
	for _, word := range []string{"connectapi", "slack"} {
		name := "List<" + word + ".Thing>"
		shapes := []TypeSymbol{
			{Name: name},
			{Namespace: name},
			{SuperClass: name},
			{Interfaces: []string{name}},
			{Members: []MemberSymbol{{Type: name}}},
			{Members: []MemberSymbol{{Parameters: []apexast.Parameter{{Type: name}}}}},
		}
		for i := range shapes {
			if !typeSymbolNamesWord(&shapes[i], word) {
				t.Errorf("%s signature field %d was not observed", word, i)
			}
		}
	}
	if os.Getenv("GLADE_GATED_SIGNATURE_TEST") == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestGatedNamespaceIndexSignatures$")
		cmd.Env = append(os.Environ(), "GLADE_GATED_SIGNATURE_TEST=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("signature subprocess: %v\n%s", err, output)
		}
		return
	}
	before := StandardPlatformSymbolGeneration()
	NoteIndexSignatures(Index{Types: []TypeSymbol{{Members: []MemberSymbol{{Type: "Slack.App"}}}}})
	if after := StandardPlatformSymbolGeneration(); after <= before {
		t.Fatal("Slack signature did not invalidate the view")
	} else {
		before = after
	}
	NoteIndexSignatures(Index{Types: []TypeSymbol{{SuperClass: "ConnectApi.FeedItem"}}})
	if StandardPlatformSymbolGeneration() <= before || !ConnectAPIShapeRequested() {
		t.Fatal("ConnectApi signature did not request the captured shape and invalidate the view")
	}
}
