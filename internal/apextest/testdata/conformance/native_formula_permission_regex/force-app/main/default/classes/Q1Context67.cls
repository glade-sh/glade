@IsTest private class Q1Context67 {
 @IsTest static void capture() {
  String out='apex='+FeatureManagement.checkPermission('Probe_Bypass');
  FormulaEval.FormulaInstance formula=Formula.builder().withType(Account.class).withReturnType(FormulaEval.FormulaReturnType.BOOLEAN).withGlobalVariables(FormulaEval.FormulaGlobal.values()).withFormula('$Permission.Probe_Bypass').build();
  out+=';formula='+String.valueOf(formula.evaluate(new Account()));
  Flow.Interview interview=Flow.Interview.createInterview('Q1PermissionFlow',new Map<String,Object>());interview.start();
  out+=';flow='+String.valueOf(interview.getVariableValue('result'));
  System.assert(false,'Q1EXTRA|'+out);
 }
}
