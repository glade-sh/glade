public class DateWalker {
 private static Map<String,List<String>> paths {
  get { paths=paths ?? new Map<String,List<String>>{'OpportunityLineItem'=>new List<String>{'ServiceDate'}};return paths; }
  set;
 }
 public DateWalker(List<String> path) { paths.put('OpportunityLineItem',path); }
 public Datetime read(SObject item) {
  List<String> fields=new List<String>(paths.get(item.getSObjectType().toString()));
  while(fields.size()>1) { item=item.getSObject(fields.remove(0)); }
  return item.isSet(fields[0]) ? (Datetime)item.get(fields[0]) : null;
 }
}
