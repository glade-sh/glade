public class SubscriberNoRowsProbe {
 public static String sample() {
  Map<String,Object> out=new Map<String,Object>();
  String topic='SyntheticEvent__e';
  List<EventBusSubscriber> existing=[SELECT Id,Name,Topic,Type,Status,Position,LastError FROM EventBusSubscriber WHERE Topic=:topic AND Type='ApexTrigger'];
  List<EventBusSubscriber> missing=[SELECT Id FROM EventBusSubscriber WHERE Topic='/event/SyntheticAbsent__e'];
  List<EventBusSubscriber> channel=[SELECT Id FROM EventBusSubscriber WHERE Topic='/event/SyntheticEvent__e'];
  List<EventBusSubscriber> unsubscribed=[SELECT Id FROM EventBusSubscriber WHERE Topic='UnsubscribedEvent__e'];
  List<EventBusSubscriber> disabled=[SELECT Id FROM EventBusSubscriber WHERE Topic='DisabledEvent__e'];
  List<EventBusSubscriber> second=[SELECT Id,Name FROM EventBusSubscriber WHERE Topic='SecondEvent__e' AND Type='ApexTrigger'];
  List<EventBusSubscriber> otherType=[SELECT Id FROM EventBusSubscriber WHERE Topic=:topic AND Type='Flow'];
  out.put('existing',existing.size()); out.put('missing',missing.size()); out.put('channel',channel.size()); out.put('unsubscribed',unsubscribed.size()); out.put('disabled',disabled.size()); out.put('otherType',otherType.size()); out.put('second',second.size());
  if(!second.isEmpty())out.put('secondName',second[0].Name);
  if(!existing.isEmpty()){EventBusSubscriber row=existing[0];String rowId=String.valueOf(row.Id);out.put('name',row.Name);out.put('type',row.Type);out.put('status',row.Status);out.put('idStarts000',rowId.startsWith('000'));out.put('idAllZeroPrefix',rowId.startsWith('000000000000000'));out.put('idExact',rowId=='000000000000000AAA');out.put('idLength',rowId.length());if(!second.isEmpty())out.put('idSame',row.Id==second[0].Id);out.put('positionPresent',row.Position!=null);out.put('positionPositive',row.Position>0);out.put('positionZero',row.Position==0);out.put('lastErrorNull',row.LastError==null);}
  if(!existing.isEmpty()){try{Id id=existing[0].Id;List<EventBusSubscriber> matches=[SELECT Id FROM EventBusSubscriber WHERE Id=:id];out.put('idQuery',matches.size());}catch(Exception e){out.put('idQuery',e.getTypeName());}}
  try{Database.QueryLocatorIterator it=Database.getQueryLocator('SELECT Id FROM EventBusSubscriber WHERE Topic = \'SyntheticEvent__e\'').iterator();out.put('locator',it.hasNext());if(it.hasNext()){EventBusSubscriber row=(EventBusSubscriber)it.next();out.put('locatorIdExact',String.valueOf(row.Id)=='000000000000000AAA');}}catch(Exception e){out.put('locator',e.getTypeName());}
  try { EventBusSubscriber r=[SELECT Id FROM EventBusSubscriber WHERE Topic='/event/SyntheticAbsent__e']; out.put('singleton','found'); }
  catch(QueryException e) { out.put('singleton',e.getTypeName()); }
  return JSON.serialize(out);
 }
}
