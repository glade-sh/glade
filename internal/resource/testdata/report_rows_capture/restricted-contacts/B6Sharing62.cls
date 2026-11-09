@IsTest(SeeAllData=true) private with sharing class B6Sharing62 { @IsTest static void capture() {
 User u=[SELECT Id FROM User WHERE Alias='b6usr' LIMIT 1];
 Map<String,Id> ids=new Map<String,Id>();for(Report r:[SELECT Id,DeveloperName FROM Report WHERE DeveloperName IN ('B6_Accounts','B6_NoScope','B6_Contacts')]) ids.put(r.DeveloperName,r.Id);
 String outcome='';System.runAs(u) {
 Integer n=[SELECT COUNT() FROM Account]; outcome+='visible='+n+';contacts='+[SELECT COUNT() FROM Contact];
 for(String key:ids.keySet()) try { Reports.ReportResults x=Reports.ReportManager.runReport(ids.get(key),false);outcome+=';'+key+'='+x.getFactMap().get('T!T').getAggregates()[0].getValue(); } catch(Exception e) {outcome+=';'+key+'='+e.getTypeName()+':'+e.getMessage();}
 } System.assert(false,outcome);
} }