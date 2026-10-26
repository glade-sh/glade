public class DatedValueProbe {
 public static Map<String,Object> observe() {
  Date early=Date.newInstance(2024,2,3), later=Date.newInstance(2024,3,4);
  Opportunity parent=new Opportunity(Name='Window',CloseDate=early,StageName='Prospecting');
  OpportunityLineItem line=new OpportunityLineItem(Opportunity=parent,ServiceDate=later,Quantity=1,TotalPrice=7);
  SObject copied=(SObject)JSON.deserialize(JSON.serialize(line),SObject.class);
  List<String> path=new List<String>{'Opportunity','CloseDate'};
  DateWalker walker=new DateWalker(path);
  RateWindow window=new RateWindow(Date.newInstance(2024,2,1),Date.newInstance(2024,3,1),1.25);
  String key=copied.Id+((Object)copied).hashCode().format();
  Integer initialHash=copied.hashCode();
  Datetime selected=walker.read(copied);
  SObject transformed=copied.clone(true,true,true,true);
  Map<String,Object> payload=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(transformed));
  payload.put('TotalPrice',window.convert(selected,7));
  transformed=(SObject)JSON.deserialize(JSON.serialize(payload),SObject.class);
  Map<String,SObject> cache=new Map<String,SObject>{key=>transformed};
  SObject found=cache.get(copied.Id+((Object)copied).hashCode().format()) ?? copied;
  return new Map<String,Object>{'parentPresent'=>copied.getSObject('Opportunity')!=null,
   'closeDateSet'=>copied.getSObject('Opportunity').isSet('CloseDate'),
   'earlyWindow'=>window.includes(selected),'converted'=>window.convert(selected,7),
   'cachedAmount'=>found.get('TotalPrice'),'hashStable'=>initialHash==copied.hashCode(),
   'pathRetained'=>path.size(),'readAgain'=>walker.read(copied)==selected};
 }
}
