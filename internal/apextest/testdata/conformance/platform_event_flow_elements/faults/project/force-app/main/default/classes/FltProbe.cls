public class FltProbe {
 public static SObject makeEvt(String key,String op) { return new FltImm__e(Key__c=key,Operation__c=op); }
 public static String clean(String text) {
  if(text==null) return null;
  text=text.replaceAll('Error ID: [0-9\\-]+ \\([0-9]+\\)','Error ID: N');
  Matcher m=Pattern.compile('\\b[A-Za-z0-9]{15}(?:[A-Za-z0-9]{3})?\\b').matcher(text);
  String out='';Integer last=0;
  while(m.find()) {
   String token=m.group();
   if(Pattern.compile('[0-9]').matcher(token).find()) { out+=text.substring(last,m.start())+'ID';last=m.end(); }
  }
  return out+text.substring(last);
 }
 public static List<String> send(String kind,String key,String op) {
  List<String> out=new List<String>();
  if(kind=='event') {
   out.add(String.valueOf(EventBus.publish(makeEvt(key,op)).isSuccess()));
  } else if(kind=='rec') {
   try { insert new FltRec__c(Name=key,Operation__c=op);out.add('ok'); }
   catch(Exception e) { out.add(e.getTypeName()+': '+clean(e.getMessage())); }
  } else {
   try { Flow.Interview.FltAuto f=new Flow.Interview.FltAuto(new Map<String,Object>{'key'=>key,'op'=>op});f.start();out.add('ok'); }
   catch(Exception e) { out.add(e.getTypeName()+': '+clean(e.getMessage())); }
  }
  return out;
 }
 public static Map<String,Object> read(String key) {
  List<String> entries=new List<String>();String pattern=key+'%';
  List<String> faults=new List<String>();
  for(FltLog__c row:[SELECT Name,Phase__c,Detail__c FROM FltLog__c WHERE Name LIKE :pattern]) {
   entries.add(row.Name.substring(key.length())+'|'+row.Phase__c);
   if(row.Phase__c=='fault') faults.add(clean(row.Detail__c));
  }
  entries.sort();faults.sort();
  return new Map<String,Object>{'entries'=>entries,'faults'=>faults};
 }
 public static void observe(String id,String kind,String op,String route) {
  String key='Flt-'+id+'-'+route;
  List<String> published=send(kind,key,op);
  Test.getEventBus().deliver();
  if(route=='deliver2') Test.getEventBus().deliver();
  Map<String,Object> out=read(key);out.put('published',published);
  System.assert(false,'EVT_OBS|'+id+'|'+route+'|'+JSON.serialize(out));
 }
}
