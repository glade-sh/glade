package sema

import (
	"strings"
	"testing"
)

func TestThrowOnlyStepResultAllowsQuotedSemicolon(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"StepResult.cls":            `public class StepResult {}`,
		"ProbeFailureException.cls": `public class ProbeFailureException extends Exception {}`,
		"ThrowCompletion.cls": `public class ThrowCompletion {
  public StepResult execute() {
    throw new ProbeFailureException('first clause; second clause');
  }
}`,
		"MissingCompletion.cls": `public class MissingCompletion {
  public StepResult execute(Boolean fail) {
    if (fail) {
      throw new ProbeFailureException('failure; still no return');
    }
  }
}`,
	})

	var throwOnlyReturnDiagnostic, missingPathDiagnostic bool
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code != "GLADESEMA019" || !strings.Contains(diagnostic.Message, "must return StepResult") {
			continue
		}
		switch {
		case strings.HasSuffix(diagnostic.File, "/ThrowCompletion.cls"):
			throwOnlyReturnDiagnostic = true
		case strings.HasSuffix(diagnostic.File, "/MissingCompletion.cls"):
			missingPathDiagnostic = true
		}
	}
	if throwOnlyReturnDiagnostic {
		t.Fatalf("a throw-only method with a quoted semicolon should terminate: %#v", result.Diagnostics)
	}
	if !missingPathDiagnostic {
		t.Fatalf("a non-throwing path without a return should be diagnosed: %#v", result.Diagnostics)
	}
}
