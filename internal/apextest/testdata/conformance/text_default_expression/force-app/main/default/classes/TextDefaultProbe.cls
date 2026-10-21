public class TextDefaultProbe {
 public static Map<String,Object> observe(){
  Map<String,Object> out = new Map<String,Object>();
  TokenSample__c row = new TokenSample__c(Name='fresh');
  out.put('fresh',row.Token__c == null);
  insert row;
  row = [SELECT Token__c,Pair__c,Literal__c,Function__c FROM TokenSample__c WHERE Id=:row.Id];
  String token=row.Token__c;
  out.put('generated',new Map<String,Object>{'length'=>token.length(),'prefix'=>token.startsWith('seed-'),'userSuffix'=>token.endsWith(UserInfo.getUserId().substring(5,10)),'dateShape'=>Pattern.matches('seed-[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}Z.*',token)});
  out.put('pair',row.Pair__c); out.put('literal',row.Literal__c); out.put('function',row.Function__c);
  TokenSample__c supplied = new TokenSample__c(Name='supplied',Token__c='manual'); insert supplied;
  out.put('supplied',[SELECT Token__c FROM TokenSample__c WHERE Id=:supplied.Id].Token__c);
  TokenSample__c blank = new TokenSample__c(Name='null',Token__c=null); insert blank;
  out.put('explicitNull',[SELECT Token__c FROM TokenSample__c WHERE Id=:blank.Id].Token__c);
  row.Name='changed'; update row;
  out.put('stable',token == [SELECT Token__c FROM TokenSample__c WHERE Id=:row.Id].Token__c);
  return out;
 }
}
