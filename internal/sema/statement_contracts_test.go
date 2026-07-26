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
		"Boolean selector": `public class Probe { public void run() { switch on true { when true { } } } }`,
		"Decimal selector": `public class Probe { public void run() { switch on 1.0 { when 1.0 { } } } }`,
		"Date selector":    `public class Probe { public void run() { switch on Date.today() { when Date.today() { } } } }`,
		"duplicate branch": `public class Probe { public void run() { switch on 1 { when 1 { } when 1 { } } } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !result.HasErrors() {
				t.Fatalf("expected switch-contract diagnostic, got %#v", result.Diagnostics)
			}
		})
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
