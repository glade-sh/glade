package server

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// replayCapturedEnclosingStaticLWC is called by the gated L08 browser test.
// It replays the owned LWC-to-Apex observations at LWC 59/Apex 62 and 67/67.
func replayCapturedEnclosingStaticLWC(t *testing.T, repo, playwright string) {
	t.Helper()
	const fixture = "testdata/enclosing_static_lwc"
	readTSV := func(name string) [][]string {
		t.Helper()
		file, err := os.Open(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		reader := csv.NewReader(file)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	rows, cases := readTSV("org.tsv"), readTSV("cases.tsv")
	if len(rows) != 5 || len(cases) != 3 || strings.Join(rows[0], ",") != "id,lwc_api,apex_api,route,outcome" || strings.Join(cases[0], ",") != "id,lwc_api,apex_api,component,controller" {
		t.Fatal("incomplete native LWC capture matrix")
	}
	type browserCase struct {
		ID        string `json:"id"`
		Component string `json:"component"`
		Tag       string `json:"tag"`
	}
	wanted := map[string]string{}
	var browserCases []browserCase
	for _, item := range cases[1:] {
		if len(item) != 5 || item[0] != "enclosing_inherited" {
			t.Fatalf("invalid case row: %v", item)
		}
		id := item[1] + "/" + item[2]
		if _, duplicate := wanted[id]; duplicate {
			t.Fatalf("duplicate case: %s", id)
		}
		matches := 0
		for _, row := range rows[1:] {
			if len(row) != 5 {
				t.Fatalf("invalid native row: %v", row)
			}
			if row[0] == item[0] && row[1] == item[1] && row[2] == item[2] && row[3] == "lwc-browser" {
				matches++
				wanted[id] = row[4]
			}
		}
		if matches != 1 || wanted[id] != "passed" {
			t.Fatalf("missing native browser outcome for %s", id)
		}
		rejected := 0
		for _, row := range rows[1:] {
			if row[0] == "instance_rejected" && row[1] == item[1] && row[2] == item[2] && row[3] == "metadata-deploy" && strings.HasPrefix(row[4], "Method does not exist or incorrect signature:") {
				rejected++
			}
		}
		if rejected != 1 {
			t.Fatalf("missing native rejection control for %s", id)
		}
		t.Run("reject_"+id, func(t *testing.T) {
			root := t.TempDir()
			l08WriteProject(t, root, item[2])
			body, err := os.ReadFile(filepath.Join(fixture, "reject.apex"))
			if err != nil {
				t.Fatal(err)
			}
			l08WriteClass(t, root, "RejectProbe", string(body), item[2])
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := l08Index(p); err == nil {
				t.Fatal("native rejected instance call was accepted")
			}
		})
		browserCases = append(browserCases, browserCase{ID: id, Component: item[3]})
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
		t.Fatal(err)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	index, err := l08Index(p)
	if err != nil {
		t.Fatalf("native accepted project: %v", err)
	}
	source, err := NewSourceMetadataFromProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	handler := NewWithSource(&org, source)
	handler.SetProjectIndex(index)
	defer handler.ResetLightningCache()
	if handler.runtimeErr != nil {
		t.Fatal(handler.runtimeErr)
	}
	if err := handler.ensureLightningLocked(); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]string{}
	for i := range browserCases {
		entry, ok := handler.lightning.compiled.Modules["c:"+browserCases[i].Component]
		if !ok {
			t.Fatalf("compiled module missing: %s", browserCases[i].Component)
		}
		browserCases[i].Tag = entry.Tag
		allowed[browserCases[i].Component] = entry.Tag
	}
	bootstrap := lwcbrowser.BootstrapHTML(handler.lightning.pageCfg)
	host := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.URL.Path == "/" {
			component := r.URL.Query().Get("component")
			tag, ok := allowed[component]
			if !ok {
				http.Error(w, "unknown component", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html>%s<main></main><script type="module">import "@lwc/synthetic-shadow";import {createElement} from "lwc";import Page from "c/%s";const el=createElement(%q,{is:Page});document.querySelector("main").appendChild(el);</script>`, bootstrap, component, tag)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer host.Close()
	config, err := json.Marshal(map[string]any{"url": host.URL, "cases": browserCases, "playwrightModule": playwright, "executablePath": os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE")})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(repo, "internal/server/testdata/enclosing_static_lwc/browser.mjs"))
	cmd.Stdin = bytes.NewReader(config)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("local LWC browser: %v: %s", err, l08OneLine(stderr.String()))
	}
	var got struct {
		Values map[string]string `json:"values"`
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	for id, want := range wanted {
		if got.Errors[id] != "" || got.Values[id] != want {
			t.Errorf("%s: local %q, native %q, error %q", id, got.Values[id], want, got.Errors[id])
		}
	}
}
