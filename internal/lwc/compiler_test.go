package lwc

import "testing"

func TestParseTemplateExtractsBindingAndDirective(t *testing.T) {
	source := `<template>
  <p if:true={show}>{greeting}</p>
  <ul>
    <template for:each={items} for:item="row">
      <li key={row.id}>{row.label}</li>
    </template>
  </ul>
</template>`
	tree, err := ParseTemplate(source)
	if err != nil {
		t.Fatal(err)
	}
	if tree.Tag != "template" {
		t.Fatalf("root = %#v", tree)
	}
	p := findFirstElement(tree, "p")
	if p == nil || p.Directives["if:true"] != "{show}" {
		t.Fatalf("p node = %#v", p)
	}
	if len(p.Children) != 1 || p.Children[0].Text != "{greeting}" {
		t.Fatalf("p children = %#v", p.Children)
	}
	li := findFirstElement(tree, "li")
	if li == nil || li.Directives["key"] != "{row.id}" {
		t.Fatalf("li node = %#v", li)
	}
}

func findFirstElement(root *TemplateNode, tag string) *TemplateNode {
	if root == nil {
		return nil
	}
	if root.Type == NodeElement && root.Tag == tag {
		return root
	}
	for _, child := range root.Children {
		if found := findFirstElement(child, tag); found != nil {
			return found
		}
	}
	return nil
}
