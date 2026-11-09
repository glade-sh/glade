public class R6MetadataProbe {
 public static String observe(String code) {
  try { insert new Opportunity(Name='R6Metadata', Description=code, StageName='Prospecting', CloseDate=Date.today()); return 'PASS'; }
  catch (Exception e) { return e.getMessage().contains('R6MATCH') ? 'MATCH' : 'OTHER:'+e.getMessage(); }
 }
}
