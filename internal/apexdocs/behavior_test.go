package apexdocs

import (
	"path/filepath"
	"testing"
)

func behaviorKinds(behaviors []DocBehavior) map[string]string {
	out := map[string]string{}
	for _, b := range behaviors {
		out[b.Kind] = b.Evidence
	}
	return out
}

func TestCollectBehaviorsCalloutInTest(t *testing.T) {
	root := t.TempDir()
	// The apostrophe is U+2019, matching the scraped docs, and the phrase
	// wraps lines exactly as Salesforce renders it.
	writeDoc(t, filepath.Join(root, "apex_System_PageReference_getContentAsPDF.md"), `# getContentAsPDF() Method

## Signature
`+"`public Blob getContentAsPDF()`"+`

## Usage
Use this method to generate a PDF. In a test method, any call to
getContentAsPDF is treated as a
callout and you can’t call it in a test.
`)

	inv, err := BuildInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	kinds := behaviorKinds(inv.Documents[0].Behaviors)
	if _, ok := kinds[BehaviorCalloutInTest]; !ok {
		t.Fatalf("expected callout-in-test, got %#v", kinds)
	}
}

func TestCollectBehaviorsThrowsAndDeprecated(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, filepath.Join(root, "apex_System_Foo_bar.md"), `# bar() Method

## Usage
This method is deprecated. It throws a LimitException when the governor
limit is exceeded.
`)

	inv, err := BuildInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	kinds := behaviorKinds(inv.Documents[0].Behaviors)
	if _, ok := kinds[BehaviorDeprecated]; !ok {
		t.Fatalf("expected deprecated, got %#v", kinds)
	}
	if got := kinds[BehaviorThrows]; got != "throws LimitException" {
		t.Fatalf("throws evidence = %q (%#v)", got, kinds)
	}
}

func TestCollectBehaviorsCantBeUsedIn(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, filepath.Join(root, "apex_System_Foo_baz.md"), `# baz() Method

## Usage
This method can’t be used in the following contexts:

- Triggers
- Batch Apex
`)

	inv, err := BuildInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	kinds := behaviorKinds(inv.Documents[0].Behaviors)
	if _, ok := kinds[BehaviorNotInTriggers]; !ok {
		t.Fatalf("expected not-in-triggers, got %#v", kinds)
	}
	if _, ok := kinds[BehaviorNotInBatch]; !ok {
		t.Fatalf("expected not-in-batch, got %#v", kinds)
	}
}

func TestCollectBehaviorsNoUsageNoMarkers(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, filepath.Join(root, "apex_System_Foo_qux.md"), `# qux() Method

## Signature
`+"`public void qux()`"+`

## Return Value
Type: void
`)

	inv, err := BuildInventory(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(inv.Documents[0].Behaviors); got != 0 {
		t.Fatalf("expected no behaviors, got %d", got)
	}
}
