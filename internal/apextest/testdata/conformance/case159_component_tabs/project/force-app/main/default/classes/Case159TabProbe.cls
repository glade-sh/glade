public class Case159TabProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> result=new Map<String,Object>();
  for(Schema.DescribeTabSetResult app:Schema.describeTabs()) for(Schema.DescribeTabResult tab:app.getTabs()) {
   if(new Set<String>{'RoadLwc','RoadAura','RoadLwcTwo','RoadAuraTwo','RoadPage'}.contains(tab.getName())) { String id=tab.getSObjectName(); result.put(tab.getName(),new Map<String,Object>{'custom'=>tab.isCustom(),'prefix'=>id==null?null:id.substring(0,3),'length'=>id==null?null:id.length(),'urlIsNull'=>tab.getUrl()==null}); }
  }
  return result;
 }
}
