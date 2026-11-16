public class OlProbe66 {
  public static String run(){
    List<String> o=new List<String>();
    Map<String,System.OrgLimit> m=System.OrgLimits.getMap();
    o.add('OL1 elastic='+m.containsKey('DailyAsyncApexElasticExecutions'));
    o.add('OL2 processed='+m.containsKey('DailyAsyncApexProcessed'));
    o.add('OL3 asyncExec='+m.containsKey('DailyAsyncApexExecutions')+' apiReq='+m.containsKey('DailyApiRequests')+' single='+m.containsKey('SingleEmail'));
    o.add('OL4 sizeAtLeast19='+(m.size()>=19));
    return String.join(o,' ## ');
  }
}
