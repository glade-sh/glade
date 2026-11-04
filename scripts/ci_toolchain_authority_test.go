package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestCIExplicitlyAuthorizesLWCCheckout(t *testing.T) {
	for _, spec := range []struct {
		file, root string
		jobs       []string
	}{
		{"../.github/workflows/ci.yml", "${{ github.workspace }}", []string{"node-integration", "server-and-playground", "test"}},
		{"../.github/workflows/browser.yml", "${{ github.workspace }}/source", []string{"browser"}},
		{"../.github/workflows/race.yml", "${{ github.workspace }}", []string{"plan", "race-browser", "race-early", "race", "race-push"}},
	} {
		data, err := os.ReadFile(spec.file)
		if err != nil {
			t.Fatal(err)
		}
		jobs := workflowJobBlocks(t, string(data))
		for _, job := range spec.jobs {
			if !strings.Contains(jobs[job], "      GLADE_ROOT: "+spec.root+"\n") {
				t.Errorf("%s job %s must explicitly authorize checkout %s", spec.file, job, spec.root)
			}
		}
	}
}
