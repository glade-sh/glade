public with sharing class InventoryHostV67 {
 @AuraEnabled public static String observe() {
  List<Object> profiles=new List<Object>(); for(Profile p:[SELECT Name,UserLicense.Name FROM Profile ORDER BY Name]) profiles.add(new Map<String,Object>{'name'=>p.Name,'license'=>p.UserLicense.Name});
  List<Object> addresses=new List<Object>(); for(EmailServicesAddress a:[SELECT DeveloperName,LocalPart,IsActive,EmailDomainName,Function.FunctionName FROM EmailServicesAddress ORDER BY DeveloperName]) addresses.add(new Map<String,Object>{'name'=>a.DeveloperName,'local'=>a.LocalPart,'active'=>a.IsActive,'function'=>a.Function.FunctionName,'sandboxSuffix'=>a.EmailDomainName.endsWith('.apex.sandbox.salesforce.com')});
  return JSON.serialize(new Map<String,Object>{'profiles'=>profiles,'addresses'=>addresses});
 }
 public String getOutcome() {return observe();}
}