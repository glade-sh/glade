package apextest

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// This alpha-renamed variant retains the creator-collection native rows and
// changes only the local Contact variable name from patch to Account. The
// original captured project remains covered separately, without transformation.
func TestFlowNamedResourceRelationshipAnonymousAndIsTestRoutes(t *testing.T) {
	runFlowRecordAssignmentNativeConformance(t, "flow_creator_collection", "GroveCreatorProbe", "GroveCreatorProbeTest", "observeCreator", "Grove_Creator", "FLOW_CREATOR_COLLECTION", []string{"C01", "C02", "C03"},
		func(t *testing.T, api string, classes, files map[string]string) typesys.Index {
			for path, source := range files {
				if strings.HasSuffix(path, ".flow-meta.xml") {
					files[path] = strings.NewReplacer(">patch<", ">Account<", ">patch.", ">Account.").Replace(source)
				}
			}
			return flowRefsIndex(t, api, classes, files)
		})
}
