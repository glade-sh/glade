for (String expr : new List<String>{'IF(ISNULL($ObjectType.Account.Fields.Name), \'null\', \'set\')', 'Name'}) {
  try {
    FormulaEval.FormulaInstance ff = Formula.builder().withType(Account.class).withReturnType(FormulaEval.FormulaReturnType.STRING).withFormula(expr).build();
    System.debug(LoggingLevel.ERROR, 'ROW|F ' + expr + '|' + ff.evaluate(new Account(Name = 'LF F')));
  } catch (Exception e) {
    System.debug(LoggingLevel.ERROR, 'ROW|F ' + expr + '|EXC ' + e.getTypeName() + ': ' + e.getMessage());
  }
}
try { insert new LF_Probe__c(Name = 'LF VR'); System.debug(LoggingLevel.ERROR, 'ROW|V validation rule|inserted'); } catch (DmlException e) { System.debug(LoggingLevel.ERROR, 'ROW|V validation rule|' + e.getDmlMessage(0)); }
for (String r : LFActivityProbe.run()) System.debug(LoggingLevel.ERROR, 'ROW|' + r);
for (String r : LFActivityProbe2.run()) System.debug(LoggingLevel.ERROR, 'ROW|' + r);
