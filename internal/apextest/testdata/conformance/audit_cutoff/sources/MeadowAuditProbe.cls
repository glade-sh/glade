public class MeadowAuditProbe {
 public static Map<String,Object> observe() {
  Savepoint boundary;
  if(!Test.isRunningTest()) boundary=Database.setSavepoint();
  try {
   insert new Account(Name='Meadow One');
   insert new Account(Name='Meadow Two');
   insert new Account(Name='Meadow Three');
   Datetime begins=Datetime.now().addHours(1);
   List<Event> meetings=new List<Event>{new Event(Subject='Morning',ShowAs='Busy',StartDateTime=begins,EndDateTime=begins.addHours(1)),new Event(Subject='Free',ShowAs='Free',StartDateTime=begins,EndDateTime=begins.addHours(1)),new Event(Subject='Afternoon',ShowAs='Busy',StartDateTime=begins,EndDateTime=begins.addHours(1))};
   insert meetings;
   Set<Id> ids=new Map<Id,Event>(meetings).keySet();
   if(Test.isRunningTest()) Test.startTest();
   Datetime cutoff=Datetime.now();
   Datetime earlier=cutoff.addHours(-1);
   List<Event> stored=[SELECT Id,LastModifiedDate FROM Event WHERE Id IN :ids];
   Boolean noFuture=true;
   for(Event meeting:stored) noFuture=noFuture && meeting.LastModifiedDate<=cutoff;
   Map<String,Object> rows=new Map<String,Object>();
   rows.put('AT01',noFuture);
   rows.put('AT02',[SELECT COUNT() FROM Event WHERE Id IN :ids AND LastModifiedDate > :cutoff AND ShowAs != 'Free']==0);
   rows.put('AT03',[SELECT COUNT() FROM Event WHERE Id IN :ids AND LastModifiedDate > :earlier AND ShowAs != 'Free']==2);
   rows.put('AC01',stored.size()==3 && stored[0].LastModifiedDate==stored[1].LastModifiedDate && stored[1].LastModifiedDate==stored[2].LastModifiedDate);
   if(Test.isRunningTest()) Test.stopTest();
   return rows;
  } finally { if(boundary!=null) Database.rollback(boundary); }
 }
}
