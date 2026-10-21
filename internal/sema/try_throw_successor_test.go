package sema

import "testing"

func TestStatementContractsRejectCapturedTryThrowSuccessor(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"ReachabilityProbe.cls": `public class ReachabilityProbe {
 public class DisplayException extends Exception {}
 public void observe() {
  Map<String,Object> result = new Map<String,Object>();
  try {
   throw new DisplayException('Missing call.');
   result.put('provider',new List<String>());
  } catch(Exception error) {}
 }
}`,
	})
	for _, d := range result.Diagnostics {
		if d.NativeMessage == "Unreachable statement" {
			return
		}
	}
	t.Fatalf("S09 native rejection missing: %#v", result.Diagnostics)
}
