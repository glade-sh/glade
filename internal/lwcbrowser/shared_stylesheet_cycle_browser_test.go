package lwcbrowser

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwc/compile"
	"github.com/glade-sh/glade/internal/project"
)

func TestSharedStylesheetCyclesNativeBrowser(t *testing.T) {
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(repo, "third_party", "lwc", "node_modules", "@lwc", "engine-dom", "dist", "index.js")
	shadow := filepath.Join(repo, "third_party", "lwc", "node_modules", "@lwc", "synthetic-shadow", "dist", "index.js")
	playwright := filepath.Join(repo, "lwcruntime", "node_modules", "playwright")
	f, err := os.Open(filepath.Join("..", "lwc", "compile", "testdata", "pr163_stylesheet_cycles", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 11 {
		t.Fatalf("native rows %d: %v", len(rows), err)
	}
	for _, row := range rows[1:] {
		row := row
		if row[2] == "deploy" && row[0] != "missing" {
			continue
		}
		api, mode := row[1], row[0]
		t.Run(mode+"_"+api, func(t *testing.T) {
			root := t.TempDir()
			lwc := filepath.Join(root, "force-app", "main", "default", "lwc")
			writeLWCAPIVersionFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			meta := `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + api + `</apiVersion><isExposed>false</isExposed></LightningComponentBundle>`
			alpha := "@import 'c/betaStyles';"
			if mode == "missing" {
				alpha = "@import 'c/absentCycleStyles';"
			}
			beta := `.text { color: rgb(17, 34, 51); }`
			if mode == "cycle" {
				beta = "@import 'c/alphaStyles'; " + beta
			}
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "alphaStyles", "alphaStyles.css"), alpha)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "alphaStyles", "alphaStyles.js-meta.xml"), meta)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "betaStyles", "betaStyles.css"), beta)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "betaStyles", "betaStyles.js-meta.xml"), meta)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "probe", "probe.js"), `import { LightningElement } from 'lwc'; export default class Probe extends LightningElement {}`)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "probe", "probe.html"), `<template><span class="text">Native Style Probe</span></template>`)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "probe", "probe.css"), `@import 'c/alphaStyles';`)
			writeLWCAPIVersionFixtureFile(t, filepath.Join(lwc, "probe", "probe.js-meta.xml"), meta)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(root, "out")
			manifest, err := compile.Compile(p, compile.Options{OutDir: out, Namespace: "c", RepoRoot: repo, CompleteLocalModuleInventory: true})
			if mode == "missing" {
				if err == nil || !strings.Contains(err.Error(), row[4]) {
					t.Fatalf("missing module error %v, want %q", err, row[4])
				}
				return
			}
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
			imports, _ := json.Marshal(map[string]string{"lwc": "/lwc.js", "@lwc/synthetic-shadow": "/shadow.js", "c/probe": moduleURL("probe"), "c/alphaStyles": moduleURL("alphaStyles"), "c/betaStyles": moduleURL("betaStyles")})
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
(async()=>{await new Promise(r=>server.listen(0,'127.0.0.1',r));const b=await chromium.launch({headless:true});try {const p=await b.newPage();const errs=[];p.on('pageerror',e=>errs.push(e.message)); await p.goto('http://127.0.0.1:'+server.address().port,{waitUntil:'networkidle'});const n=p.locator('c-probe .text');await n.waitFor({timeout:6000}).catch(()=>{throw Error('missing marker: '+errs.join(';'))});const color=await n.evaluate(e=>getComputedStyle(e).color);if(color!==%q)throw Error('color '+color);if(errs.length)throw Error(errs.join(';'));}finally{await b.close();server.close();}})().catch(e=>{console.error(e);server.close();process.exit(1)});`, playwright, imports, out, engine, shadow, row[4])
			cmd := exec.Command("node", "-e", script)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("browser: %v\n%s", err, output)
			}
		})
	}
}
