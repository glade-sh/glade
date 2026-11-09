public class Case159TabEquality {
 public static Map<String,Object> observe() {
  Map<String,String> values=new Map<String,String>();
  for(Schema.DescribeTabSetResult app:Schema.describeTabs()) for(Schema.DescribeTabResult tab:app.getTabs()) if(new Set<String>{'RoadLwc','RoadLwcAgain','RoadLwcTwo','RoadAura','RoadAuraTwo'}.contains(tab.getName())) values.put(tab.getName(),tab.getSObjectName());
  return new Map<String,Object>{'count'=>values.size(),'sameTarget'=>values.get('RoadLwc')==values.get('RoadLwcAgain'),'distinctLwc'=>values.get('RoadLwc')!=values.get('RoadLwcTwo'),'distinctAura'=>values.get('RoadAura')!=values.get('RoadAuraTwo')};
 }
}
