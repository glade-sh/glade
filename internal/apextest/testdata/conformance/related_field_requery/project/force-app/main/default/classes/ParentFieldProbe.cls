public class ParentFieldProbe {
 public static Map<String,Object> observe() {
  Map<String,Object> answer = new Map<String,Object>();
  Account parent = new Account(Name='North', AnnualRevenue=8);
  insert parent;
  Contact child = new Contact(LastName='Reader', AccountId=parent.Id);
  insert child;
  Contact partial = [SELECT Id, LastName, Account.Id FROM Contact WHERE Id=:child.Id];
  Contact expanded = [SELECT Id, Account.Name, Account.Owner.Id FROM Contact WHERE Id=:child.Id ORDER BY Account.Name];
  answer.put('partialHasAccount', partial.getPopulatedFieldsAsMap().containsKey('Account'));
  answer.put('partialHasName', FieldWalker.present(partial, new List<String>{'Account','Name'}));
  answer.put('expandedHasName', FieldWalker.present(expanded, new List<String>{'Account','Name'}));
  answer.put('expandedName', expanded.Account.Name);
  answer.put('accountReference',Contact.AccountId.getDescribe().getReferenceTo().isEmpty()==false);
  answer.put('accountPolymorphic',Contact.AccountId.getDescribe().isNamePointing());
  try { partial.Account.put('AccountId', parent.Id); answer.put('invalidParentWrite','accepted'); }
  catch(SObjectException e) { answer.put('invalidParentWrite',e.getTypeName()); }
  List<Contact> items = new List<Contact>{partial};
  RelationshipRefresh.apply(items, new Map<Id,SObject>{child.Id=>expanded});
  Contact refreshed = (Contact)items[0];
  answer.put('refreshedHasAccount',refreshed.getPopulatedFieldsAsMap().containsKey('Account'));
  try { answer.put('refreshedName',refreshed.Account.Name); }
  catch(SObjectException e) { answer.put('refreshedName',e.getTypeName()); }
  answer.put('refreshedHasOwner',FieldWalker.present(refreshed,new List<String>{'Account','Owner','Id'}));
  return answer;
 }
}
