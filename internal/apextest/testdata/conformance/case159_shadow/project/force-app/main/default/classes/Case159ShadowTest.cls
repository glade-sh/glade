@IsTest private class Case159ShadowTest {
 @IsTest static void recordsAndLists() {
  insert new Schema.Account(Name='Synthetic');
  insert new Schema.Contact(LastName='Synthetic');
  List<Schema.Contact> contacts=(List<Schema.Contact>)Database.query('SELECT Id FROM Contact');
  List<Schema.Account> accounts=(List<Schema.Account>)Database.query('SELECT Id FROM Account');
  System.assertEquals(1,contacts.size()); System.assertEquals(1,accounts.size());
  System.assertEquals('Contact',String.valueOf(contacts.getSObjectType()));
  System.assertEquals('Account',String.valueOf(accounts.getSObjectType()));
  System.assert(((Schema.Contact)Schema.Contact.SObjectType.newSObject()) != null);
  System.assert(((Schema.Account)Schema.Account.SObjectType.newSObject()) != null);
  Contact localContact=new Contact(); Account localAccount=new Account();
  System.assertEquals('local',localContact.label); System.assertEquals('local',localAccount.label);
  Boolean rejected=false; try { Object x=localContact; Schema.Contact c=(Schema.Contact)x; } catch(TypeException e) { rejected=true; } System.assert(rejected);
  rejected=false; try { Object x=localAccount; Schema.Account a=(Schema.Account)x; } catch(TypeException e) { rejected=true; } System.assert(rejected);
 }
}
