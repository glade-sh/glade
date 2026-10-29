public class QueryChoiceProbe {
 private static Account choose(Boolean empty) { return empty ? new Account(Name='empty') : [SELECT Id,Name FROM Account WHERE Name='observed' LIMIT 1]; }
 private static Account reverse(Boolean stored) { return stored ? [SELECT Id,Name FROM Account WHERE Name='observed' LIMIT 1] : new Account(Name='empty'); }
 public static Map<String,Object> observe() {
  insert new Account(Name='observed');
  Account local=false ? [SELECT Id,Name FROM Account LIMIT 1] : new Account(Name='local');
  Account nullLeft=true ? null : [SELECT Id FROM Account LIMIT 1];
  Account nullRight=false ? [SELECT Id FROM Account LIMIT 1] : null;
  List<Account> listControl=false ? null : [SELECT Id FROM Account LIMIT 1];
  return new Map<String,Object>{'nullLeft'=>nullLeft==null,'nullRight'=>nullRight==null,'listControl'=>listControl.size(),'left'=>choose(true).Name,'right'=>choose(false).Name,'reverse'=>reverse(true).Name,'local'=>local.Name};
 }
}
