public class DefaultFormulaProbe {
 public static String inspect(Boolean load, Boolean doInsert) {
  DefaultProbe__c x = (DefaultProbe__c) DefaultProbe__c.SObjectType.newSObject(null, load);
  if (doInsert) { x.Name = 'probe'; insert as system x; x = [SELECT Moment__c, Offset__c, Minute__c, Tomorrow__c, Number__c, Blank__c FROM DefaultProbe__c WHERE Id = :x.Id WITH SYSTEM_MODE]; }
  DateTime nowValue = System.now();
  Long momentDelta = x.Moment__c == null ? null : x.Moment__c.getTime() - nowValue.getTime();
  Long offsetDelta = x.Offset__c == null ? null : x.Offset__c.getTime() - nowValue.addSeconds(Math.mod(nowValue.minuteGmt(),15)*60).getTime();
  return 'momentNear='+(momentDelta != null && Math.abs(momentDelta) < 3000)+'|offsetNear='+(offsetDelta != null && Math.abs(offsetDelta) < 3000)+'|minuteNow='+(x.Minute__c != null && x.Minute__c == nowValue.minuteGmt())+'|tomorrow='+(x.Tomorrow__c == Date.today().addDays(1))+'|number='+x.Number__c+'|blankNull='+(x.Blank__c == null);
 }
}