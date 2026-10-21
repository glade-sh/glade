package sema

import "testing"

func TestAnalyzeDatacloudSafeFieldCollectionType(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"MatchProbe.cls": `public class MatchProbe {
  public List<Datacloud.MatchRecord> read(Datacloud.MatchResult value) {
    return value?.matchRecords ?? new List<Datacloud.MatchRecord>();
  }
}`,
	}, "66.0")
	if result.HasErrors() {
		t.Fatalf("documented MatchResult.matchRecords property type was lost: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSafeNavigationStaticFieldWithNestedTypeName(t *testing.T) {
	t.Parallel()
	// Apex Developer Guide: Namespace, Class, and Variable Name Precedence.
	// In a class-qualified expression, a static variable is resolved before
	// interpreting the same spelling as a nested type.
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"ReceiverProbe.cls": `public class ReceiverProbe {
  public interface Worker { Object apply(Object value); }
  public static Worker worker;
  public class Request {
    public Object run(Object value) { return ReceiverProbe.worker?.apply(value); }
  }
}`,
	}, "66.0")
	if result.HasErrors() {
		t.Fatalf("safe navigation on a static field value was treated as a type receiver: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSafeNavigationStaticPropertyWithNestedTypeName(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"ReceiverProbe.cls": `public class ReceiverProbe {
  public interface Worker { Object apply(Object value); }
  public static Worker worker { get; private set; }
  public class Request {
    public Object run(Object value) { return ReceiverProbe.worker?.apply(value); }
  }
}`,
	}, "66.0")
	if result.HasErrors() {
		t.Fatalf("safe navigation on a static property value was treated as a type receiver: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSafeNavigationStillRejectsTypeReceiver(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"ReceiverProbe.cls": `public class ReceiverProbe {
  public class Worker { public String text; }
  public Object run() { return ReceiverProbe.Worker?.text; }
}`,
	}, "66.0")
	if !result.HasErrors() {
		t.Fatal("safe navigation on an actual nested type receiver was accepted")
	}
}
