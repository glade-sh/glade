package aura

import "testing"

func TestParseLWCPassthroughSetupAssistant(t *testing.T) {
	source := `<!-- Strike Setup Assistant v3.0.2 -->
<aura:component>
    <aura:attribute name="isClassic" type="Boolean" />
    <c:setup isClassic="{!v.isClassic}" />
</aura:component>`
	target, ok := ParseLWCPassthrough(source, "samplepkg")
	if !ok {
		t.Fatal("expected passthrough")
	}
	if target != "samplepkg:setup" {
		t.Fatalf("target = %q", target)
	}
}

func TestParseLWCPassthroughRejectsMultipleChildren(t *testing.T) {
	source := `<aura:component>
    <c:one />
    <c:two />
</aura:component>`
	if _, ok := ParseLWCPassthrough(source, "c"); ok {
		t.Fatal("expected no passthrough for multiple LWC children")
	}
}

func TestParseLWCPassthroughRejectsAuraOnly(t *testing.T) {
	source := `<aura:component>
    <aura:attribute name="x" type="String" />
</aura:component>`
	if _, ok := ParseLWCPassthrough(source, "c"); ok {
		t.Fatal("expected no passthrough for aura-only component")
	}
}
