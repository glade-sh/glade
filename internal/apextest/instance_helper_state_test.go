package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestInstanceHelperStateAcrossSourceUnits(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, units := range []int{1, 2} {
			for _, init := range []struct{ name, field, body string }{
				{"field", " = new StateHelper()", ""},
				{"constructor", "", "public StateBox() { this.helper = new StateHelper(); }"},
				{"block", "", "{ this.helper = new StateHelper(); }"},
			} {
				t.Run(fmt.Sprintf("%s/units-%d/%s", api, units, init.name), func(t *testing.T) {
					root := t.TempDir()
					var paths []string
					for unit := 0; unit < units; unit++ {
						baseRoot := filepath.Join(root, fmt.Sprintf("unit%d", unit))
						writeFile(t, filepath.Join(baseRoot, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
						testName := fmt.Sprintf("StateReplay%dTest", unit)
						for _, source := range []struct{ name, body string }{
							{"StateHelper", `public class StateHelper {
 public String text = 'ready';
 public String read() { return this.text; }
}`},
							{"StateReader", `public interface StateReader { String read(); }`},
							{"StateBox", fmt.Sprintf(`public virtual class StateBox implements StateReader {
 private StateHelper helper%s;
 %s
 public String read() { return this.helper.read(); }
 public String readImplicit() { return helper.read(); }
 public String readPeer(StateBox peer) { return peer.helper.read(); }
 public StateHelper peek() { return helper; }
 public void replace() { helper = new StateHelper(); helper.text = 'implicit'; }
 public void replaceExplicit() { this.helper = new StateHelper(); this.helper.text = 'explicit'; }
 public void updatePeer(StateBox peer) { peer.helper.text = 'peer'; }
 public void clear() { helper = null; }
}`, init.field, init.body)},
							{"StateChild", `public class StateChild extends StateBox {}`},
							{testName, fmt.Sprintf(`@IsTest private class %s {
 @IsTest static void replay() {
  for (StateBox subject : new List<StateBox>{new StateBox(), new StateChild()}) {
   StateReader routed = subject;
   System.assertEquals('ready', routed.read());
   System.assertEquals('ready', subject.readImplicit());
   System.assertEquals('ready', subject.readPeer(subject));
   subject.replace();
   System.assertEquals('implicit', subject.read());
   System.assertEquals('implicit', subject.peek().read());
   subject.replaceExplicit();
   System.assertEquals('explicit', subject.readImplicit());
   subject.updatePeer(subject);
   System.assertEquals('peer', subject.read());
   System.assertEquals('peer', subject.readPeer(subject));
   subject.clear();
   System.assertEquals(null, subject.peek());
  }
 }
}`, testName)},
						} {
							path := filepath.Join(baseRoot, "force-app/classes", source.name+".cls")
							writeFile(t, path, source.body)
							paths = append(paths, path)
						}
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
					run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
					if got := run.Summary(); got.Total != units || got.Passed != units {
						t.Fatalf("summary=%+v, problem=%s", got, firstRunProblem(run))
					}
				})
			}
		}
	}
}
