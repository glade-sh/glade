package sema

import "testing"

func TestStatementContextRejectsIllegalBreakAndContinue(t *testing.T) {
	t.Parallel()
	for name, statement := range map[string]string{
		"break outside loop or switch":    "break;",
		"continue outside loop":           "continue;",
		"continue inside switch not loop": "switch on 1 { when 1 { continue; } }",
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"Probe.cls": "public class Probe { public void run() { " + statement + " } }",
			})
			if !result.HasErrors() {
				t.Fatalf("expected statement-context diagnostic, got %#v", result.Diagnostics)
			}
		})
	}
}

func TestStatementContractsRejectInvalidThrowAndCatchTypes(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"throw primitive":     `public class Probe { public void run() { throw 1; } }`,
		"catch non-exception": `public class Probe { public void run() { try { } catch (String error) { } } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !result.HasErrors() {
				t.Fatalf("expected exception-contract diagnostic, got %#v", result.Diagnostics)
			}
		})
	}
}

func TestStatementContractsRejectUnreachableInstructions(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public void run() { return; System.debug('unreachable'); } }`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected unreachable-code diagnostic, got %#v", result.Diagnostics)
	}
}

func TestStatementContractsAllowInstructionsAfterThrow(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public void run() { throw new NullPointerException(); Integer value = 1; } }`,
	})
	if result.HasErrors() {
		t.Fatalf("instructions after throw must remain compiler-compatible: %#v", result.Diagnostics)
	}
}

func TestSwitchContractsRejectUnsupportedSelectorsAndDuplicateValues(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"Boolean selector":              `public class Probe { public void run() { switch on true { when true { } } } }`,
		"Decimal selector":              `public class Probe { public void run() { switch on 1.0 { when 1.0 { } } } }`,
		"Date selector":                 `public class Probe { public void run() { switch on Date.today() { when Date.today() { } } } }`,
		"Datetime selector":             `public class Probe { public void run() { switch on Datetime.now() { when else { } } } }`,
		"mismatched branch":             `public class Probe { public void run() { switch on 'value' { when 1 { } } } }`,
		"numeric mismatch":              `public class Probe { public void run() { switch on 1 { when 1.0 { } } } }`,
		"duplicate branch":              `public class Probe { public void run() { switch on 1 { when 1 { } when 1 { } } } }`,
		"variable branch":               `public class Probe { public void run() { String expected = 'value'; switch on 'value' { when expected { } } } }`,
		"type branch on scalar":         `public class Probe { public void run() { switch on 'value' { when Account selected { } } } }`,
		"unrelated sobject type branch": `public class Probe { public void run(Account value) { switch on value { when Contact selected { } } } }`,
		"duplicate sobject type branch": `public class Probe { public void run(SObject value) { switch on value { when Account first { } when Account second { } } } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !result.HasErrors() {
				t.Fatalf("expected switch-contract diagnostic, got %#v", result.Diagnostics)
			}
		})
	}
}

func TestSwitchContractsAllowSupportedSelectorTypes(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public enum Choice { One, Two }
  public void run(Integer integerValue, Long longValue, String stringValue, Choice enumValue, Account accountValue) {
    switch on integerValue { when 1 { } }
    switch on longValue { when 1 { } }
    switch on stringValue { when 'value' { } }
    switch on enumValue { when One { } }
    switch on accountValue { when Account selected { } }
  }
}`,
	})
	if result.HasErrors() {
		t.Fatalf("supported switch selector was rejected: %#v", result.Diagnostics)
	}
}

func TestSwitchContractsAllowQualifiedPlatformEnumConstants(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public void run(JSONToken token, Schema.DisplayType displayType) {
    switch on token { when JSONToken.START_OBJECT { } when else { } }
    switch on displayType { when Schema.DisplayType.String { } when else { } }
  }
}`,
	})
	if result.HasErrors() {
		t.Fatalf("qualified platform enum constants were rejected: %#v", result.Diagnostics)
	}
}

