package visualforce_test

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/visualforce"
)

// Native postback.tsv records the Save action result after this control's
// values are posted. The named submit control identifies the action.
func TestNativeCommandButtonPostbackSelectsAction(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := posProject(t, "testdata/vf_render_markup", "", "", api, map[string]bool{"PstTextId": true})
			machine, project, index := ispickvalMachine(t, root)
			result, err := visualforce.RenderPage(visualforce.PageRenderRequest{
				Project: project, VFIndex: index, Machine: machine, PageName: "PstTextId",
				FormValues: map[string]string{"j_id0:f1:i1": "v1", "j_id0:f1:btn": "Save"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Error != nil {
				t.Fatal(result.Error.Message)
			}
			if !strings.Contains(result.HTML, "hid=null|s=v1|t=null|b=null|sel=null|mode=null|") {
				t.Fatalf("native action result missing: %s", result.HTML)
			}
		})
	}
}
