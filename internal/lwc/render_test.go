package lwc

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestParseAPIProperties(t *testing.T) {
	source := `import { LightningElement, api } from 'lwc';
export default class Counter extends LightningElement {
  @api count = 0;
  @api label = 'Items';
  hidden = 1;
}`
	props, err := ParseAPIProperties(source)
	if err != nil {
		t.Fatal(err)
	}
	if props["count"] != "0" || props["label"] != "'Items'" {
		t.Fatalf("props = %#v", props)
	}
	if _, ok := props["hidden"]; ok {
		t.Fatalf("non-api field leaked: %#v", props)
	}
}

func TestResolveBindingPath(t *testing.T) {
	bag := PropertyBag{
		"count": IntValue(3),
		"row":   ObjectValue(map[string]Value{"label": StringValue("A")}),
	}
	if got, err := ResolveBinding("{count}", bag); err != nil || got != "3" {
		t.Fatalf("count = %q err=%v", got, err)
	}
	if got, err := ResolveBinding("{row.label}", bag); err != nil || got != "A" {
		t.Fatalf("row.label = %q err=%v", got, err)
	}
}

func TestRenderTemplateStaticBindingsAndDirectives(t *testing.T) {
	source := `<template>
  <p if:true={show}>{greeting}</p>
  <template for:each={items} for:item="row">
    <span key={row.id}>{row.label}</span>
  </template>
</template>`
	tree, err := ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	ctx := &RenderContext{
		Properties: PropertyBag{
			"show":     BoolValue(true),
			"greeting": StringValue("Hello"),
			"items": ArrayValue([]Value{
				ObjectValue(map[string]Value{
					"id":    StringValue("1"),
					"label": StringValue("One"),
				}),
			}),
		},
	}
	html, err := RenderTemplate(tree, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "Hello") || !strings.Contains(html, "One") {
		t.Fatalf("html = %q", html)
	}
}

func TestRenderLightningButtonStub(t *testing.T) {
	tree, err := ParseTemplate(`<template><lightning-button label={btnLabel} variant="brand"></lightning-button></template>`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := RenderTemplate(tree, &RenderContext{
		Properties: PropertyBag{"btnLabel": StringValue("Save")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `<button`) || !strings.Contains(out, `Save`) {
		t.Fatalf("out = %q", out)
	}
}

func TestRenderCustomComponentComposition(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "lwc-rendering")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := BuildIndex(p)
	if err != nil {
		t.Fatal(err)
	}
	parent, ok := idx.Bundle("parent")
	if !ok {
		t.Fatal("missing parent bundle")
	}
	htmlSource, err := parent.ReadHTML()
	if err != nil {
		t.Fatal(err)
	}
	tree, err := ParseTemplate(htmlSource)
	if err != nil {
		t.Fatal(err)
	}
	out, err := RenderTemplate(tree, &RenderContext{
		Index:      idx,
		Properties: PropertyBag{"greeting": StringValue("Hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Hi") {
		t.Fatalf("out = %q", out)
	}
}

func TestRenderComponentUsesAPIDefaults(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "lwc-rendering")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := BuildIndex(p)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RenderComponent(RenderRequest{
		Index:         idx,
		ComponentName: "counter",
		Overrides:     map[string]string{"count": "5"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "5") {
		t.Fatalf("html = %q", result.HTML)
	}
}
