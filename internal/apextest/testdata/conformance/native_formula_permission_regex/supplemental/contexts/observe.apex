String out = 'apex='+FeatureManagement.checkPermission('Probe_Bypass');
try { FormulaEval.FormulaInstance f=Formula.builder().withType(Account.class).withReturnType(FormulaEval.FormulaReturnType.BOOLEAN).withGlobalVariables(FormulaEval.FormulaGlobal.values()).withFormula('$Permission.Probe_Bypass').build(); out+=';formula='+String.valueOf(f.evaluate(new Account())); } catch (Exception e) { out+=';formulaError='+e.getTypeName()+':'+e.getMessage(); }
try { Flow.Interview f=Flow.Interview.createInterview('Q1PermissionFlow',new Map<String,Object>()); f.start(); out+=';flow='+String.valueOf(f.getVariableValue('result')); } catch (Exception e) { out+=';flowError='+e.getTypeName()+':'+e.getMessage(); }
System.assert(false,'Q1EXTRA|'+out);
