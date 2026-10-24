public class ParentDiscoveryProbe {
 public static Map<String,Object> observe() {
  Account company=new Account(Name='Discovery');insert company;
  Contact child=new Contact(LastName='Reader',AccountId=company.Id);insert child;
  Contact partial=[SELECT Id,LastName,Account.Id FROM Contact WHERE Id=:child.Id];
  ParentHydrator hydrate=new ParentHydrator();
  List<SObject> expanded=hydrate.expand(new List<Contact>{partial},new List<String>{'Account.Name','Account.Owner.Id'});
  Contact full=(Contact)expanded[0];
  Map<String,Object> result=new Map<String,Object>{'fields'=>hydrate.fieldCount(),'hasAccount'=>full.getPopulatedFieldsAsMap().containsKey('Account')};
  try {result.put('name',full.Account.Name);result.put('hasOwner',full.Account.Owner.Id!=null);} catch(SObjectException e) {result.put('error',e.getTypeName());}
  ParentHydrator namesOnly=new ParentHydrator();
  List<SObject> nameRows=namesOnly.expand(new List<Contact>{partial},new List<String>{'Account.Name'});
  Contact named=(Contact)nameRows[0];
  result.put('nameOnlyFields',namesOnly.fieldCount());
  result.put('nameOnlyPresent',named.getPopulatedFieldsAsMap().containsKey('Account'));
  try {result.put('nameOnly',named.Account.Name);} catch(SObjectException e) {result.put('nameOnlyError',e.getTypeName());}
  return result;
 }
}
