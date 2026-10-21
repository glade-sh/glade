package sema

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSemaBindingResolverNestedFieldsBoundedWork(t *testing.T) {
	var source strings.Builder
	source.WriteString("public class BindingEnvelope { public static String outerName; public void unrelated() {")
	for i := 0; i < 512; i++ {
		fmt.Fprintf(&source, "Integer local%d = %d;\n", i, i)
	}
	source.WriteString("}\n")
	for i := 0; i < 24; i++ {
		fmt.Fprintf(&source, "public class Nested%d { public String innerName; }\n", i)
	}
	source.WriteString("}")
	text := source.String()
	spans := newSemaCodeSpans(text)
	outer := strings.IndexByte(text, '{')
	var nested []int
	for i := 0; i < 24; i++ {
		start := strings.Index(text, fmt.Sprintf("public class Nested%d {", i))
		nested = append(nested, start+strings.IndexByte(text[start:], '{'))
	}
	measure := func(starts []int) (float64, time.Duration) {
		started := time.Now()
		allocs := testing.AllocsPerRun(1, func() {
			resolver := newSemaBindingResolver(text, spans)
			for _, start := range starts {
				fields := resolver.fieldBindings(start)
				if len(fields) == 0 {
					t.Fatal("lost enclosing fields")
				}
			}
		})
		return allocs, time.Since(started)
	}
	outerAllocs, outerTime := measure([]int{outer})
	nestedAllocs, nestedTime := measure(nested)
	t.Logf("outer scan %.0f allocations / %s; 24 nested owners %.0f allocations / %s", outerAllocs, outerTime, nestedAllocs, nestedTime)
	// Nested owners add small declarations. They must not each rediscover the
	// hundreds of unrelated locals in the enclosing body. An allocation bound
	// avoids depending on the machine's timing or scheduling.
	if limit := 3*outerAllocs + 500; nestedAllocs > limit {
		t.Fatalf("nested owners allocated %.0f, want <= %.0f: enclosing declarations rescanned", nestedAllocs, limit)
	}
}

func TestSemaBindingResolverNestedFieldsPreserveScopes(t *testing.T) {
	const source = `public class BindingEnvelope {
 public static String sharedName;
 public static Integer outerOnly;
 public String instanceOnly;
 public void unrelated() { Decimal hiddenLocal; }
 public class First {
  public Boolean sharedName;
  public String ownField;
  public void run() { Date ownField; Object rows = [SELECT Id FROM Account WHERE Name = :sharedName]; }
 }
 public class Second {
  public Id sharedName;
  public static void run() { Object rows = [SELECT Id FROM Account WHERE Name = :sharedName]; }
 }
}`
	first := strings.Index(source, ":sharedName")
	second := strings.LastIndex(source, ":sharedName")
	for _, order := range [][]int{{first, second, first}, {second, first, second}} {
		resolver := newSemaBindingResolver(source, newSemaCodeSpans(source))
		for _, offset := range order {
			bindings := resolver.bindingsAt(offset)
			if bindings["outeronly"] != "Integer" || bindings["hiddenlocal"] != "" {
				t.Fatalf("enclosing scope = %#v", bindings)
			}
			if offset == first {
				if bindings["sharedname"] != "Boolean" || bindings["ownfield"] != "Date" {
					t.Fatalf("instance/local shadowing = %#v", bindings)
				}
			} else if bindings["sharedname"] != "String" || bindings["instanceonly"] != "" || bindings["ownfield"] != "" {
				t.Fatalf("static/sibling visibility = %#v", bindings)
			}
		}
	}
}
