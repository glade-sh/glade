public class MeadowFieldTokenProbe {
 public static Map<String,Boolean> observe() {
  Map<String,Boolean> values = new Map<String,Boolean>();
  values.put('FA01', String.valueOf(Contact.Field.Email) == 'Email');
  values.put('FA02', String.valueOf(FieldAliasEntry__c.Field.ExternalKey__c) == 'ExternalKey__c');
  values.put('FA03', standardUpsert('field'));
  values.put('FA04', customUpsert('field'));
  values.put('FC01', standardUpsert('fields'));
  values.put('FC02', customUpsert('fields'));
  values.put('FC03', standardUpsert('direct'));
  values.put('FC04', customUpsert('direct'));
  return values;
 }
 private static Boolean standardUpsert(String mode) {
  Schema.SObjectField key;
  if (mode == 'field') key = Contact.Field.Email;
  else if (mode == 'fields') key = Contact.Fields.Email;
  else key = Contact.Email;
  String email = mode + '-field-token@example.invalid';
  Contact first = new Contact(LastName='First', Email=email);
  Database.UpsertResult created = Database.upsert(first, key, true);
  Contact second = new Contact(LastName='Second', Email=email);
  Database.UpsertResult updated = Database.upsert(second, key, true);
  Contact stored = [SELECT Id, LastName FROM Contact WHERE Email=:email LIMIT 1];
  return created.isSuccess() && updated.isSuccess() && first.Id == second.Id && stored.Id == first.Id && stored.LastName == 'Second';
 }
 private static Boolean customUpsert(String mode) {
  Schema.SObjectField key;
  if (mode == 'field') key = FieldAliasEntry__c.Field.ExternalKey__c;
  else if (mode == 'fields') key = FieldAliasEntry__c.Fields.ExternalKey__c;
  else key = FieldAliasEntry__c.ExternalKey__c;
  FieldAliasEntry__c first = new FieldAliasEntry__c(Name='First', ExternalKey__c=mode);
  Database.UpsertResult created = Database.upsert(first, key, true);
  FieldAliasEntry__c second = new FieldAliasEntry__c(Name='Second', ExternalKey__c=mode);
  Database.UpsertResult updated = Database.upsert(second, key, true);
  FieldAliasEntry__c stored = [SELECT Id, Name FROM FieldAliasEntry__c WHERE ExternalKey__c=:mode LIMIT 1];
  return created.isSuccess() && updated.isSuccess() && first.Id == second.Id && stored.Id == first.Id && stored.Name == 'Second';
 }
}
