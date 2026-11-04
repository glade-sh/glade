package schema

import (
	"reflect"
	"testing"
)

func TestCase324MembershipFirstMatch(t *testing.T) {
	base := Object{Fields: []Field{{Name: "Alpha", Label: "first"}, {Name: "ALPHA", Label: "second"}, {Name: "K", Label: "kelvin"}}, RecordTypes: []RecordType{{DeveloperName: "Alpha"}, {DeveloperName: "ALPHA"}, {DeveloperName: "K"}}, ValidationRules: []ValidationRule{{Name: "Alpha"}, {Name: "ALPHA"}, {Name: "K"}}}
	overlay := Object{Fields: []Field{{Name: "alpha"}, {Name: "k"}, {Name: "New"}}, RecordTypes: []RecordType{{DeveloperName: "alpha"}, {DeveloperName: "k"}, {DeveloperName: "New"}}, ValidationRules: []ValidationRule{{Name: "alpha"}, {Name: "k"}, {Name: "New"}}}
	mergeObjectMetadata(&base, overlay)
	if got := []string{base.Fields[0].Label, base.Fields[1].Label, base.Fields[2].Name, base.Fields[3].Name}; !reflect.DeepEqual(got, []string{"first", "second", "K", "New"}) {
		t.Fatalf("fields: %v", got)
	}
	if len(base.RecordTypes) != 4 || base.RecordTypes[3].DeveloperName != "New" || len(base.ValidationRules) != 4 || base.ValidationRules[3].Name != "New" {
		t.Fatalf("members: %#v %#v", base.RecordTypes, base.ValidationRules)
	}
	for _, n := range []string{"alpha", "k", "K", "missing"} {
		if fieldIndex(base.Fields, n) != recordTypeIndex(base.RecordTypes, n) || fieldIndex(base.Fields, n) != validationRuleIndex(base.ValidationRules, n) {
			t.Fatalf("inconsistent first slot for %q", n)
		}
	}
}

func TestCase324IndexMatchesLinearFirstSlot(t *testing.T) {
	names := []string{"Alpha", "ALPHA", "K", "Other", "K", "ſ", "Σ", "ς", "\xff"}
	for split := 0; split <= len(names); split++ {
		var index memberIndex
		fields := make([]Field, 0, len(names))
		for i, name := range names[:split] {
			fields = append(fields, Field{Name: name})
			index.observe(name, i)
		}
		for _, query := range names {
			want := fieldIndex(fields, query)
			got := index.find(query, func() int { return fieldIndex(fields, query) })
			if got != want {
				t.Fatalf("split %d query %q: %d != %d", split, query, got, want)
			}
		}
	}
}
