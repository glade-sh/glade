@IsTest private class BoundarySliceTest {
 @IsTest static void key_view() {
  Map<String,Integer> records=new Map<String,Integer>{'first'=>1,'second'=>2};
  records.keySet().removeAll(new Set<String>{'first'});
  System.assert(false,'OBSERVED|key_view|'+JSON.serialize(new Map<String,Object>{'retained'=>records.containsKey('first'),'size'=>records.size()}));
 }
 @IsTest static void key_alias() {
  Map<String,Integer> records=new Map<String,Integer>{'first'=>1,'second'=>2};
  Set<String> keys=records.keySet(); addEntry(records);
  Integer loopCount=0; for(String key:keys) loopCount++;
  Integer constructedCount=new List<String>(keys).size();
  Boolean argumentOrder=JSON.serialize(keys,addFourth(records)).contains('fourth');
  Map<String,Object> envelope=new Map<String,Object>{'keys'=>keys};
  Boolean nestedOrder=JSON.serialize(envelope,addFifth(records)).contains('fifth');
  Boolean serializedThird=JSON.serialize(keys).contains('third');
  Boolean equalKeys=keys==new Set<String>{'first','second','third','fourth','fifth'};
  Set<String> copy=keys.clone(); keys.remove('first');
  
  System.assert(false,'OBSERVED|key_alias|'+JSON.serialize(new Map<String,Object>{'argumentOrder'=>argumentOrder,'nestedOrder'=>nestedOrder,'loopCount'=>loopCount,'constructedCount'=>constructedCount,'serializedThird'=>serializedThird,'equalKeys'=>equalKeys,'mapSize'=>records.size(),'keysSize'=>keys.size(),'copySize'=>copy.size(),'mapHasFirst'=>records.containsKey('first'),'keysHasThird'=>keys.contains('third'),'mapHasExtra'=>records.containsKey('extra')}));
 }
 private static Boolean addFourth(Map<String,Integer> records){records.put('fourth',4);return false;}
 private static Boolean addFifth(Map<String,Integer> records){records.put('fifth',5);return false;}
 private static void addEntry(Map<String,Integer> records) {records.put('third',3);}
 @IsTest static void custom_key_history() {
  ProbeKey first=new ProbeKey(1); ProbeKey second=new ProbeKey(3);
  Map<ProbeKey,String> records=new Map<ProbeKey,String>{first=>'one',second=>'three'};
  Set<ProbeKey> keys=records.keySet(); first.code=2;
  records.put(second,'updated');
  Boolean changedHashFound=keys.contains(first); Integer beforeClear=keys.size();
  records.clear(); Integer afterClear=keys.size();
  System.assert(false,'OBSERVED|custom_key_history|'+JSON.serialize(new Map<String,Object>{'changedHashFound'=>changedHashFound,'beforeClear'=>beforeClear,'afterClear'=>afterClear}));
 }
 private class ProbeKey {
  Integer code; ProbeKey(Integer value){code=value;}
  public override Integer hashCode(){return code;}
  public Boolean equals(Object other){return other instanceof ProbeKey && code==((ProbeKey)other).code;}
 }
 @IsTest static void key_consumers() {
  List<Account> rows=new List<Account>{new Account(Name='Synthetic Alpha'),new Account(Name='Synthetic Beta')};
  insert as system rows;
  Map<String,Integer> names=new Map<String,Integer>{'Synthetic Alpha'=>1};
  Set<String> keys=names.keySet(); names.put('Synthetic Beta',2);
  Integer both=[SELECT Count() FROM Account WHERE Name IN :keys WITH SYSTEM_MODE];
  Boolean hashMatches=System.hashCode(keys)==System.hashCode(new Set<String>{'Synthetic Alpha','Synthetic Beta'});
  names.remove('Synthetic Alpha');
  Integer one=[SELECT Count() FROM Account WHERE Name IN :keys WITH SYSTEM_MODE];
  System.assert(false,'OBSERVED|key_consumers|'+JSON.serialize(new Map<String,Object>{'both'=>both,'one'=>one,'hashMatches'=>hashMatches}));
 }
 @IsTest static void xml_timing() {
  Map<String,Object> observed=new Map<String,Object>{'inner'=>false,'outer'=>false};
  try {
   HttpResponse response=new HttpResponse(); response.setBody('Malformed text');
   XmlStreamReader reader=response.getXmlStreamReader();
   try { while(reader.hasNext()) reader.next(); } catch(Exception e){observed.put('inner',true);observed.put('type',e.getTypeName());}
  } catch(Exception e){observed.put('outer',true);observed.put('type',e.getTypeName());}
  System.assert(false,'OBSERVED|xml_timing|'+JSON.serialize(observed));
 }
 @IsTest static void notification_shape() {
  Map<String,Object> observed=new Map<String,Object>();
  for(String variant:new List<String>{'empty','badType','badTarget','badRecipients','nullRecipients','emptyRecipients','longTitle'}) {
   try {
    Messaging.CustomNotification notice=new Messaging.CustomNotification();
    Set<String> recipients=new Set<String>{UserInfo.getUserId()};
    if(variant!='empty') {notice.setTitle('Synthetic');notice.setBody('Synthetic');notice.setNotificationTypeId('0ML000000000001');notice.setTargetId(UserInfo.getUserId());}
    if(variant=='badType') notice.setNotificationTypeId('invalid-type');
    if(variant=='badTarget') notice.setTargetId('invalid-target');
    if(variant=='badRecipients') recipients=new Set<String>{'invalid-recipient'};
    if(variant=='nullRecipients') recipients=null;
    if(variant=='emptyRecipients') recipients=new Set<String>();
    if(variant=='longTitle') notice.setTitle('x'.repeat(251));
    notice.send(recipients); observed.put(variant,'returned');
   } catch(Exception e){observed.put(variant,e.getTypeName()+':'+e.getMessage());}
  }
  System.assert(false,'OBSERVED|notification_shape|'+JSON.serialize(observed));
 }
 @IsTest static void xml_constructor() {
  Map<String,Object> observed=new Map<String,Object>{'constructed'=>false,'advanced'=>false};
  try {
   XmlStreamReader reader=new XmlStreamReader('<root>'); observed.put('constructed',true);
   reader.next(); observed.put('advanced',true);
   while(reader.hasNext()) reader.next();
  } catch(Exception e){observed.put('type',e.getTypeName());}
  System.assert(false,'OBSERVED|xml_constructor|'+JSON.serialize(observed));
 }
 @IsTest static void invalid_target() {
  Map<String,Object> observed=new Map<String,Object>{'threw'=>false};
  try {
   Messaging.CustomNotification notice=new Messaging.CustomNotification(); notice.setTitle('Synthetic');notice.setBody('Synthetic');notice.setTargetId('invalid-target');notice.send(new Set<String>{UserInfo.getUserId()});
  } catch(Exception e){observed.put('threw',true);observed.put('message',e.getMessage());}
  System.assert(false,'OBSERVED|invalid_target|'+JSON.serialize(observed));
 }
}
