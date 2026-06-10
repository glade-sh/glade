package aura

import "testing"

func TestParseOutAppDependencies(t *testing.T) {
	source := `<aura:application access="GLOBAL" extends="ltng:outApp">
    <aura:dependency resource="c:myWidget"/>
    <aura:dependency resource="c:anotherWidget"/>
</aura:application>`
	app, err := ParseOutApp("lightningOut", source)
	if err != nil {
		t.Fatal(err)
	}
	if app.Extends != "ltng:outApp" {
		t.Fatalf("extends = %q", app.Extends)
	}
	if len(app.Dependencies) != 2 || app.Dependencies[0] != "c:myWidget" {
		t.Fatalf("deps = %#v", app.Dependencies)
	}
}
