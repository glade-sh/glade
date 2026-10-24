public class BirchEntryProbe {
 public static Map<String,Object> observe(String phase, Boolean changed) {
  String tag = 'Birch'+(phase=='before'?'Before':'After')+(changed?'True':'False');
  Account row = new Account(Name=tag,Phone='Draft');
  insert row;
  Map<String,Object> result = new Map<String,Object>{'created'=>effect(row.Id,phase)};
  if (changed) {
   row.Phone='Ready'; update row;
   result.put('updated',effect(row.Id,phase));
  }
  return result;
 }
 private static Integer effect(Id id, String phase) {
  if (phase=='before') return [SELECT Description FROM Account WHERE Id=:id].Description=='matched'?1:0;
  return [SELECT COUNT() FROM Contact WHERE AccountId=:id];
 }
 public static void emit(String phase, Boolean changed) {
  System.assert(false,'BIRCH|'+JSON.serialize(observe(phase,changed)));
 }
 public static void missingPrior(String phase) {
  Map<String,Object> result = new Map<String,Object>();
  try {
   Flow.Interview interview = Flow.Interview.createInterview('Birch'+(phase=='before'?'Before':'After')+'True',new Map<String,Object>{'$Record'=>new Account(Name='Birch',Phone='Ready')});
   interview.start();
   result.put('rejected',false);
  } catch (Exception issue) {
   result.put('rejected',true);
   result.put('exceptionType',issue.getTypeName());
   result.put('exceptionMessage',issue.getMessage());
  }
  System.assert(false,'BIRCH|'+JSON.serialize(result));
 }
}
