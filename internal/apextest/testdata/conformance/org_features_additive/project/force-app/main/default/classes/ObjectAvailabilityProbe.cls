public class ObjectAvailabilityProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> result = new Map<String,Object>();
  Map<String,Schema.SObjectType> described = Schema.getGlobalDescribe();
  for (String name : new List<String>{'CurrencyType','DatedConversionRate','AccountContactRelation','Network','NetworkMember','Knowledge__kav','Knowledge__ka','Quote','QuoteLineItem','Territory2','Territory2Model','Territory2Type','UserTerritory2Association','ObjectTerritory2Association','ForecastingItem','ForecastingQuota','ForecastingType','Survey','OpportunitySplit','OpportunityTeamMember','AccountTeamMember','WebStore','CampaignInfluence','ServiceChannel','Account','AbsentObject__c'}) {
   result.put(name+'.describe', described.containsKey(name));
   result.put(name+'.type', Type.forName('Schema.'+name) != null);
   try { Database.countQuery('SELECT COUNT() FROM '+name); result.put(name+'.query', true); }
   catch (Exception e) { result.put(name+'.query', e.getTypeName()); }
  }
  return result;
 }
}
