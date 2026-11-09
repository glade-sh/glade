@IsTest private class PslaRepeatTest {
 @TestSetup static void setup() {
  Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  insert new PermissionSet(Name='SyntheticAccess', Label='Synthetic Access');
  Id platformLicense=[SELECT Id FROM UserLicense WHERE Name='Salesforce Platform' AND Status='Active' LIMIT 1].Id;
  insert new PermissionSet(Name='SyntheticMismatch', Label='Synthetic Mismatch', LicenseId=platformLicense);
  insert new User(Alias='pslarec',Email='synthetic@example.invalid',EmailEncodingKey='UTF-8',LastName='Synthetic',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='GMT',Username='psla-recapture-'+System.now().getTime()+'@example.invalid');
 }
 static User actor(){return [SELECT Id FROM User WHERE Alias='pslarec' LIMIT 1];}
 static Id licenseId(){return [SELECT Id FROM PermissionSetLicense WHERE DeveloperName='IdentityConnect' LIMIT 1].Id;}
 @IsTest static void accountThenLicense() {
  User u=actor(); Id license=licenseId(); {
   insert new Account(Name='Synthetic');
   try { insert new PermissionSetLicenseAssign(AssigneeId=u.Id,PermissionSetLicenseId=license); System.assert(false,'expected mixed DML'); }
   catch(DmlException e) { System.assert(e.getMessage().contains('MIXED_DML_OPERATION'),e.getMessage()); System.assert(e.getMessage().contains('PermissionSetLicenseAssign, original object: Account'),e.getMessage()); }
  }
 }
 @IsTest static void licenseThenAccount() {
  User u=actor(); Id license=licenseId(); {
   insert new PermissionSetLicenseAssign(AssigneeId=u.Id,PermissionSetLicenseId=license);
   try {insert new Account(Name='Synthetic');System.assert(false,'expected mixed DML');}
   catch(DmlException e){System.assert(e.getMessage().contains('MIXED_DML_OPERATION'),e.getMessage());System.assert(e.getMessage().contains('Account, original object: PermissionSetLicenseAssign'),e.getMessage());}
  }
 }
 @IsTest static void licenseThenPermission() {
  User u=actor(); Id license=licenseId(); Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticAccess' LIMIT 1].Id;
  insert new PermissionSetLicenseAssign(AssigneeId=u.Id, PermissionSetLicenseId=license);
  insert new PermissionSetAssignment(AssigneeId=u.Id, PermissionSetId=ps);
 }
 @IsTest static void permissionThenLicense() {
  User u=actor(); Id license=licenseId(); Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticAccess' LIMIT 1].Id;
  insert new PermissionSetAssignment(AssigneeId=u.Id, PermissionSetId=ps);
  insert new PermissionSetLicenseAssign(AssigneeId=u.Id, PermissionSetLicenseId=license);
 }
 @IsTest static void accountThenPermission() {
  User u=actor(); Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticAccess' LIMIT 1].Id;
  insert new Account(Name='Synthetic');
  try { insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps); System.assert(false,'expected mixed DML'); }
  catch(DmlException e) { System.assert(e.getMessage().contains('MIXED_DML_OPERATION'),e.getMessage()); System.assert(e.getMessage().contains('PermissionSetAssignment, original object: Account'),e.getMessage()); }
 }
 @IsTest static void licenseThenMismatch() {
  User u=actor(); Id license=licenseId(); Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticMismatch' LIMIT 1].Id;
  insert new PermissionSetLicenseAssign(AssigneeId=u.Id,PermissionSetLicenseId=license);
  try {insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps);System.assert(false,'expected license mismatch');}
  catch(DmlException e) {System.assert(e.getMessage().contains('FIELD_INTEGRITY_EXCEPTION'),e.getMessage());System.assert(!e.getMessage().contains('MIXED_DML_OPERATION'),e.getMessage());}
 }
 @IsTest static void mismatchThenLicense() {
  User u=actor(); Id license=licenseId(); Id ps=[SELECT Id FROM PermissionSet WHERE Name='SyntheticMismatch' LIMIT 1].Id;
  try {insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps);System.assert(false,'expected license mismatch');}
  catch(DmlException e) {System.assert(e.getMessage().contains('FIELD_INTEGRITY_EXCEPTION'),e.getMessage());System.assert(!e.getMessage().contains('MIXED_DML_OPERATION'),e.getMessage());}
  insert new PermissionSetLicenseAssign(AssigneeId=u.Id,PermissionSetLicenseId=license);
 }
}
