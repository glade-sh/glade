public class R6MultiProbe {
 public static String observe(String code){try{insert new Opportunity(Name='R6Multi',Description=code,StageName='Prospecting',CloseDate=Date.today());return 'PASS';}catch(Exception e){return e.getMessage().contains('R6MATCH')?'MATCH':'OTHER:'+e.getMessage();}}
}
