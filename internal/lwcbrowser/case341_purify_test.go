package lwcbrowser

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCase341PurifyFailsClosed(t *testing.T) {
	path, err := filepath.Abs("../../lwcruntime/src/lightning/purifyLib.mjs")
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	shim, ok := LightningUtilityModuleJS("purifyLib")
	if !ok {
		t.Fatal("missing shim")
	}
	sources, err := json.Marshal([]string{string(runtime), shim})
	if err != nil {
		t.Fatal(err)
	}
	script := `globalThis.document={createElement:()=>({innerHTML:'',content:{querySelectorAll:()=>[]}})};
const sources=JSON.parse(process.argv[1]);
for(const source of sources){const mod=await import('data:text/javascript;base64,'+Buffer.from(source).toString('base64'));
for(const value of ['<img src=x onerror="void 0">','<svg onload="void 0"></svg>','<b>safe</b>']){
let threw=false; try { mod.default(value); } catch(e) { threw=true; if(!String(e.message).includes('unsupported')) throw e; }
if(!threw) throw Error('unsafe helper did not fail closed');}}
`
	cmd := exec.Command("node", "--input-type=module", "-e", script, string(sources))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("purify helper: %v\n%s", err, out)
	}
}
