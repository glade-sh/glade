Map<String,Object> result = new Map<String,Object>();
Map<String,Schema.SObjectType> described = Schema.getGlobalDescribe();
for (String name : new List<String>{'AccountContactRelation','AccountTeamMember','OpportunityTeamMember','OpportunitySplit','Order','OrderItem','Entitlement','Idea','CampaignInfluence','Territory2','Territory2Model','Territory2Type','UserTerritory2Association','ObjectTerritory2Association','ForecastingItem','ForecastingQuota','ForecastingType','Knowledge__kav','Knowledge__ka','CurrencyType','DatedConversionRate','Quote','QuoteLineItem','Network','NetworkMember','Account','AbsentObject__c'}) {
 result.put(name+'.describe', described.containsKey(name));
 result.put(name+'.type', Type.forName('Schema.'+name) != null);
 try { Database.countQuery('SELECT COUNT() FROM '+name); result.put(name+'.query', true); }
 catch (Exception e) { result.put(name+'.query', e.getTypeName()); }
 if (described.containsKey(name)) {
  try { insert described.get(name).newSObject(); result.put(name+'.dml', 'inserted'); }
  catch (DmlException e) { result.put(name+'.dml', 'DmlException:'+e.getDmlStatusCode(0)); }
  catch (Exception e) { result.put(name+'.dml', e.getTypeName()); }
 } else { result.put(name+'.dml', 'absent'); }
}
System.debug('OBSERVED|capture|'+JSON.serialize(result));
