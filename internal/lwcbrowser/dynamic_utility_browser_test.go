package lwcbrowser

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/project"
)

func TestDynamicUtilityBrowserExecution(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(repo, "third_party", "lwc", "node_modules", "@lwc", "engine-dom", "dist", "index.js")
	shadow := filepath.Join(repo, "third_party", "lwc", "node_modules", "@lwc", "synthetic-shadow", "dist", "index.js")
	playwright := filepath.Join(repo, "lwcruntime", "node_modules", "playwright")
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			lwc := filepath.Join(root, "force-app", "main", "default", "lwc")
			writeLWCAPIVersionFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			meta := `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`
			utilMeta := strings.Replace(meta, "</LightningComponentBundle>", `<capabilities><capability>lightning__dynamicComponent</capability></capabilities></LightningComponentBundle>`, 1)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "util", "util.js"), `export async function load(name) { return await import(name); }`)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "util", "util.js-meta.xml"), utilMeta)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "child", "child.js"), `import { LightningElement } from 'lwc'; export default class Child extends LightningElement {}`)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "child", "child.html"), `<template><p>Child</p></template>`)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "child", "child.js-meta.xml"), meta)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "probe", "probe.js"), `import { LightningElement } from 'lwc'; import { load } from 'c/util'; export default class Probe extends LightningElement { status = 'pending'; async connectedCallback() { const module = await load('c/child'); this.status = module.default ? 'loaded' : 'missing'; } }`)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "probe", "probe.html"), `<template><span class="status">{status}</span></template>`)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "probe", "probe.js-meta.xml"), meta)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(root, "out")
			manifest, err := compile.Compile(p, compile.Options{OutDir: out, Namespace: "c", RepoRoot: repo})
			if err != nil {
				t.Fatal(err)
			}
			moduleURL := func(name string) string {
				entry, ok := manifest.Modules["c:"+name]
				if !ok {
					t.Fatalf("missing module %s", name)
				}
				rel, err := filepath.Rel(out, entry.File)
				if err != nil {
					t.Fatal(err)
				}
				return "/modules/" + filepath.ToSlash(rel)
			}
			imports, _ := json.Marshal(map[string]string{"lwc": "/lwc.js", "@lwc/synthetic-shadow": "/shadow.js", "c/probe": moduleURL("probe"), "c/util": moduleURL("util"), "c/child": moduleURL("child")})
			script := fmt.Sprintf(`
const {chromium}=require(%q), http=require('node:http'), fs=require('node:fs'), path=require('node:path');
const imports=%s, out=%q, engine=%q, shadow=%q;
const server=http.createServer((req,res)=>{
  res.setHeader('Content-Type',req.url==='/'?'text/html':'application/javascript');
  if(req.url==='/') return res.end('<!doctype html><script>window.process={env:{NODE_ENV:"production"}}</script><script type="importmap">'+JSON.stringify({imports})+'</script><div id="host"></div><script type="module" src="/entry.js"></script>');
  if(req.url==='/entry.js') return res.end("import '@lwc/synthetic-shadow'; import {createElement} from 'lwc'; import Probe from 'c/probe'; document.getElementById('host').appendChild(createElement('c-probe',{is:Probe}));");
  const f=req.url==='/lwc.js'?engine:req.url==='/shadow.js'?shadow:req.url.startsWith('/modules/')?path.join(out,req.url.substring('/modules/'.length)):null;
  if(!f || !fs.existsSync(f)){res.statusCode=404;return res.end();} res.end(fs.readFileSync(f));
});
(async()=>{await new Promise(r=>server.listen(0,'127.0.0.1',r));const b=await chromium.launch({headless:true});try {const p=await b.newPage();const errs=[];p.on('pageerror',e=>errs.push(e.message)); await p.goto('http://127.0.0.1:'+server.address().port,{waitUntil:'networkidle'});const n=p.locator('c-probe .status');await n.waitFor();await p.waitForFunction(()=>document.querySelector('c-probe')?.shadowRoot?.querySelector('.status')?.textContent==='loaded');if(errs.length)throw Error(errs.join(';'));}finally{await b.close();server.close();}})().catch(e=>{console.error(e);server.close();process.exit(1)});`, playwright, imports, out, engine, shadow)
			cmd := exec.Command("node", "-e", script)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("browser: %v\n%s", err, output)
			}
		})
	}
}
