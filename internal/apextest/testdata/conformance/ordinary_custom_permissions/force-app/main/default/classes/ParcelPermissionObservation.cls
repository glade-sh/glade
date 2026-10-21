@IsTest private class ParcelPermissionObservation {
 static String facets(String label) {
  Schema.DescribeSObjectResult o=PermissionParcel__c.SObjectType.getDescribe();
  Schema.DescribeFieldResult f=PermissionParcel__c.Enabled__c.getDescribe();
  return label+':'+o.isAccessible()+','+o.isCreateable()+','+o.isUpdateable()+','+f.isAccessible()+','+f.isCreateable()+','+f.isUpdateable();
 }
 @IsTest static void observe() {
  List<String> rows=new List<String>{facets('running')};
  User running=[SELECT Id,ProfileId FROM User WHERE Id=:UserInfo.getUserId()];
  System.runAs(running) { rows.add(facets('running-id')); }
  User actor=new User(Alias='parcel',Email='parcel@example.invalid',EmailEncodingKey='UTF-8',LastName='Observer',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',Username='parcel-'+UserInfo.getOrganizationId()+'@example.invalid',ProfileId=[SELECT Id FROM Profile WHERE Name='Parcel Observer'].Id);
  insert actor;
  System.runAs(actor) { rows.add(facets('profile')); }
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='ParcelEmpty'].Id);
  System.runAs(actor) { rows.add(facets('empty')); }
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='ParcelObject'].Id);
  System.runAs(actor) { rows.add(facets('object')); }
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='ParcelFull'].Id);
  System.runAs(actor) { rows.add(facets('full')); }
  System.assert(false,'PERMISSION_ROWS|'+String.join(rows,';')+'|END');
 }

 static String removed(String label, SObject record, System.AccessType mode) {
  SObjectAccessDecision decision=Security.stripInaccessible(mode,new List<SObject>{record},false);
  Set<String> fields=decision.getRemovedFields().get('Account');
  return label+':'+record.getPopulatedFieldsAsMap().containsKey('Enabled__c')+','+(fields!=null && fields.contains('Enabled__c'))+','+(fields!=null && fields.contains('Name'));
 }
 @IsTest static void observeDefaults() {
  User actor=new User(Alias='parcel',Email='parcel@example.invalid',EmailEncodingKey='UTF-8',LastName='Observer',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',Username='default-'+UserInfo.getOrganizationId()+'@example.invalid',ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User'].Id);
  insert actor;
  List<String> rows=new List<String>();
  System.runAs(actor) {
   rows.add(removed('create-implicit',new Account(Name='Implicit'),System.AccessType.CREATABLE));
   rows.add(removed('create-false',new Account(Name='Explicit',Enabled__c=false),System.AccessType.CREATABLE));
   rows.add(removed('create-true',new Account(Name='Explicit',Enabled__c=true),System.AccessType.CREATABLE));
   rows.add(removed('update-implicit',new Account(Name='Implicit'),System.AccessType.UPDATABLE));
   rows.add(removed('update-false',new Account(Name='Explicit',Enabled__c=false),System.AccessType.UPDATABLE));
   rows.add(removed('update-true',new Account(Name='Explicit',Enabled__c=true),System.AccessType.UPDATABLE));
   Account stored=new Account(Name='Stored');
   insert stored;
   rows.add(removed('inserted-implicit',stored,System.AccessType.UPDATABLE));
   Account queried=[SELECT Id,Name,Enabled__c FROM Account WHERE Id=:stored.Id WITH SYSTEM_MODE];
   rows.add(removed('queried-false',queried,System.AccessType.UPDATABLE));
   rows.add(removed('cloned-implicit',new Account(Name='Clone').clone(false,false),System.AccessType.CREATABLE));
  }
  System.assert(false,'DEFAULT_ROWS|'+String.join(rows,';')+'|END');
 }
}
