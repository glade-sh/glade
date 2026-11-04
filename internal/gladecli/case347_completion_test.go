package gladecli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/cliui"
)

func TestExecProjectCompletionAliases(t *testing.T) {
	ref, ok := cliui.FindCommandHelp("exec")
	if !ok {
		t.Fatal("missing exec reference")
	}
	bash := " " + strings.Join(bashFlags(ref), " ") + " "
	for _, name := range []string{" --project ", " -p "} {
		if !strings.Contains(bash, name) {
			t.Errorf("bash missing %q", name)
		}
	}
	if strings.Contains(bash, "--project,") {
		t.Error("bash contains combined option")
	}
	zsh := strings.Join(zshFlags(ref), "\n")
	for _, name := range []string{"--project[", "-p["} {
		if !strings.Contains(zsh, name) {
			t.Errorf("zsh missing %q", name)
		}
	}
	var fish bytes.Buffer
	if err := writeFishCompletion(&fish); err != nil {
		t.Fatal(err)
	}
	for _, option := range []string{"__fish_seen_subcommand_from exec' -l project -r", "__fish_seen_subcommand_from exec' -s p -r"} {
		if !strings.Contains(fish.String(), option) {
			t.Errorf("fish missing %q", option)
		}
	}
}
