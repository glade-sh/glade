@IsTest private class ScheduledPathTest {
 @IsTest static void futurePath() {
  Test.startTest();
  TimedParcel__c parcel=new TimedParcel__c(Name='Scheduled parcel',NextRun__c=System.now().addDays(1));
  Database.insert(parcel,AccessLevel.SYSTEM_MODE);
  Boolean beforeStop=[SELECT Marker__c FROM TimedParcel__c WHERE Id=:parcel.Id WITH SYSTEM_MODE].Marker__c;
  Test.stopTest();
  Boolean afterStop=[SELECT Marker__c FROM TimedParcel__c WHERE Id=:parcel.Id WITH SYSTEM_MODE].Marker__c;
  System.assert(false,'OBSERVED|futurePath|'+JSON.serialize(new Map<String,Object>{'before'=>beforeStop,'after'=>afterStop}));
 }
 @IsTest static void immediateAndFuturePath() {
  Test.startTest();
  TimedParcel__c parcel=new TimedParcel__c(Name='Combined parcel',NextRun__c=System.now().addDays(1));
  Database.insert(parcel,AccessLevel.SYSTEM_MODE);
  TimedParcel__c beforeStop=[SELECT Immediate__c,Marker__c FROM TimedParcel__c WHERE Id=:parcel.Id WITH SYSTEM_MODE];
  Test.stopTest();
  TimedParcel__c afterStop=[SELECT Immediate__c,Marker__c FROM TimedParcel__c WHERE Id=:parcel.Id WITH SYSTEM_MODE];
  System.assert(false,'OBSERVED|immediateAndFuturePath|'+JSON.serialize(new Map<String,Object>{'beforeImmediate'=>beforeStop.Immediate__c,'beforeScheduled'=>beforeStop.Marker__c,'afterImmediate'=>afterStop.Immediate__c,'afterScheduled'=>afterStop.Marker__c}));
 }
}
