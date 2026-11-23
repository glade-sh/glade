public class DefaultFormulaProbe {
 public static String inspect(String mode) {
  Id rt = DefaultProbe__c.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('RT_B').getRecordTypeId();
  DefaultProbe__c x;
  if (mode == 'rtLoaded') x = (DefaultProbe__c) DefaultProbe__c.SObjectType.newSObject(rt, true);
  else if (mode == 'rtUnloaded') x = (DefaultProbe__c) DefaultProbe__c.SObjectType.newSObject(rt, false);
  else if (mode == 'nullLoaded') x = (DefaultProbe__c) DefaultProbe__c.SObjectType.newSObject(null, true);
  else if (mode == 'rtInsert') { x = new DefaultProbe__c(RecordTypeId = rt, Name = 'probe'); }
  else { x = new DefaultProbe__c(Name = 'probe'); }
  if (mode == 'rtInsert' || mode == 'plainInsert') { insert as system x; x = [SELECT Moment__c, Offset__c, Minute__c, Tomorrow__c, Number__c, Blank__c, Corp1__c, Corp2__c, Back__c, DateV__c, Floor__c FROM DefaultProbe__c WHERE Id = :x.Id WITH SYSTEM_MODE]; }
  DateTime nowValue = System.now();
  Long momentDelta = x.Moment__c == null ? null : x.Moment__c.getTime() - nowValue.getTime();
  Long offsetDelta = x.Offset__c == null ? null : x.Offset__c.getTime() - nowValue.addSeconds(Math.mod(nowValue.minuteGmt(),15)*60).getTime();
  Integer minToNext = 30 - Math.mod(nowValue.minuteGmt(),30);
  Boolean corp1 = x.Corp1__c != null && Math.abs(x.Corp1__c.getTime() - nowValue.addSeconds(minToNext*60).getTime()) < 3000;
  Boolean corp2 = x.Corp2__c != null && Math.abs(x.Corp2__c.getTime() - nowValue.addSeconds(minToNext*60).getTime()) < 3000;
  Boolean back = x.Back__c != null && Math.abs(x.Back__c.getTime() - nowValue.addSeconds(-3600).getTime()) < 3000;
  Boolean dateV = x.DateV__c == nowValue.dateGmt().addDays(1);
  Boolean floorOk = x.Floor__c != null && x.Floor__c == (nowValue.minuteGmt()/15)*15;
  return 'corp1='+corp1+'|corp2='+corp2+'|back='+back+'|dateV='+dateV+'|floor='+floorOk+'|momentNear='+(momentDelta != null && Math.abs(momentDelta) < 3000)+'|offsetNear='+(offsetDelta != null && Math.abs(offsetDelta) < 3000)+'|minuteNow='+(x.Minute__c != null && x.Minute__c == nowValue.minuteGmt())+'|tomorrow='+(x.Tomorrow__c == Date.today().addDays(1))+'|number='+x.Number__c+'|blankNull='+(x.Blank__c == null);
 }
}
