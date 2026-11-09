package vm

import "testing"

// An empty Visualforce page registry (the loader could not analyze any page)
// must not change the URL a page token reports: native lowercases every
// unqualified page (qualified_page_reference, case159_community_page).
func TestPageTokenURLIsLowercaseWithoutRegistry(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals('/apex/probehome', Page.ProbeHome.getUrl());
System.assertEquals('/apex/ns__MixedCase', Page.ns__MixedCase.getUrl());
PageReference ref = Page.ProbeDetail;
ref.getParameters().put('at', 'st');
System.assertEquals('/apex/probedetail?at=st', ref.getUrl());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	if machine.pageReferences != nil {
		t.Fatal("test needs an empty registry")
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
