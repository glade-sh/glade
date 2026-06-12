package lwc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestBuildIndexGroupsLWCBundleFiles(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "force-app/main/default/lwc/counter")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	writeBundleFile(t, filepath.Join(base, "counter.js"), `export default class Counter {}`)
	writeBundleFile(t, filepath.Join(base, "counter.html"), `<template><p>{count}</p></template>`)
	writeBundleFile(t, filepath.Join(base, "counter.css"), `.title { color: red; }`)
	writeBundleFile(t, filepath.Join(base, "counter.js-meta.xml"), `<LightningComponentBundle xmlns="http://soap.sforce.com/2006/04/metadata"><isExposed>true</isExposed></LightningComponentBundle>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := BuildIndex(p)
	if err != nil {
		t.Fatal(err)
	}
	bundle, ok := idx.Bundle("counter")
	if !ok {
		t.Fatal("missing counter bundle")
	}
	if bundle.JSFile == "" || bundle.HTMLFile == "" || bundle.CSSFile == "" || bundle.MetaFile == "" {
		t.Fatalf("bundle = %#v", bundle)
	}
	html, err := bundle.ReadHTML()
	if err != nil {
		t.Fatal(err)
	}
	if html == "" {
		t.Fatal("expected html content")
	}
}

func writeBundleFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