func TestSwitchContractsAllowUnqualifiedJSONTokenCasesFromMethodResult(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public void run(JSONParser parser) {
    switch on parser.getCurrentToken() {
      when START_ARRAY, START_OBJECT {}
      when else {}
    }
  }
}`,
	})
	if result.HasErrors() {
		t.Fatalf("JSONParser token switch was rejected: %#v", result.Diagnostics)
	}
}

func TestSwitchContractsAllowUnqualifiedTriggerOperationCases(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public void run() {
    switch on Trigger.operationType {
      when BEFORE_INSERT, AFTER_UPDATE {}
      when else {}
    }
  }
}`,
	})
	if result.HasErrors() {
		t.Fatalf("Trigger operation switch was rejected: %#v", result.Diagnostics)
	}
}

func TestSwitchContractsAllowUnqualifiedAccessTypeCases(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public void run(System.AccessType accessType) {
    switch on accessType {
      when CREATABLE, UPDATABLE {}
      when else {}
    }
  }
}`,
	})
	if result.HasErrors() {
		t.Fatalf("AccessType switch was rejected: %#v", result.Diagnostics)
	}
}

func TestSwitchContractsPreferSelectorEnumWhenValueMatchesNestedType(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public License provider;
  public License license;
  public License dea;
  public License boardCert;
  public class License {
    public String sfObject;
    public Boolean hasSfObject() { return sfObject != null; }
  }
  public String run(MapType mapType) {
    License mapObject;
    switch on mapType {
      when PROVIDER { mapObject = this.provider; }
      when LICENSE { mapObject = this.license; }
      when DEA { mapObject = this.dea; }
      when BOARDCERT { mapObject = this.boardCert; }
    }
    return mapObject.hasSfObject() ? mapObject.sfObject : null;
  }
  public enum MapType { PROVIDER, LICENSE, DEA, BOARDCERT }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("selector enum value colliding with nested type was rejected: %#v", result.Diagnostics)
	}
}

func TestSwitchContractsKeepStringBranchValuesCaseSensitive(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public void run(String value) {
    switch on value {
      when 'Nsc', 'NSC' {}
      when else {}
    }
  }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("case-distinct String switch values were rejected: %#v", result.Diagnostics)
	}
}

func TestSwitchContractsResolveNestedEnumFromGenericReturn(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public enum Choice { One, Two }
  private static Map<String, Choice> choices = new Map<String, Choice>{'one' => Choice.One};
  public void run() {
    Choice value = choices.get('one');
    switch on value { when One { } when else { } }
  }
}`,
	})
	if result.HasErrors() {
		t.Fatalf("nested enum returned from a generic was rejected: %#v", result.Diagnostics)
	}
}

func TestSwitchContractsResolveClassicForInitializerEnum(t *testing.T) {
	enumResult := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public enum Choice { One, Two }
  public void run() {
    for (Choice value = Choice.One; false; value = Choice.Two) {
      switch on value { when One { } when else { } }
    }
  }
}`,
	})
	if enumResult.HasErrors() {
		t.Fatalf("enum for-loop selector was rejected: %#v", enumResult.Diagnostics)
	}
}

func TestStatementContractsRejectElseOrderingAndInvalidCustomExceptionName(t *testing.T) {
	t.Parallel()
	elseResult := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public void run() { switch on 1 { when else { } when 1 { } } } }`,
	})
	if !elseResult.HasErrors() {
		t.Fatalf("expected when-else ordering diagnostic, got %#v", elseResult.Diagnostics)
	}
	nameResult := analyzeDeclarationProject(t, map[string]string{
		"Bad.cls": `public class Bad extends Exception { }`,
	})
	if !nameResult.HasErrors() {
		t.Fatalf("expected custom Exception naming diagnostic, got %#v", nameResult.Diagnostics)
	}
}

func TestStatementContractsRejectAlreadyCoveredCatch(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public void run() { try { } catch (Exception first) { } catch (DmlException second) { } } }`,
	})
	if !result.HasErrors() {
		t.Fatalf("expected covered-catch diagnostic, got %#v", result.Diagnostics)
	}
}

func TestStatementContractsAllowLoopAndExceptionControls(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public void run() {
    while (true) { break; }
    try { throw new Exception(); } catch (Exception error) { System.debug(error); }
  }
}`,
	})
	if result.HasErrors() {
		t.Fatalf("unexpected statement-contract diagnostic: %#v", result.Diagnostics)
	}
}
