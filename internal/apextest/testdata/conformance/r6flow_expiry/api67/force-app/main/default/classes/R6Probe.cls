public class R6Probe {
 public static String observe(String name) {
  try { insert new Opportunity(Name=name, StageName='Prospecting', CloseDate=Date.today()); return 'PASS'; }
  catch (Exception e) { return e.getMessage().contains('R6MATCH') ? 'MATCH' : 'OTHER:'+e.getMessage(); }
 }
}